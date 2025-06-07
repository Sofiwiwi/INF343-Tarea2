// asignacion.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	amqp "github.com/rabbitmq/amqp091-go"

	dronepb "INF343-Tarea2/proto/drones"
	pb "INF343-Tarea2/proto/emergencia"
)

const (
	mongoDBURI            = "mongodb://10.10.28.18:27017"
	dbName                = "emergencies_db"
	dronesCollection      = "drones"
	emergenciesCollection = "emergencies"
	rabbitMQURL           = "amqp://admin123:admin123@10.10.28.18:5672/"
	registrationQueue     = "emergency_registration_queue"
	droneUpdatesQueue     = "drone_updates_queue" // Cola que el monitoreo consume
	assignmentPort        = ":50051"
	dronesServiceAddress  = "10.10.28.19:50053" // Dirección del servicio de Drones
)

// Drone representa la estructura de un dron en la base de datos
type Drone struct {
	ID        string  `bson:"id"`
	Latitude  float64 `bson:"latitude"`
	Longitude float64 `bson:"longitude"`
	Status    string  `bson:"status"` // e.g., "available", "assigned"
}

// EmergencyStatus representa el estado de una emergencia
type EmergencyStatus struct {
	EmergencyID int64   `json:"emergency_id"`
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Magnitude   int32   `json:"magnitude"`
	Status      string  `json:"status"` // e.g., "En curso", "Extinguido"
}

// AssignmentService implementa los servicios gRPC definidos en emergencias.proto
type AssignmentService struct {
	pb.UnimplementedAsignacionServiceServer
	mongoClient      *mongo.Client
	rabbitMQConn     *amqp.Connection
	emergencyQueue   chan *pb.Emergency // Cola para emergencias entrantes
	currentEmergency *pb.Emergency      // Emergencia actualmente en curso
	currentDroneID   string             // ID del dron asignado a la emergencia actual
	emergencyWg      sync.WaitGroup     // Para esperar a que la emergencia actual se extinga
	mu               sync.Mutex         // Protege el acceso a currentEmergency y currentDroneID
	nextEmergencyID  int64              // Generador de IDs para emergencias
}

// NewAssignmentService crea una nueva instancia del servicio de asignación
func NewAssignmentService(mc *mongo.Client, rmqc *amqp.Connection) *AssignmentService {
	return &AssignmentService{
		mongoClient:     mc,
		rabbitMQConn:    rmqc,
		emergencyQueue:  make(chan *pb.Emergency, 100),   // Buffer para emergencias
		nextEmergencyID: time.Now().UnixNano() / 1000000, // ID inicial basado en tiempo, para ser único
	}
}

// SendEmergencies recibe la lista de emergencias del cliente y las añade a la cola
func (s *AssignmentService) SendEmergencies(ctx context.Context, req *pb.EmergencyListRequest) (*pb.EmergencyListResponse, error) {
	log.Printf("Recibiendo %d emergencias del cliente.", len(req.GetEmergencies()))
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, empb := range req.GetEmergencies() {
		// Asignar un ID único a cada emergencia antes de encolarla
		empb.EmergencyId = s.nextEmergencyID
		s.nextEmergencyID++ // Incrementar para el siguiente ID

		s.emergencyQueue <- empb
		log.Printf("Emergencia %s (ID: %d) añadida a la cola.", empb.GetName(), empb.GetEmergencyId())
	}

	return &pb.EmergencyListResponse{
		Success: true,
		Message: fmt.Sprintf("Emergencias recibidas y añadidas a la cola. Total en cola: %d", len(s.emergencyQueue)),
	}, nil
}

// NotifyEmergencyExtinguished recibe una notificación del servicio de Drones
// de que una emergencia ha sido extinguida.
func (s *AssignmentService) NotifyEmergencyExtinguished(ctx context.Context, req *pb.EmergencyExtinguishedNotification) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentEmergency != nil && s.currentEmergency.GetEmergencyId() == req.GetEmergencyId() {
		log.Printf("Notificación: Emergencia %d ('%s') extinguida por dron %s con estado '%s'.",
			req.GetEmergencyId(), s.currentEmergency.GetName(), req.GetDronId(), req.GetFinalStatus())

		// La emergencia actual ha terminado.
		s.emergencyWg.Done() // Señaliza que la goroutine de procesamiento puede continuar
		s.currentEmergency = nil
		s.currentDroneID = ""
	} else {
		log.Printf("Notificación de emergencia extinguida para ID %d no coincide con la emergencia actual en proceso.", req.GetEmergencyId())
	}
	return &emptypb.Empty{}, nil
}

// processEmergencies es una goroutine que se encarga de procesar las emergencias de la cola
func (s *AssignmentService) processEmergencies() {
	log.Println("Iniciando procesamiento de emergencias...")
	for {
		select {
		case emergency := <-s.emergencyQueue:
			s.mu.Lock()
			if s.currentEmergency != nil {
				s.mu.Unlock()
				log.Printf("Ya hay una emergencia en proceso (%s). Esperando para procesar %s (ID: %d).",
					s.currentEmergency.GetName(), emergency.GetName(), emergency.GetEmergencyId())
				// Si ya hay una emergencia en curso, volvemos a poner esta en la cola y esperamos.
				// Una solución más robusta podría usar un backoff o una cola de prioridad.
				time.Sleep(1 * time.Second)   // Pequeña pausa antes de reintentar
				s.emergencyQueue <- emergency // Devolver a la cola
				continue
			}
			s.currentEmergency = emergency
			s.mu.Unlock()

			log.Printf("Procesando emergencia: %s (ID: %d) en %.2f,%.2f (Magnitud: %d)",
				emergency.GetName(), emergency.GetEmergencyId(), emergency.GetLatitude(),
				emergency.GetLongitude(), emergency.GetMagnitude())

			// 1. Obtener drones disponibles de MongoDB
			drones, err := s.getAvailableDrones()
			if err != nil {
				log.Printf("Error al obtener drones disponibles: %v. Devolviendo emergencia a la cola.", err)
				s.emergencyQueue <- emergency
				continue
			}
			if len(drones) == 0 {
				log.Println("No hay drones disponibles. Devolviendo emergencia a la cola y reintentando en 5s.")
				s.emergencyQueue <- emergency
				time.Sleep(5 * time.Second)
				continue
			}

			// 2. Encontrar el dron más cercano
			closestDrone, err := findClosestDrone(emergency.GetLatitude(), emergency.GetLongitude(), drones)
			if err != nil {
				log.Printf("Error al encontrar dron más cercano: %v. Devolviendo emergencia a la cola.", err)
				s.emergencyQueue <- emergency
				continue
			}
			s.mu.Lock()
			s.currentDroneID = closestDrone.ID
			s.mu.Unlock()
			log.Printf("Dron %s asignado a emergencia %d.", closestDrone.ID, emergency.GetEmergencyId())

			// 3. Comunicar asignación al Servicio de Drones vía gRPC
			connDrones, err := grpc.Dial(dronesServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Printf("No se pudo conectar al Servicio de Drones en %s: %v. Reintentando emergencia.", dronesServiceAddress, err)
				s.emergencyQueue <- emergency // Devolver a la cola para reintentar
				continue
			}
			defer connDrones.Close() // Asegúrate de cerrar la conexión cuando esta iteración termine

			droneClient := dronepb.NewDronesServiceClient(connDrones)
			assignReq := &dronepb.AssignEmergencyRequest{
				EmergencyId:    emergency.GetEmergencyId(),
				EmergencyName:  emergency.GetName(),
				Latitude:       emergency.GetLatitude(),
				Longitude:      emergency.GetLongitude(),
				Magnitude:      emergency.GetMagnitude(),
				DronId:         closestDrone.ID,
				AssignmentTime: timestamppb.Now(),
			}

			_, err = droneClient.AssignEmergency(context.Background(), assignReq)
			if err != nil {
				log.Printf("Error al asignar emergencia al Servicio de Drones (%s): %v. Devolviendo emergencia a la cola.", closestDrone.ID, err)
				s.emergencyQueue <- emergency
				continue
			}
			log.Printf("Emergencia %d asignada exitosamente al dron %s vía gRPC.", emergency.GetEmergencyId(), closestDrone.ID)

			// 4. Comunicar información de la emergencia al servicio de Registro vía RabbitMQ
			// Publicar mensaje de "En curso"
			emergencyStatus := EmergencyStatus{
				EmergencyID: emergency.GetEmergencyId(),
				Name:        emergency.GetName(),
				Latitude:    emergency.GetLatitude(),
				Longitude:   emergency.GetLongitude(),
				Magnitude:   emergency.GetMagnitude(),
				Status:      "En curso",
			}
			err = s.publishEmergencyToRegistration(emergencyStatus)
			if err != nil {
				log.Printf("Error al publicar emergencia a Registro vía RabbitMQ: %v", err)
				// Considera qué hacer aquí: ¿reintentar o dejar que el registro la reciba después?
				// Por ahora, simplemente lo loggeamos.
			}
			log.Printf("Emergencia %d enviada al servicio de registro (estado: 'En curso').", emergency.GetEmergencyId())

			// 5. Esperar a que la emergencia sea apagada
			s.emergencyWg.Add(1) // Esperamos una señal de que la emergencia actual ha terminado
			s.emergencyWg.Wait() // Bloquea hasta que NotifyEmergencyExtinguished se llama
			log.Printf("Emergencia %d extinguida. Buscando siguiente emergencia...", emergency.GetEmergencyId())

		case <-time.After(1 * time.Second): // Pequeño delay para no busy-wait si no hay emergencias
			s.mu.Lock()
			if s.currentEmergency == nil && len(s.emergencyQueue) == 0 {
				log.Println("No hay emergencias en la cola. Esperando nuevas emergencias...")
			}
			s.mu.Unlock()
		}
	}
}

// getAvailableDrones obtiene la lista de drones con estado "available" de MongoDB
func (s *AssignmentService) getAvailableDrones() ([]Drone, error) {
	collection := s.mongoClient.Database(dbName).Collection(dronesCollection)
	filter := bson.M{"status": "available"}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("error al buscar drones: %w", err)
	}
	defer cursor.Close(context.Background())

	var drones []Drone
	if err = cursor.All(context.Background(), &drones); err != nil {
		return nil, fmt.Errorf("error al decodificar drones: %w", err)
	}

	if len(drones) == 0 {
		// Opcional: si no hay drones disponibles, intentar buscar cualquier dron para propósitos de prueba
		// collection := s.mongoClient.Database(dbName).Collection(dronesCollection)
		// cursor, err := collection.Find(context.Background(), bson.M{})
		// if err == nil {
		// 	cursor.All(context.Background(), &drones)
		// 	log.Println("No hay drones 'available', encontrando cualquier dron para simulación.")
		// }
	}

	return drones, nil
}

// findClosestDrone calcula el dron más cercano a la emergencia
func findClosestDrone(emergencyLat, emergencyLon float64, drones []Drone) (*Drone, error) {
	if len(drones) == 0 {
		return nil, fmt.Errorf("no hay drones para encontrar el más cercano")
	}

	var closestDrone *Drone
	minDistance := math.MaxFloat64

	for i := range drones {
		drone := &drones[i] // Usar puntero para modificar directamente si es necesario o para evitar copias grandes
		distance := calculateDistance(emergencyLat, emergencyLon, drone.Latitude, drone.Longitude)
		if distance < minDistance {
			minDistance = distance
			closestDrone = drone
		}
	}

	return closestDrone, nil
}

// calculateDistance calcula la distancia euclidiana entre dos puntos (simplificado para la grilla 100x100)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	return math.Sqrt(math.Pow(lat1-lat2, 2) + math.Pow(lon1-lon2, 2))
}

// publishEmergencyToRegistration publica un mensaje en la cola de RabbitMQ para el servicio de registro
func (s *AssignmentService) publishEmergencyToRegistration(status EmergencyStatus) error {
	ch, err := s.rabbitMQConn.Channel()
	if err != nil {
		return fmt.Errorf("fallo al abrir un canal de RabbitMQ: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		registrationQueue, // name
		false,             // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("fallo al declarar la cola '%s': %w", registrationQueue, err)
	}

	body, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("fallo al serializar estado de emergencia a JSON: %w", err)
	}

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return fmt.Errorf("fallo al publicar mensaje: %w", err)
	}
	return nil
}

// connectToMongoDB establece una conexión con MongoDB
func connectToMongoDB(uri string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("fallo al conectar a MongoDB: %w", err)
	}

	// Hacer ping para verificar la conexión
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("fallo al hacer ping a MongoDB: %w", err)
	}

	log.Println("Conexión a MongoDB establecida exitosamente.")
	return client, nil
}

// connectToRabbitMQ establece una conexión con RabbitMQ
func connectToRabbitMQ(url string) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	// Bucle de reintento de conexión
	for i := 0; i < 5; i++ { // Intentar 5 veces
		conn, err = amqp.Dial(url)
		if err == nil {
			log.Println("Conexión a RabbitMQ establecida exitosamente.")
			return conn, nil
		}
		log.Printf("Fallo al conectar a RabbitMQ: %v. Reintentando en %d segundos...", err, (i+1)*2)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	return nil, fmt.Errorf("fallo definitivo al conectar a RabbitMQ después de varios reintentos: %w", err)
}

func main() {
	// 1. Conectar a MongoDB
	mongoClient, err := connectToMongoDB(mongoDBURI)
	if err != nil {
		log.Fatalf("No se pudo iniciar el servicio de asignación: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Fatalf("Error al desconectar de MongoDB: %v", err)
		}
	}()

	// 2. Conectar a RabbitMQ
	rabbitMQConn, err := connectToRabbitMQ(rabbitMQURL)
	if err != nil {
		log.Fatalf("No se pudo iniciar el servicio de asignación: %v", err)
	}
	defer rabbitMQConn.Close()

	// 3. Inicializar el servicio de asignación
	assignmentService := NewAssignmentService(mongoClient, rabbitMQConn)

	// 4. Iniciar el servidor gRPC
	lis, err := net.Listen("tcp", assignmentPort)
	if err != nil {
		log.Fatalf("Fallo al escuchar en el puerto %s: %v", assignmentPort, err)
	}

	s := grpc.NewServer()
	pb.RegisterAsignacionServiceServer(s, assignmentService) // Registrar el servicio de asignación

	log.Printf("Servicio de Asignación gRPC escuchando en %s", assignmentPort)

	// 5. Iniciar la goroutine que procesa las emergencias
	go assignmentService.processEmergencies()

	// 6. Iniciar el servidor gRPC (bloqueante)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Fallo al servir el servidor gRPC: %v", err)
	}
}

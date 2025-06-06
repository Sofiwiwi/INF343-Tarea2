// drones.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	amqp "github.com/rabbitmq/amqp091-go"

	dronepb "github.com/INF343-Tarea2/proto/drones"  // Para recibir asignaciones (AssignEmergency)
	pb "github.com/INF343-Tarea2/proto/emergencia"   // Para notificar a Asignación (NotifyEmergencyExtinguished)
	monpb "github.com/INF343-Tarea2/proto/monitoreo" // Para enviar actualizaciones a Monitoreo
)

const (
	mongoDBURI               = "mongodb://localhost:27017"
	dbName                   = "emergencies_db"
	dronesCollection         = "drones"
	assignmentServiceAddress = "localhost:50051" // Dirección del servicio de Asignación
	dronesPort               = ":50053"
	rabbitMQURL              = "amqp://guest:guest@localhost:5672/"
	monitoringQueue          = "drone_updates_queue"          // Cola para el servicio de Monitoreo
	registrationQueue        = "emergency_registration_queue" // Cola para el servicio de Registro
)

// Drone representa la estructura de un dron en la base de datos
type Drone struct {
	ID        string  `bson:"id"`
	Latitude  float64 `bson:"latitude"`
	Longitude float64 `bson:"longitude"`
	Status    string  `bson:"status"` // e.g., "available", "assigned", "returning"
}

// DroneService implements the gRPC DronesServiceServer interface.
type DroneService struct {
	dronepb.UnimplementedDronesServiceServer
	mongoClient  *mongo.Client
	rabbitMQConn *amqp.Connection
	// Podrías tener un mapa de drones si cada dron corriera su propia simulación
	// pero para este caso, cada llamada a AssignEmergency simulará un dron.
}

// NewDroneService creates a new instance of the drone service.
func NewDroneService(mc *mongo.Client, rmqc *amqp.Connection) *DroneService {
	return &DroneService{
		mongoClient:  mc,
		rabbitMQConn: rmqc,
	}
}

// AssignEmergency handles the gRPC request from the Assignment service.
// This function simulates the drone's operations for a given emergency.
func (s *DroneService) AssignEmergency(ctx context.Context, req *dronepb.AssignEmergencyRequest) (*dronepb.AssignEmergencyResponse, error) {
	log.Printf("Dron %s asignado a emergencia %d (%s) en %.2f,%.2f (Magnitud: %d).",
		req.GetDronId(), req.GetEmergencyId(), req.GetEmergencyName(),
		req.GetLatitude(), req.GetLongitude(), req.GetMagnitude())

	// 1. Marcar dron como "assigned" en la base de datos
	err := s.updateDroneStatusInDB(req.GetDronId(), "assigned")
	if err != nil {
		log.Printf("Error al actualizar estado del dron %s a 'assigned': %v", req.GetDronId(), err)
		return &dronepb.AssignEmergencyResponse{Success: false, Message: "Error al actualizar estado del dron."}, err
	}

	// 2. Simular desplazamiento y apagado
	currentLat, currentLon, err := s.getDronePosition(req.GetDronId())
	if err != nil {
		log.Printf("Error al obtener posición inicial del dron %s: %v", req.GetDronId(), err)
		// Intentar usar una posición por defecto si no se encuentra en DB
		currentLat, currentLon = req.GetLatitude(), req.GetLongitude() // Fallback: empezar en la emergencia para simulación
	}

	// Goroutine para simular y enviar actualizaciones en paralelo
	go func() {
		// Simular movimiento (0.5 s por unidad de distancia)
		log.Printf("Dron %s en camino a emergencia %d...", req.GetDronId(), req.GetEmergencyId())
		s.simulateMovement(req.GetEmergencyId(), req.GetEmergencyName(), req.GetDronId(), currentLat, currentLon, req.GetLatitude(), req.GetLongitude())

		// Simular apagado (2 s por unidad de magnitud)
		log.Printf("Dron %s apagando emergencia %d...", req.GetDronId(), req.GetEmergencyId())
		s.simulateExtinguishing(req.GetEmergencyId(), req.GetEmergencyName(), req.GetDronId(), req.GetMagnitude(), req.GetLatitude(), req.GetLongitude())

		// 3. Cuando la emergencia sea extinguida:

		// a. Actualizar la posición final del dron en la base de datos
		err := s.updateDronePositionInDB(req.GetDronId(), req.GetLatitude(), req.GetLongitude())
		if err != nil {
			log.Printf("Error al actualizar posición final del dron %s: %v", req.GetDronId(), err)
		} else {
			log.Printf("Dron %s actualizado a posición final (%.2f, %.2f).", req.GetDronId(), req.GetLatitude(), req.GetLongitude())
		}
		// Y marcar como disponible de nuevo (o en "returning" si hubiera otra fase)
		err = s.updateDroneStatusInDB(req.GetDronId(), "available")
		if err != nil {
			log.Printf("Error al actualizar estado del dron %s a 'available': %v", req.GetDronId(), err)
		}

		// b. Informar al servicio de Registro de Emergencias via RabbitMQ
		registrationUpdate := &monpb.EmergencyStatusUpdate{ // Reutilizamos el tipo del monitoreo, Registro solo necesita ID y Status
			EmergencyId:   req.GetEmergencyId(),
			EmergencyName: req.GetEmergencyName(),
			Status:        "Extinguido",
			DronId:        req.GetDronId(),
			Latitude:      req.GetLatitude(),
			Longitude:     req.GetLongitude(),
			Magnitude:     req.GetMagnitude(),
			Timestamp:     timestamppb.Now(),
		}
		err = s.publishStatusToRabbitMQ(registrationUpdate, registrationQueue)
		if err != nil {
			log.Printf("Error al enviar 'Extinguido' al servicio de Registro para ID %d: %v", req.GetEmergencyId(), err)
		} else {
			log.Printf("Notificación 'Extinguido' para emergencia %d enviada a Registro.", req.GetEmergencyId())
		}

		// c. Informar al servicio de Asignación que emergencia ha sido apagada via gRPC
		assignConn, err := grpc.Dial(assignmentServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Error al conectar con Servicio de Asignación para notificar extinción: %v", err)
			return // No se pudo notificar, pero el dron terminó su tarea
		}
		defer assignConn.Close()
		assignClient := pb.NewAsignacionServiceClient(assignConn)

		notificationReq := &pb.EmergencyExtinguishedNotification{
			EmergencyId: req.GetEmergencyId(),
			DronId:      req.GetDronId(),
			FinalStatus: "Extinguido",
		}

		_, err = assignClient.NotifyEmergencyExtinguished(context.Background(), notificationReq)
		if err != nil {
			log.Printf("Error al notificar al Servicio de Asignación sobre extinción de emergencia %d: %v", req.GetEmergencyId(), err)
		} else {
			log.Printf("Servicio de Asignación notificado de extinción de emergencia %d.", req.GetEmergencyId())
		}

		log.Printf("Incendio %s (ID: %d) ha sido extinguido por dron %s.", req.GetEmergencyName(), req.GetEmergencyId(), req.GetDronId())
	}()

	return &dronepb.AssignEmergencyResponse{Success: true, Message: "Asignación de emergencia recibida y procesando."}, nil
}

// simulateMovement simulates the drone's travel to the emergency location.
func (s *DroneService) simulateMovement(emergencyID int64, emergencyName, dronID string, startLat, startLon, targetLat, targetLon float64) {
	distance := calculateDistance(startLat, startLon, targetLat, targetLon)
	travelTime := time.Duration(math.Ceil(distance/2)) * time.Second // 0.5s por unidad de distancia, así que distancia/2 * 1s
	log.Printf("Dron %s (ID: %d) viajará %.2f unidades de distancia, tiempo estimado: %s.", dronID, emergencyID, distance, travelTime)

	steps := 10 // Número de pasos para la simulación
	for i := 0; i < steps; i++ {
		// Calcular posición intermedia (simulación simple de movimiento lineal)
		fraction := float64(i) / float64(steps)
		currentLat := startLat + (targetLat-startLat)*fraction
		currentLon := startLon + (targetLon-startLon)*fraction

		// Informar del status cada 5 segundos al servicio de monitoreo
		if i%(steps/5) == 0 || i == steps-1 { // Solo para algunos pasos o el final
			update := &monpb.EmergencyStatusUpdate{
				EmergencyId:   emergencyID,
				EmergencyName: emergencyName,
				Status:        "En camino",
				DronId:        dronID,
				Latitude:      currentLat,
				Longitude:     currentLon,
				Magnitude:     0, // Magnitud no relevante para estado 'En camino'
				Timestamp:     timestamppb.Now(),
			}
			err := s.publishStatusToRabbitMQ(update, monitoringQueue)
			if err != nil {
				log.Printf("Error al publicar estado de movimiento de dron %s a Monitoreo: %v", dronID, err)
			}
			log.Printf("Dron %s en camino... (%.2f, %.2f)", dronID, currentLat, currentLon)
		}
		time.Sleep(travelTime / time.Duration(steps)) // Pausa por cada paso
	}
	log.Printf("Dron %s ha llegado a la emergencia %d.", dronID, emergencyID)

	// Envía una última actualización al llegar, si no se hizo en el último paso del bucle
	update := &monpb.EmergencyStatusUpdate{
		EmergencyId:   emergencyID,
		EmergencyName: emergencyName,
		Status:        "Llegado",
		DronId:        dronID,
		Latitude:      targetLat,
		Longitude:     targetLon,
		Magnitude:     0,
		Timestamp:     timestamppb.Now(),
	}
	err := s.publishStatusToRabbitMQ(update, monitoringQueue)
	if err != nil {
		log.Printf("Error al publicar estado 'Llegado' de dron %s a Monitoreo: %v", dronID, err)
	}
}

// simulateExtinguishing simulates the fire extinguishing process.
func (s *DroneService) simulateExtinguishing(emergencyID int64, emergencyName, dronID string, magnitude int32, emergencyLat, emergencyLon float64) {
	extinguishTime := time.Duration(magnitude*2) * time.Second // 2s por unidad de magnitud
	log.Printf("Dron %s apagando incendio en emergencia %d (magnitud %d), tiempo estimado: %s.", dronID, emergencyID, magnitude, extinguishTime)

	steps := 5 // Número de pasos para la simulación
	for i := 0; i < steps; i++ {
		// Informar del status cada 5 segundos al servicio de monitoreo
		if i%(steps/2) == 0 || i == steps-1 { // Solo para algunos pasos o el final
			update := &monpb.EmergencyStatusUpdate{
				EmergencyId:   emergencyID,
				EmergencyName: emergencyName,
				Status:        "Apagando",
				DronId:        dronID,
				Latitude:      emergencyLat,
				Longitude:     emergencyLon,
				Magnitude:     magnitude,
				Timestamp:     timestamppb.Now(),
			}
			err := s.publishStatusToRabbitMQ(update, monitoringQueue)
			if err != nil {
				log.Printf("Error al publicar estado 'Apagando' de dron %s a Monitoreo: %v", dronID, err)
			}
			log.Printf("Dron %s apagando emergencia...", dronID)
		}
		time.Sleep(extinguishTime / time.Duration(steps)) // Pausa por cada paso
	}
	log.Printf("Dron %s ha terminado de apagar la emergencia %d.", dronID, emergencyID)
}

// calculateDistance calculates the Euclidean distance between two points (simplified for 100x100 grid)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	return math.Sqrt(math.Pow(lat1-lat2, 2) + math.Pow(lon1-lon2, 2))
}

// publishStatusToRabbitMQ publishes a status update message to the specified RabbitMQ queue.
func (s *DroneService) publishStatusToRabbitMQ(update *monpb.EmergencyStatusUpdate, queueName string) error {
	ch, err := s.rabbitMQConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a RabbitMQ channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue '%s': %w", queueName, err)
	}

	body, err := json.Marshal(update) // Marshal the protobuf message to JSON
	if err != nil {
		return fmt.Errorf("failed to marshal status update to JSON: %w", err)
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
		return fmt.Errorf("failed to publish message to queue '%s': %w", queueName, err)
	}
	return nil
}

// updateDronePositionInDB updates the drone's position in MongoDB.
func (s *DroneService) updateDronePositionInDB(dronID string, newLat, newLon float64) error {
	collection := s.mongoClient.Database(dbName).Collection(dronesCollection)
	filter := bson.M{"id": dronID}
	update := bson.M{"$set": bson.M{"latitude": newLat, "longitude": newLon}}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update drone %s position in DB: %w", dronID, err)
	}
	return nil
}

// updateDroneStatusInDB updates the drone's status in MongoDB.
func (s *DroneService) updateDroneStatusInDB(dronID, status string) error {
	collection := s.mongoClient.Database(dbName).Collection(dronesCollection)
	filter := bson.M{"id": dronID}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update drone %s status in DB: %w", dronID, err)
	}
	return nil
}

// getDronePosition fetches the current position of a drone from MongoDB.
func (s *DroneService) getDronePosition(dronID string) (float64, float64, error) {
	collection := s.mongoClient.Database(dbName).Collection(dronesCollection)
	var drone Drone
	filter := bson.M{"id": dronID}
	err := collection.FindOne(context.Background(), filter).Decode(&drone)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find drone %s in DB: %w", dronID, err)
	}
	return drone.Latitude, drone.Longitude, nil
}

// connectToMongoDB establishes a connection with MongoDB
func connectToMongoDB(uri string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	log.Println("MongoDB connection established successfully.")
	return client, nil
}

// connectToRabbitMQ establishes a connection with RabbitMQ
func connectToRabbitMQ(url string) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	// Retry loop for connection
	for i := 0; i < 5; i++ { // Try 5 times
		conn, err = amqp.Dial(url)
		if err == nil {
			log.Println("RabbitMQ connection established successfully.")
			return conn, nil
		}
		log.Printf("Failed to connect to RabbitMQ: %v. Retrying in %d seconds...", err, (i+1)*2)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	return nil, fmt.Errorf("final failure to connect to RabbitMQ after multiple retries: %w", err)
}

func main() {
	// 1. Connect to MongoDB
	mongoClient, err := connectToMongoDB(mongoDBURI)
	if err != nil {
		log.Fatalf("Failed to start drone service: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Fatalf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// 2. Connect to RabbitMQ
	rabbitMQConn, err := connectToRabbitMQ(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to start drone service: %v", err)
	}
	defer rabbitMQConn.Close()

	// 3. Initialize the drone service
	droneService := NewDroneService(mongoClient, rabbitMQConn)

	// 4. Start the gRPC server
	lis, err := net.Listen("tcp", dronesPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", dronesPort, err)
	}

	s := grpc.NewServer()
	dronepb.RegisterDronesServiceServer(s, droneService) // Register the drone service

	log.Printf("Drone gRPC service listening on %s", dronesPort)

	// 5. Start the gRPC server (blocking)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

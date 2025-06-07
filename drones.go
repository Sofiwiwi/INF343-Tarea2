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

	amqp "github.com/rabbitmq/amqp091-go"

	dronepb "INF343-Tarea2/proto/drones"
	pb "INF343-Tarea2/proto/emergencia"
)

const (
	mongoDBURI               = "mongodb://10.10.28.18:27017"
	dbName                   = "emergencies_db"
	dronesCollection         = "drones"
	assignmentServiceAddress = "10.10.28.18:50051"
	dronesPort               = ":50053"
	rabbitMQURL              = "amqp://admin123:admin123@10.10.28.18:5672/"
	monitoringQueue          = "drone_updates_queue"
	registrationQueue        = "emergency_registration_queue"
)

type Drone struct {
	ID        string  `bson:"id"`
	Latitude  float64 `bson:"latitude"`
	Longitude float64 `bson:"longitude"`
	Status    string  `bson:"status"`
}

type DroneService struct {
	dronepb.UnimplementedDronesServiceServer
	mongoClient  *mongo.Client
	rabbitMQConn *amqp.Connection
}

func NewDroneService(mc *mongo.Client, rmqc *amqp.Connection) *DroneService {
	return &DroneService{
		mongoClient:  mc,
		rabbitMQConn: rmqc,
	}
}

func (s *DroneService) AssignEmergency(ctx context.Context, req *dronepb.AssignEmergencyRequest) (*dronepb.AssignEmergencyResponse, error) {
	log.Printf("Dron %s asignado a emergencia %d (%s)", req.GetDronId(), req.GetEmergencyId(), req.GetEmergencyName())

	if err := s.updateDroneStatusInDB(req.GetDronId(), "assigned"); err != nil {
		return &dronepb.AssignEmergencyResponse{Success: false, Message: "Error actualizando estado"}, err
	}

	lat, lon, err := s.getDronePosition(req.GetDronId())
	if err != nil {
		log.Printf("Usando posición por defecto para dron %s", req.GetDronId())
		lat, lon = req.GetLatitude(), req.GetLongitude()
	}

	go func() {
		s.simulateMovement(req.GetEmergencyId(), req.GetEmergencyName(), req.GetDronId(), lat, lon, req.GetLatitude(), req.GetLongitude())
		s.simulateExtinguishing(req.GetEmergencyId(), req.GetEmergencyName(), req.GetDronId(), req.GetMagnitude(), req.GetLatitude(), req.GetLongitude())

		_ = s.updateDronePositionInDB(req.GetDronId(), req.GetLatitude(), req.GetLongitude())
		_ = s.updateDroneStatusInDB(req.GetDronId(), "available")

		mensaje := formatUpdateMessage("Extinguido", req.GetEmergencyName(), req.GetDronId(), req.GetLatitude(), req.GetLongitude(), req.GetMagnitude())
		_ = s.publishStyledMessageToRabbitMQ(mensaje, registrationQueue)

		conn, err := grpc.Dial(assignmentServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Error al conectar con asignación: %v", err)
			return
		}
		defer conn.Close()

		client := pb.NewAsignacionServiceClient(conn)
		_, _ = client.NotifyEmergencyExtinguished(context.Background(), &pb.EmergencyExtinguishedNotification{
			EmergencyId: req.GetEmergencyId(),
			DronId:      req.GetDronId(),
			FinalStatus: "Extinguido",
		})
	}()

	return &dronepb.AssignEmergencyResponse{Success: true, Message: "Asignación en proceso"}, nil
}

func (s *DroneService) simulateMovement(id int64, name, dronID string, lat1, lon1, lat2, lon2 float64) {
	steps := 10
	distance := calculateDistance(lat1, lon1, lat2, lon2)
	delay := time.Duration(math.Ceil(distance/2)) * time.Second

	for i := 0; i < steps; i++ {
		frac := float64(i) / float64(steps)
		curLat := lat1 + (lat2-lat1)*frac
		curLon := lon1 + (lon2-lon1)*frac

		if i%(steps/5) == 0 || i == steps-1 {
			mensaje := formatUpdateMessage("En camino", name, dronID, curLat, curLon, 0)
			_ = s.publishStyledMessageToRabbitMQ(mensaje, monitoringQueue)
		}
		time.Sleep(delay / time.Duration(steps))
	}

	mensaje := formatUpdateMessage("Llegado", name, dronID, lat2, lon2, 0)
	_ = s.publishStyledMessageToRabbitMQ(mensaje, monitoringQueue)
}

func (s *DroneService) simulateExtinguishing(id int64, name, dronID string, mag int32, lat, lon float64) {
	steps := 5
	delay := time.Duration(mag*2) * time.Second

	for i := 0; i < steps; i++ {
		if i%(steps/2) == 0 || i == steps-1 {
			mensaje := formatUpdateMessage("Apagando", name, dronID, lat, lon, mag)
			_ = s.publishStyledMessageToRabbitMQ(mensaje, monitoringQueue)
		}
		time.Sleep(delay / time.Duration(steps))
	}
}

func calculateDistance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x1-x2, y1-y2)
}

func formatUpdateMessage(etapa, emergencia, dron string, lat, lon float64, mag int32) string {
	switch etapa {
	case "En camino":
		return fmt.Sprintf("Dron %s en camino a emergencia %s...", dron, emergencia)
	case "Llegado":
		return fmt.Sprintf("Dron %s llegó al lugar de la emergencia %s", dron, emergencia)
	case "Apagando":
		return fmt.Sprintf("Dron %s apagando emergencia %s...", dron, emergencia)
	case "Extinguido":
		return fmt.Sprintf("Incendio %s ha sido extinguido por %s", emergencia, dron)
	default:
		return fmt.Sprintf("Estado desconocido de %s", emergencia)
	}
}

func (s *DroneService) publishStyledMessageToRabbitMQ(message, queue string) error {
	ch, err := s.rabbitMQConn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(queue, false, false, false, false, nil)
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]string{"mensaje": message})
	if err != nil {
		return err
	}

	return ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (s *DroneService) updateDronePositionInDB(dronID string, lat, lon float64) error {
	return s.updateDroneField(dronID, bson.M{"latitude": lat, "longitude": lon})
}

func (s *DroneService) updateDroneStatusInDB(dronID, status string) error {
	return s.updateDroneField(dronID, bson.M{"status": status})
}

func (s *DroneService) updateDroneField(dronID string, update bson.M) error {
	_, err := s.mongoClient.Database(dbName).Collection(dronesCollection).UpdateOne(
		context.Background(), bson.M{"id": dronID}, bson.M{"$set": update},
	)
	return err
}

func (s *DroneService) getDronePosition(dronID string) (float64, float64, error) {
	var drone Drone
	err := s.mongoClient.Database(dbName).Collection(dronesCollection).FindOne(
		context.Background(), bson.M{"id": dronID},
	).Decode(&drone)
	return drone.Latitude, drone.Longitude, err
}

func connectToMongoDB(uri string) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, err
	}
	log.Println("MongoDB conectado")
	return client, nil
}

func connectToRabbitMQ(url string) (*amqp.Connection, error) {
	for i := 0; i < 5; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			log.Println("RabbitMQ conectado")
			return conn, nil
		}
		log.Printf("Fallo RabbitMQ: %v. Reintentando...", err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	return nil, fmt.Errorf("no se pudo conectar a RabbitMQ")
}

func main() {
	mongoClient, err := connectToMongoDB(mongoDBURI)
	if err != nil {
		log.Fatalf("MongoDB error: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	rabbitMQConn, err := connectToRabbitMQ(rabbitMQURL)
	if err != nil {
		log.Fatalf("RabbitMQ error: %v", err)
	}
	defer rabbitMQConn.Close()

	service := NewDroneService(mongoClient, rabbitMQConn)

	lis, err := net.Listen("tcp", dronesPort)
	if err != nil {
		log.Fatalf("No se puede escuchar en %s: %v", dronesPort, err)
	}

	server := grpc.NewServer()
	dronepb.RegisterDronesServiceServer(server, service)
	log.Printf("Servicio Drone gRPC en %s", dronesPort)

	if err := server.Serve(lis); err != nil {
		log.Fatalf("Error al servir gRPC: %v", err)
	}
}

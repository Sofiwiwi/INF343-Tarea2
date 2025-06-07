package main

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	amqp "github.com/rabbitmq/amqp091-go"

	monpb "INF343-Tarea2/proto/monitoreo"
)

type MonitoringService struct {
	monpb.UnimplementedMonitoreoServiceServer
	mu                  sync.Mutex
	subscribers         map[chan *monpb.EmergencyStatusUpdate]bool
	updatesFromRabbitMQ chan *monpb.EmergencyStatusUpdate
}

func NewMonitoringService() *MonitoringService {
	return &MonitoringService{
		subscribers:         make(map[chan *monpb.EmergencyStatusUpdate]bool),
		updatesFromRabbitMQ: make(chan *monpb.EmergencyStatusUpdate, 100),
	}
}

func (s *MonitoringService) SubscribeToUpdates(req *monpb.SubscriptionRequest, stream monpb.MonitoreoService_SubscribeToUpdatesServer) error {
	clientUpdates := make(chan *monpb.EmergencyStatusUpdate)

	s.mu.Lock()
	s.subscribers[clientUpdates] = true
	s.mu.Unlock()

	log.Println("Cliente suscrito al servicio de monitoreo.")

	for {
		select {
		case update := <-clientUpdates:
			if err := stream.Send(update); err != nil {
				log.Printf("Error al enviar actualización: %v", err)
				s.removeSubscriber(clientUpdates)
				return err
			}
		case <-stream.Context().Done():
			log.Println("Cliente canceló la suscripción.")
			s.removeSubscriber(clientUpdates)
			return stream.Context().Err()
		}
	}
}

func (s *MonitoringService) removeSubscriber(clientUpdates chan *monpb.EmergencyStatusUpdate) {
	s.mu.Lock()
	delete(s.subscribers, clientUpdates)
	close(clientUpdates)
	s.mu.Unlock()
	log.Printf("Suscriptor eliminado. Quedan %d activos.", len(s.subscribers))
}

func (s *MonitoringService) consumeRabbitMQMessages(rabbitMQURL, queueName string) {
	for {
		conn, err := amqp.Dial(rabbitMQURL)
		if err != nil {
			log.Printf("❌ Error conectando a RabbitMQ: %v. Reintentando...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("❌ Error abriendo canal RabbitMQ: %v", err)
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		q, err := ch.QueueDeclare(queueName, false, false, false, false, nil)
		if err != nil {
			log.Printf("❌ Error declarando cola '%s': %v", queueName, err)
			ch.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
		if err != nil {
			log.Printf("❌ Error al registrar consumidor: %v", err)
			ch.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("✅ Esperando mensajes en RabbitMQ '%s'...", q.Name)

		for d := range msgs {
			var update monpb.EmergencyStatusUpdate
			if err := json.Unmarshal(d.Body, &update); err != nil {
				log.Printf("❌ Error deserializando mensaje: %v", err)
				continue
			}
			if update.Timestamp == nil {
				update.Timestamp = timestamppb.Now()
			}
			s.updatesFromRabbitMQ <- &update
		}

		log.Println("⚠️ Canal RabbitMQ cerrado. Reconectando...")
		conn.Close()
		ch.Close()
	}
}

func (s *MonitoringService) broadcastUpdates() {
	for update := range s.updatesFromRabbitMQ {
		s.mu.Lock()
		for client := range s.subscribers {
			select {
			case client <- update:
			default:
				log.Println("⚠️ Canal de cliente bloqueado. Posible cliente desconectado.")
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	const (
		grpcPort    = ":50052"
		queueName   = "drone_updates_queue"
		rabbitMQURL = "amqp://guest:guest@localhost:5672/" // usar 'localhost' si se ejecuta en la misma VM
	)

	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("❌ No se pudo escuchar en %s: %v", grpcPort, err)
	}

	server := grpc.NewServer()
	monitoringService := NewMonitoringService()
	monpb.RegisterMonitoreoServiceServer(server, monitoringService)

	log.Printf("✅ Servicio de Monitoreo gRPC escuchando en %s", grpcPort)

	go monitoringService.consumeRabbitMQMessages(rabbitMQURL, queueName)
	go monitoringService.broadcastUpdates()

	if err := server.Serve(lis); err != nil {
		log.Fatalf("❌ Error al iniciar el servidor gRPC: %v", err)
	}
}

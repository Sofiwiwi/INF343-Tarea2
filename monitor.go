// monitoreo.go
package main

import (
	"encoding/json" // Para (des)serializar mensajes de RabbitMQ si vienen en JSON
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	amqp "github.com/rabbitmq/amqp091-go" // Cliente RabbitMQ

	// Importa los paquetes gRPC generados a partir de los .proto
	// Reemplaza 'your_module_name' con el nombre real de tu módulo Go (e.g., github.com/Sofiwiwi/INF343-Tarea2)
	monpb "INF343-Tarea2/proto/monitoreo"
)

// MonitoringService implements the gRPC MonitoreoService.
type MonitoringService struct {
	monpb.UnimplementedMonitoreoServiceServer                                            // Required for future gRPC compatibility
	mu                                        sync.Mutex                                 // Protects concurrency on the stream list
	subscribers                               map[chan *monpb.EmergencyStatusUpdate]bool // Map of channels for subscribers
	// Channel to receive processed updates from RabbitMQ
	updatesFromRabbitMQ chan *monpb.EmergencyStatusUpdate
}

// NewMonitoringService creates a new instance of the monitoring service.
func NewMonitoringService() *MonitoringService {
	return &MonitoringService{
		subscribers:         make(map[chan *monpb.EmergencyStatusUpdate]bool),
		updatesFromRabbitMQ: make(chan *monpb.EmergencyStatusUpdate, 100), // Buffer for updates
	}
}

// SubscribeToUpdates implements the gRPC streaming method to send updates to the client.
func (s *MonitoringService) SubscribeToUpdates(req *monpb.SubscriptionRequest, stream monpb.MonitoreoService_SubscribeToUpdatesServer) error {
	// Create a channel for this client and add it to the list of subscribers
	clientUpdates := make(chan *monpb.EmergencyStatusUpdate)

	s.mu.Lock()
	s.subscribers[clientUpdates] = true
	s.mu.Unlock()

	log.Printf("New client subscribed to the monitoring service.")

	// Send updates to this client until the connection closes or an error occurs
	for {
		select {
		case update := <-clientUpdates:
			// Send the update to the client
			if err := stream.Send(update); err != nil {
				log.Printf("Error sending update to a client: %v", err)
				// If there's an error sending, assume the client has disconnected
				s.removeSubscriber(clientUpdates)
				return err // Terminate the goroutine for this client
			}
		case <-stream.Context().Done():
			// The client has canceled the subscription (e.g., the client closed)
			log.Printf("Client has canceled the subscription.")
			s.removeSubscriber(clientUpdates)
			return stream.Context().Err() // Return the cancellation error
		}
	}
}

// removeSubscriber removes a subscriber channel from the list.
func (s *MonitoringService) removeSubscriber(clientUpdates chan *monpb.EmergencyStatusUpdate) {
	s.mu.Lock()
	delete(s.subscribers, clientUpdates)
	close(clientUpdates) // Close the channel to release resources
	s.mu.Unlock()
	log.Printf("Subscriber removed. Active subscribers: %d", len(s.subscribers))
}

// consumeRabbitMQMessages connects to RabbitMQ and consumes messages from the drone update queue.
func (s *MonitoringService) consumeRabbitMQMessages(rabbitMQURL, queueName string) {
	// Retry RabbitMQ connection in case of failure
	for {
		conn, err := amqp.Dial(rabbitMQURL)
		if err != nil {
			log.Printf("Error connecting to RabbitMQ at %s: %v. Retrying in 5 seconds...", rabbitMQURL, err)
			time.Sleep(5 * time.Second)
			continue
		}
		defer conn.Close()

		ch, err := conn.Channel()
		if err != nil {
			log.Printf("Error opening a RabbitMQ channel: %v. Retrying in 5 seconds...", err)
			conn.Close() // Close the current connection before retrying
			time.Sleep(5 * time.Second)
			continue
		}
		defer ch.Close()

		q, err := ch.QueueDeclare(
			queueName, // queue name
			false,     // durable
			false,     // delete when unused
			false,     // exclusive
			false,     // no-wait
			nil,       // arguments
		)
		if err != nil {
			log.Printf("Error declaring queue '%s': %v. Retrying in 5 seconds...", queueName, err)
			ch.Close() // Close the current channel before retrying
			time.Sleep(5 * time.Second)
			continue
		}

		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // args
		)
		if err != nil {
			log.Printf("Error registering RabbitMQ consumer: %v. Retrying in 5 seconds...", err)
			ch.Close() // Close the current channel before retrying
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("Waiting for RabbitMQ messages on queue '%s'...", q.Name)

		// Message consumption loop
		for d := range msgs {
			log.Printf("Message received from RabbitMQ: %s", d.Body)

			// Assumes the message body is JSON that can be directly
			// deserialized into monpb.EmergencyStatusUpdate.
			// If the format is different, you will need to map it.
			var update monpb.EmergencyStatusUpdate
			if err := json.Unmarshal(d.Body, &update); err != nil {
				log.Printf("Error deserializing RabbitMQ message: %v. Message: %s", err, d.Body)
				continue
			}

			// Ensure timestamp is present if needed
			if update.Timestamp == nil {
				update.Timestamp = timestamppb.Now()
			}

			// Send the update to the channel to be processed and distributed to gRPC clients
			s.updatesFromRabbitMQ <- &update
		}

		log.Printf("RabbitMQ consumer stopped. Retrying connection...")
		// If the `for d := range msgs` loop terminates, it means the message channel was closed,
		// which often indicates a RabbitMQ disconnection. Reattempt connection.
	}
}

// broadcastUpdates reads from the updatesFromRabbitMQ channel and sends updates to all subscribers.
func (s *MonitoringService) broadcastUpdates() {
	for update := range s.updatesFromRabbitMQ {
		s.mu.Lock()
		for clientUpdates := range s.subscribers {
			select {
			case clientUpdates <- update:
				// Message sent successfully
			default:
				// The client's channel is blocked (likely slow or disconnected).
				// You could handle this here, for example, by closing the channel and removing the subscriber.
				log.Printf("Warning: A subscriber's channel is full or blocked. Possible slow client.")
				// s.removeSubscriber(clientUpdates) // Be careful calling this within the loop if you don't want to stop iteration
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	// Address for the gRPC monitoring service
	grpcPort := ":50052"
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", grpcPort, err)
	}

	s := grpc.NewServer()
	monitoringService := NewMonitoringService()
	monpb.RegisterMonitoreoServiceServer(s, monitoringService)

	log.Printf("Monitoring gRPC service listening on %s", grpcPort)

	// RabbitMQ configuration
	rabbitMQURL := "amqp://admin123:admin123@10.10.28.18:5672/" // Default URL, adjust if different
	queueName := "drone_updates_queue"                          // Queue name for drone updates

	// Start the RabbitMQ consumer in a separate goroutine
	go monitoringService.consumeRabbitMQMessages(rabbitMQURL, queueName)

	// Start the gRPC updates broadcaster in a separate goroutine
	go monitoringService.broadcastUpdates()

	// Start the gRPC server (blocking)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

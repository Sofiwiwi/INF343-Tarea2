// cliente.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure" // Para desarrollo, usar credenciales inseguras
	"google.golang.org/protobuf/types/known/timestamppb"

	// Importa los paquetes gRPC generados a partir de los .proto
	// Asegúrate de que estos paths coincidan con tu estructura de directorios, el nombre de tu módulo Go y el 'go_package' en los .proto
	pb "INF343-Tarea2/proto/emergencia"
	monpb "INF343-Tarea2/proto/monitoreo"
)

// Emergency es la estructura para parsear el archivo JSON de emergencias
type Emergency struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Magnitude int32   `json:"magnitude"`
}

func main() {
	// Verificar que se haya proporcionado un archivo JSON como argumento
	if len(os.Args) < 2 {
		log.Fatalf("Uso: %s <ruta_archivo_emergencias.json>", os.Args[0])
	}
	jsonFilePath := os.Args[1]

	// --- Parte 1: Leer el archivo JSON y enviar al Servicio de Asignación ---
	emergencies, err := readEmergenciesFromFile(jsonFilePath)
	if err != nil {
		log.Fatalf("Error al leer el archivo de emergencias: %v", err)
	}

	log.Printf("Emergencias leídas del archivo %s:", jsonFilePath)
	for _, e := range emergencies {
		log.Printf("  - Nombre: %s, Lat: %.2f, Lon: %.2f, Magnitud: %d", e.Name, e.Latitude, e.Longitude, e.Magnitude)
	}

	// Dirección del Servicio de Asignación (MV2)
	// En un entorno real, esta dirección debería ser configurable o descubrirse
	assignmentServiceAddress := "10.10.28.18:50051"
	connAssignment, err := grpc.Dial(assignmentServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar al Servicio de Asignación en %s: %v", assignmentServiceAddress, err)
	}
	defer connAssignment.Close()
	assignmentClient := pb.NewAsignacionServiceClient(connAssignment)

	// Convertir las emergencias locales a mensajes gRPC
	grpcEmergencies := make([]*pb.Emergency, len(emergencies))
	for i, e := range emergencies {
		grpcEmergencies[i] = &pb.Emergency{
			Name:      e.Name,
			Latitude:  e.Latitude,
			Longitude: e.Longitude,
			Magnitude: e.Magnitude,
		}
	}

	// Crear la solicitud de lista de emergencias
	req := &pb.EmergencyListRequest{
		Emergencies: grpcEmergencies,
		Timestamp:   timestamppb.Now(), // Añadir un timestamp si es necesario
	}

	// Enviar la lista de emergencias al Servicio de Asignación
	log.Printf("Enviando %d emergencias al Servicio de Asignación...", len(emergencies))
	res, err := assignmentClient.SendEmergencies(context.Background(), req)
	if err != nil {
		log.Fatalf("Error al enviar emergencias al Servicio de Asignación: %v", err)
	}
	log.Printf("Respuesta del Servicio de Asignación: %s", res.GetMessage())
	if !res.GetSuccess() {
		log.Printf("Advertencia del Servicio de Asignación: %s", res.GetMessage())
	}

	// --- Parte 2: Recibir actualizaciones del Servicio de Monitoreo ---
	// Dirección del Servicio de Monitoreo (MV1)
	// En un entorno real, esta dirección debería ser configurable o descubrirse
	monitoringServiceAddress := "localhost:50052"
	connMonitoring, err := grpc.Dial(monitoringServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar al Servicio de Monitoreo en %s: %v", monitoringServiceAddress, err)
	}
	defer connMonitoring.Close()
	monitoringClient := monpb.NewMonitoreoServiceClient(connMonitoring)

	log.Printf("Suscribiéndose a actualizaciones del Servicio de Monitoreo en %s...", monitoringServiceAddress)
	stream, err := monitoringClient.SubscribeToUpdates(context.Background(), &monpb.SubscriptionRequest{})
	if err != nil {
		log.Fatalf("Error al suscribirse a actualizaciones del Servicio de Monitoreo: %v", err)
	}

	// Bucle para recibir y mostrar las actualizaciones
	for {
		update, err := stream.Recv()
		if err == nil {
			// Simular el ejemplo de ejecución del PDF
			log.Printf("Emergencia actual: %s magnitud %d en x=%.0f, y=%.0f",
				update.GetEmergencyName(), update.GetMagnitude(), update.GetLatitude(), update.GetLongitude())

			log.Printf("Se ha asignado %s a la emergencia", update.GetDronId())

			// Simular el estado de 'Dron en camino' o 'Dron apagando'
			// El documento sugiere esto como parte del cliente, pero probablemente
			// debería venir como un campo en el update del monitoreo.
			// Por simplicidad, aquí lo mostramos basado en el status genérico.
			switch update.GetStatus() {
			case "En curso":
				// El cliente solo sabe el estado general, el monitoreo podría enviar estados más específicos
				// El ejemplo de ejecución muestra "Dron en camino..." o "Dron apagando..."
				// Esto requeriría que el servicio de monitoreo envíe esos estados detallados.
				// Para este ejemplo, simplemente verificamos el estado general.
				log.Printf("Estado de la emergencia (%s): %s", update.GetEmergencyName(), update.GetStatus())
			case "Extinguido":
				log.Printf("Incendio %s ha sido extinguido por %s", update.GetEmergencyName(), update.GetDronId())
			default:
				log.Printf("Actualización de estado de emergencia: %s - Dron %s: %s (Lat: %.2f, Lon: %.2f, Mag: %d)",
					update.GetEmergencyName(), update.GetDronId(), update.GetStatus(),
					update.GetLatitude(), update.GetLongitude(), update.GetMagnitude())
			}

		} else {
			// Manejar el final del stream o errores
			if err == context.Canceled {
				log.Println("Suscripción de monitoreo cancelada.")
			} else if err == fmt.Errorf("rpc error: code = Unavailable desc = transport is closing") {
				log.Println("Conexión con el servicio de monitoreo perdida. Reintentando en 5 segundos...")
				time.Sleep(5 * time.Second)
				// Podrías implementar una lógica de reconexión aquí
				newStream, reconnErr := monitoringClient.SubscribeToUpdates(context.Background(), &monpb.SubscriptionRequest{})
				if reconnErr != nil {
					log.Printf("Error al reintentar la suscripción: %v", reconnErr)
					break // Salir si no se puede reconectar
				}
				stream = newStream
				continue // Continuar con el nuevo stream
			} else if err == fmt.Errorf("EOF") {
				log.Println("Servicio de Monitoreo cerró el stream. Finalizando escucha de actualizaciones.")
				break
			} else {
				log.Printf("Error al recibir actualización del Servicio de Monitoreo: %v", err)
			}
		}
	}
}

// readEmergenciesFromFile lee el archivo JSON y lo parsea en una lista de Emergency
func readEmergenciesFromFile(filePath string) ([]Emergency, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el archivo: %w", err)
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el archivo: %w", err)
	}

	var emergencies []Emergency
	if err := json.Unmarshal(bytes, &emergencies); err != nil {
		return nil, fmt.Errorf("error al parsear JSON: %w", err)
	}

	return emergencies, nil
}

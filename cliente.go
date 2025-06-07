package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "INF343-Tarea2/proto/emergencia"
	monpb "INF343-Tarea2/proto/monitoreo"
)

type Emergency struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Magnitude int32   `json:"magnitude"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Uso: %s <ruta_archivo_emergencias.json>", os.Args[0])
	}
	jsonFilePath := os.Args[1]

	emergencies, err := readEmergenciesFromFile(jsonFilePath)
	if err != nil {
		log.Fatalf("Error al leer el archivo de emergencias: %v", err)
	}

	fmt.Println("\nEmergencias recibidas\n")

	assignmentServiceAddress := "10.10.28.18:50051"
	connAssignment, err := grpc.Dial(assignmentServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar al Servicio de Asignación en %s: %v", assignmentServiceAddress, err)
	}
	defer connAssignment.Close()
	assignmentClient := pb.NewAsignacionServiceClient(connAssignment)

	grpcEmergencies := make([]*pb.Emergency, len(emergencies))
	for i, e := range emergencies {
		grpcEmergencies[i] = &pb.Emergency{
			Name:      e.Name,
			Latitude:  e.Latitude,
			Longitude: e.Longitude,
			Magnitude: e.Magnitude,
		}
	}

	req := &pb.EmergencyListRequest{
		Emergencies: grpcEmergencies,
		Timestamp:   timestamppb.Now(),
	}

	fmt.Printf("Enviando %d emergencias al Servicio de Asignación...\n", len(emergencies))
	res, err := assignmentClient.SendEmergencies(context.Background(), req)
	if err != nil {
		log.Fatalf("Error al enviar emergencias al Servicio de Asignación: %v", err)
	}
	fmt.Printf("Respuesta del Servicio de Asignación: %s\n", res.GetMessage())

	monitoringServiceAddress := "localhost:50052"
	connMonitoring, err := grpc.Dial(monitoringServiceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar al Servicio de Monitoreo en %s: %v", monitoringServiceAddress, err)
	}
	defer connMonitoring.Close()
	monitoringClient := monpb.NewMonitoreoServiceClient(connMonitoring)

	stream, err := monitoringClient.SubscribeToUpdates(context.Background(), &monpb.SubscriptionRequest{})
	if err != nil {
		log.Fatalf("Error al suscribirse a actualizaciones del Servicio de Monitoreo: %v", err)
	}

	var currentEmergency string

	for {
		update, err := stream.Recv()
		if err != nil {
			log.Printf("Error al recibir actualización: %v", err)
			break
		}

		if update.GetEmergencyName() != currentEmergency {
			currentEmergency = update.GetEmergencyName()
			fmt.Printf("\nEmergencia actual: %s magnitud %d en x = %.0f, y = %.0f\n",
				update.GetEmergencyName(), update.GetMagnitude(),
				update.GetLatitude(), update.GetLongitude())
			fmt.Printf("Se ha asignado %s a la emergencia\n", update.GetDronId())
		}

		switch update.GetStatus() {
		case "En camino":
			fmt.Println("Dron en camino a emergencia...")
		case "Apagando":
			fmt.Println("Dron apagando emergencia...")
		case "Extinguido":
			fmt.Printf("Incendio %s ha sido extinguido por %s\n",
				update.GetEmergencyName(), update.GetDronId())
		default:
			fmt.Printf("Estado desconocido: %s\n", update.GetStatus())
		}
	}
}

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

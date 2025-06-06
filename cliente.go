package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	asigpb "tarea2/proto"
	monpb "tarea2/proto"

	"google.golang.org/grpc"
)

type Emergencia struct {
	Name      string `json:"name"`
	Latitude  int    `json:"latitude"`
	Longitude int    `json:"longitude"`
	Magnitude int    `json:"magnitude"`
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Uso: ./cliente emergencia.json")
	}

	// Leer archivo JSON
	filename := os.Args[1]
	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("No se pudo leer el archivo: %v", err)
	}

	var emergencias []Emergencia
	if err := json.Unmarshal(file, &emergencias); err != nil {
		log.Fatalf("Error al parsear el JSON: %v", err)
	}

	// Conectarse a servicio de asignación
	connAsig, err := grpc.Dial("localhost:50052", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("No se pudo conectar al servicio de asignación: %v", err)
	}
	defer connAsig.Close()
	clienteAsignacion := asigpb.NewAsignacionClient(connAsig)

	// Enviar emergencias
	for _, e := range emergencias {
		fmt.Printf("Enviando emergencia: %s\n", e.Name)
		_, err := clienteAsignacion.EnviarEmergencia(context.Background(), &asigpb.Emergencia{
			Nombre:   e.Name,
			Latitud:  int32(e.Latitude),
			Longitud: int32(e.Longitude),
			Magnitud: int32(e.Magnitude),
		})
		if err != nil {
			log.Printf("Error al enviar emergencia %s: %v", e.Name, err)
		}
	}

	// Conectarse a servicio de monitoreo
	connMon, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("No se pudo conectar al servicio de monitoreo: %v", err)
	}
	defer connMon.Close()
	clienteMonitoreo := monpb.NewMonitoreoClient(connMon)

	// Simulación: recibir actualizaciones cada 10s (esto se reemplazaría por stream si usaran gRPC streaming)
	for {
		time.Sleep(10 * time.Second)
		res, err := clienteMonitoreo.RecibirActualizacion(context.Background(), &monpb.ClientRequest{})
		if err != nil {
			log.Println("Error al recibir actualización:", err)
			continue
		}
		fmt.Printf("Actualización: %s - %s\n", res.Nombre, res.Estado)
	}
}

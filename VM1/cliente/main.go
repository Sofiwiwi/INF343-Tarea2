package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    pb "proto" // un solo paquete para todo

    "google.golang.org/grpc"
)

type Emergencia struct {
    Name      string `json:"name"`
    Latitude  int32  `json:"latitude"`
    Longitude int32  `json:"longitude"`
    Magnitude int32  `json:"magnitude"`
}

func leerJSON(path string) ([]*pb.Emergencia, error) {
    file, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var emergencias []Emergencia
    if err := json.Unmarshal(file, &emergencias); err != nil {
        return nil, err
    }

    var pbEmergencias []*pb.Emergencia
    for _, e := range emergencias {
        pbEmergencias = append(pbEmergencias, &pb.Emergencia{
            Name:      e.Name,
            Latitude:  e.Latitude,
            Longitude: e.Longitude,
            Magnitude: e.Magnitude,
        })
    }
    return pbEmergencias, nil
}

func enviarEmergencias(emergencias []*pb.Emergencia) {
    conn, err := grpc.Dial("10.10.28.XX:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("No se pudo conectar al servicio de asignación: %v", err)
    }
    defer conn.Close()

    client := pb.NewServicioAsignacionClient(conn)

    resp, err := client.EnviarEmergencia(context.Background(), &pb.EmergenciaRequest{
        Emergencias: emergencias,
    })
    if err != nil {
        log.Fatalf("Error al enviar emergencias: %v", err)
    }
    fmt.Println("Respuesta del servidor:", resp.Mensaje)
}

func recibirActualizaciones() {
    conn, err := grpc.Dial("localhost:50052", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("No se pudo conectar al servicio de monitoreo: %v", err)
    }
    defer conn.Close()

    client := pb.NewServicioMonitoreoClient(conn)
    stream, err := client.StreamActualizaciones(context.Background(), &pb.ClienteInfo{ClienteId: "cliente1"})
    if err != nil {
        log.Fatalf("Error al conectarse al stream: %v", err)
    }

    for {
        msg, err := stream.Recv()
        if err != nil {
            log.Fatalf("Error en stream: %v", err)
        }
        fmt.Printf("[ACTUALIZACIÓN] %s: %s\n", msg.Nombre, msg.Status)
    }
}

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Uso: ./cliente archivo.json")
    }

    emergencias, err := leerJSON(os.Args[1])
    if err != nil {
        log.Fatalf("Error leyendo JSON: %v", err)
    }

    go recibirActualizaciones()
    time.Sleep(time.Second) // pequeña espera antes de enviar
    enviarEmergencias(emergencias)
    select {} // espera indefinida para que siga recibiendo updates
}

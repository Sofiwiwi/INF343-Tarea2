package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net"

    "google.golang.org/grpc"
    "github.com/streadway/amqp"

    pb "proto"
)

type servidor struct {
    pb.UnimplementedServicioAsignacionServer
}

func (s *servidor) EnviarEmergencia(ctx context.Context, req *pb.EmergenciaRequest) (*pb.Respuesta, error) {
    for _, e := range req.Emergencias {
        fmt.Println("📥 Emergencia recibida:", e.Name)

        // Aquí deberías seleccionar un dron según coordenadas

        go publicarEnRabbitMQ(e)
    }

    return &pb.Respuesta{Mensaje: "Emergencias asignadas correctamente"}, nil
}

func publicarEnRabbitMQ(e *pb.Emergencia) {
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        log.Println("Error al conectar a RabbitMQ:", err)
        return
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        log.Println("Error abriendo canal:", err)
        return
    }
    defer ch.Close()

    _, err = ch.QueueDeclare("emergencias", true, false, false, false, nil)
    if err != nil {
        log.Println("Error declarando cola:", err)
        return
    }

    body, _ := json.Marshal(map[string]interface{}{
        "name":      e.Name,
        "latitude":  e.Latitude,
        "longitude": e.Longitude,
        "magnitude": e.Magnitude,
        "status":    "En curso",
    })

    err = ch.Publish("", "emergencias", false, false, amqp.Publishing{
        ContentType: "application/json",
        Body:        body,
    })
    if err != nil {
        log.Println("Error publicando en RabbitMQ:", err)
    } else {
        fmt.Println("Emergencia publicada en RabbitMQ:", e.Name)
    }
}

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("No se pudo escuchar en el puerto 50051: %v", err)
    }

    s := grpc.NewServer()
    pb.RegisterServicioAsignacionServer(s, &servidor{})

    fmt.Println("Servicio de asignación activo en :50051")
    if err := s.Serve(lis); err != nil {
        log.Fatalf("Error al servir: %v", err)
    }
}

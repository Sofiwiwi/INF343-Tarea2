package main

import (
    //"context"
    "fmt"
    "log"
    "net"

    "github.com/streadway/amqp"
    "google.golang.org/grpc"

    pb "proto"
)

type server struct {
    pb.UnimplementedServicioMonitoreoServer
    updates chan *pb.EstadoEmergencia
}

func (s *server) StreamActualizaciones(_ *pb.ClienteInfo, stream pb.ServicioMonitoreo_StreamActualizacionesServer) error {
    for update := range s.updates {
        if err := stream.Send(update); err != nil {
            return err
        }
    }
    return nil
}

func recibirDeRabbitMQ(updates chan *pb.EstadoEmergencia) {
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        log.Fatalf("Error conectando a RabbitMQ: %v", err)
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Error abriendo canal: %v", err)
    }
    defer ch.Close()

    q, err := ch.QueueDeclare("estado_emergencia", true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Error declarando cola: %v", err)
    }

    msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Error consumiendo mensajes: %v", err)
    }

    for msg := range msgs {
        var estado pb.EstadoEmergencia
        estado.Nombre = "Emergencia" // simplificación
        estado.Status = string(msg.Body)
        updates <- &estado
    }
}

func main() {
    updates := make(chan *pb.EstadoEmergencia, 100)

    go recibirDeRabbitMQ(updates)

    lis, err := net.Listen("tcp", ":50052")
    if err != nil {
        log.Fatalf("No se pudo escuchar en el puerto 50052: %v", err)
    }

    grpcServer := grpc.NewServer()
    pb.RegisterServicioMonitoreoServer(grpcServer, &server{updates: updates})

    fmt.Println("Servicio de monitoreo corriendo en :50052...")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("Fallo el servidor gRPC: %v", err)
    }
}

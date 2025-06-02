package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "math"
    "net"
    "time"

    pb "proto"

    "google.golang.org/grpc"
    "github.com/streadway/amqp"
)

type servidor struct {
    pb.UnimplementedServicioDronServer
}

func notificarRegistroExtincion(e *pb.Emergencia) {
    conn, _ := amqp.Dial("amqp://guest:guest@localhost:5672/")
    defer conn.Close()
    ch, _ := conn.Channel()
    defer ch.Close()

    ch.QueueDeclare("extinguidas", true, false, false, false, nil)

    msg := map[string]interface{}{
        "name":      e.Name,
        "latitude":  e.Latitude,
        "longitude": e.Longitude,
        "magnitude": e.Magnitude,
        "status":    "Extinguido",
    }

    body, _ := json.Marshal(msg)
    ch.Publish("", "extinguidas", false, false, amqp.Publishing{
        ContentType: "application/json",
        Body:        body,
    })

    fmt.Println("Estado enviado a registro: Extinguido")
}


func notificarExtincionAAsignacion(nombre string) {
    conn, err := grpc.Dial("localhost:50052", grpc.WithInsecure()) // IP de VM2
    if err != nil {
        log.Println("Error al contactar asignación:", err)
        return
    }
    defer conn.Close()

    cliente := pb.NewServicioAsignacionClient(conn)
    _, err = cliente.ConfirmarExtincion(context.Background(), &pb.Confirmacion{
        Nombre: nombre,
    })
    if err != nil {
        log.Println("Error confirmando a asignación:", err)
    } else {
        fmt.Println("Confirmación enviada a asignación")
    }
}


func enviarEstadoEmergencia(nombre, estado string) {
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        log.Println("Error RabbitMQ:", err)
        return
    }
    defer conn.Close()

    ch, _ := conn.Channel()
    defer ch.Close()

    ch.QueueDeclare("estado", true, false, false, false, nil)

    mensaje := map[string]string{"nombre": nombre, "status": estado}
    body, _ := json.Marshal(mensaje)

    ch.Publish("", "estado", false, false, amqp.Publishing{
        ContentType: "application/json",
        Body:        body,
    })

    fmt.Printf("Estado actualizado: %s - %s\n", nombre, estado)
}


func calcularDistancia(x, y int32) float64 {
    dx := float64(x - 0)
    dy := float64(y - 0)
    return math.Sqrt(dx*dx + dy*dy)
}

func (s *servidor) AtenderEmergencia(ctx context.Context, asignacion *pb.AsignacionDron) (*pb.Respuesta, error) {
    e := asignacion.Emergencia
    dronId := asignacion.DronId
    fmt.Printf("🛰️ %s atendiendo %s\n", dronId, e.Name)

    distancia := calcularDistancia(e.Latitude, e.Longitude)
    tiempoDesplazamiento := time.Duration(float64(distancia)*0.5) * time.Second
    tiempoApagado := time.Duration(e.Magnitude*2) * time.Second

    // Simular movimiento
    time.Sleep(tiempoDesplazamiento)
    fmt.Printf("%s llegó al incendio %s\n", dronId, e.Name)

    // Simular apagado y enviar updates cada 5s
    apagadoDone := make(chan bool)
    go func() {
        tiempoTotal := int(tiempoApagado.Seconds())
        for t := 0; t < tiempoTotal; t += 5 {
            time.Sleep(5 * time.Second)
            enviarEstadoEmergencia(e.Name, "En curso")
        }
        apagadoDone <- true
    }()

    <-apagadoDone
    enviarEstadoEmergencia(e.Name, "Extinguido")
    fmt.Printf("%s apagó incendio %s\n", dronId, e.Name)

    // Notificar al servicio de asignación (VM2)
    notificarExtincionAAsignacion(e.Name)

    // Notificar al servicio de registro (RabbitMQ)
    notificarRegistroExtincion(e)

    return &pb.Respuesta{Mensaje: "Emergencia apagada"}, nil
}

func main() {
    lis, err := net.Listen("tcp", ":50053")
    if err != nil {
        log.Fatalf("No se pudo escuchar: %v", err)
    }

    s := grpc.NewServer()
    pb.RegisterServicioDronServer(s, &servidor{})

    fmt.Println("🛫 Servicio de drones corriendo en :50053")
    if err := s.Serve(lis); err != nil {
        log.Fatalf("Fallo al servir: %v", err)
    }
}

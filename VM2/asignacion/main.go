package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "math"
    "net"
    //"time"

    pb "proto"

    "github.com/streadway/amqp"
    "google.golang.org/grpc"
)

type servidor struct {
    pb.UnimplementedServicioAsignacionServer
}

type Dron struct {
    ID        string
    Latitude  int32
    Longitude int32
    Disponible bool
}

var drones = []Dron{
    {"dron01", 0, 0, true},
    {"dron02", -20, 20, true},
    {"dron03", 30, -10, true},
}

var extinguida = make(chan bool)

func (s *servidor) EnviarEmergencia(ctx context.Context, req *pb.EmergenciaRequest) (*pb.Respuesta, error) {
    for _, e := range req.Emergencias {
        fmt.Printf("\nEmergencia actual: %s magnitud %d en x = %d , y = %d\n",
            e.Name, e.Magnitude, e.Latitude, e.Longitude)

        dron := dronMasCercano(e)
        if dron == nil {
            log.Println("❌ No hay drones disponibles")
            continue
        }

        fmt.Printf("Se ha asignado %s a la emergencia\n", dron.ID)
        dron.Disponible = false

        // Publicar en RabbitMQ para registrar en MongoDB
        publicarEnRabbitMQ(e)

        // Enviar al dron vía gRPC
        err := enviarADron(dron, e)
        if err != nil {
            log.Printf("❌ Error al contactar al dron: %v\n", err)
            continue
        }

        // Esperar confirmación de que la emergencia fue extinguida
        esperarExtincion()

        fmt.Printf("%s ha sido extinguido por %s\n", e.Name, dron.ID)
        dron.Disponible = true
    }

    return &pb.Respuesta{Mensaje: "Emergencias procesadas"}, nil
}

func (s *servidor) ConfirmarExtincion(ctx context.Context, c *pb.Confirmacion) (*pb.Respuesta, error) {
    fmt.Printf("✅ Emergencia %s ha sido apagada (confirmación recibida)\n", c.Nombre)
    extinguida <- true
    return &pb.Respuesta{Mensaje: "Extinción confirmada"}, nil
}

func publicarEnRabbitMQ(e *pb.Emergencia) {
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        log.Println("Error conexión RabbitMQ:", err)
        return
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        log.Println("Error canal RabbitMQ:", err)
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
        log.Println("Error publicando emergencia:", err)
    } else {
        fmt.Println("Emergencia publicada en RabbitMQ")
    }
}

func enviarADron(d *Dron, e *pb.Emergencia) error {
    conn, err := grpc.Dial("localhost:50053", grpc.WithInsecure()) // Reemplazar IP con IP de VM3
    if err != nil {
        return err
    }
    defer conn.Close()

    cliente := pb.NewServicioDronClient(conn)
    _, err = cliente.AtenderEmergencia(context.Background(), &pb.AsignacionDron{
        DronId:     d.ID,
        Emergencia: e,
    })
    return err
}

func esperarExtincion() {
    <-extinguida
}

func dronMasCercano(e *pb.Emergencia) *Dron {
    var mejor *Dron
    menorDist := float64(1 << 30)
    for i := range drones {
        if !drones[i].Disponible {
            continue
        }
        dx := float64(drones[i].Latitude - e.Latitude)
        dy := float64(drones[i].Longitude - e.Longitude)
        dist := math.Sqrt(dx*dx + dy*dy)
        if dist < menorDist {
            menorDist = dist
            mejor = &drones[i]
        }
    }
    return mejor
}

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("No se pudo escuchar en :50051: %v", err)
    }

    s := grpc.NewServer()
    pb.RegisterServicioAsignacionServer(s, &servidor{})

    fmt.Println("Servicio de asignación activo en :50051")
    if err := s.Serve(lis); err != nil {
        log.Fatalf("Error al servir: %v", err)
    }
}

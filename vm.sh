#!/bin/bash

# Verifica que se haya pasado un número
if [ -z "$1" ]; then
  echo "Uso: $0 <número_vm>"
  echo "Ejemplo: ./vm.sh 1"
  exit 1
fi

VM="$1"

# Compilar archivos .proto
echo "🔧 Compilando archivos .proto..."
protoc --go_out=. --go-grpc_out=. proto/drones/drones.proto
protoc --go_out=. --go-grpc_out=. proto/emergencia/emergencia.proto
protoc --go_out=. --go-grpc_out=. proto/monitoreo/monitoreo.proto

# Ejecutar servicios según la VM
case "$VM" in
  1)
    echo "🖥️ VM1: Ejecutando Monitoreo y luego Cliente"
    echo "Iniciando monitoreo..."
    go build cliente.go
    go run monitor.go
    echo "Iniciando cliente..."
    ./cliente.go emergencias.json
    ;;
  2)
    echo "🖥️ VM2: Ejecutando Asignación y luego Registro de emergencias"
    echo "Iniciando asignación..."
    go run asignacion.go
    echo "Iniciando registro (Python)..."
    python3 registro.py
    ;;
  3)
    echo "🖥️ VM3: Ejecutando Drones"
    go run drones.go
    ;;
  4)

  *)
    echo "⚠️ Número de VM no reconocido: $VM"
    echo "Opciones válidas:"
    echo "  1: Cliente + Monitoreo"
    echo "  2: Asignación + Registro"
    echo "  3: Drones"
    exit 1
    ;;
esac

# Sistema Distribuido de Gestión de Emergencias mediante Drones

## Integrantes
- Giuseppe Queirolo 202273112-5
- Sofía Ramírez 202273008-0

## Descripción
Este sistema distribuido simula la gestión de emergencias como incendios usando una flota de drones. Está desarrollado con Go, Python, gRPC, RabbitMQ y MongoDB, distribuido en tres máquinas virtuales.

## Estructura de Máquinas Virtuales

### VM1 - Cliente y Servicio de Monitoreo
- **Cliente**: Envía información de emergencias y recibe actualizaciones
- **Monitoreo**: Recibe actualizaciones de los drones y las transmite al cliente

### VM2 - Asignación, Registro y Base de Datos
- **Asignación**: Gestiona la asignación de drones a emergencias
- **Registro**: Almacena información en MongoDB
- **MongoDB**: Base de datos para drones y emergencias

### VM3 - Sistema de Drones
- **Drones**: Simula el comportamiento de los drones para atender emergencias

### Requisitos Previos
- Go 1.21+
- Python 3.8+
- MongoDB 6.0+
- RabbitMQ 3.11+
- Protocol Buffers Compiler (protoc)

## Ejecución
Se deben conectar a las siguientes máquinas
VM3: ubuntu@sd20251-9
VM2: ubuntu@sd20251-8
VM1: ubuntu@sd20251-7

En VM2 ejecutar ./vm.sh 2 (para ejecutar los comandos asociados para esta maquina virtual)
En VM3 ejecutar ./vm.sh 3
y en VM1 ejecutar ./vm.sh 1 y despues en la misma maquina ./cliente emergencia.json
Respetar el orden de ejecución de VM1

## Instalar dependencias
### Go
```bash
sudo apt install -y golang
```
### Python + gRPC
```bash
sudo apt install -y python3 python3-pip
pip install grpcio grpcio-tools pymongo pika
```
### RabbitMQ
```bash
sudo apt install rabbitmq-server
sudo systemctl enable rabbitmq-server
sudo systemctl start rabbitmq-server
```
### MongoDB (en MV2)

Para Ubuntu 24.04:
```bash
sudo systemctl enable mongod
sudo systemctl start mongod
```
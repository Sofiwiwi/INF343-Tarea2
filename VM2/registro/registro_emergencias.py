import pika
import json
from pymongo import MongoClient

# Conexión MongoDB
client = MongoClient("mongodb://localhost:27017/")
db = client["emergencias"]
coleccion = db["eventos"]

# Conexión RabbitMQ
conn = pika.BlockingConnection(pika.ConnectionParameters("localhost"))
channel = conn.channel()

channel.queue_declare(queue="emergencias")

def callback(ch, method, properties, body):
    data = json.loads(body)
    print(f"📦 Emergencia recibida: {data['name']}")
    coleccion.insert_one(data)

channel.basic_consume(queue="emergencias", on_message_callback=callback, auto_ack=True)

print("🟢 Servicio de registro esperando mensajes...")
channel.start_consuming()

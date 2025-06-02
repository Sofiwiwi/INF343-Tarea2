import pika
import json
from pymongo import MongoClient

client = MongoClient("mongodb://localhost:27017/")
db = client["emergencias"]
col = db["eventos"]

def procesar_emergencia(ch, method, properties, body):
    data = json.loads(body)
    print(f"📝 Registrando emergencia: {data['name']}")
    col.insert_one(data)

def procesar_extincion(ch, method, properties, body):
    data = json.loads(body)
    print(f"Emergencia extinguida: {data['name']}")
    col.update_one({"name": data["name"]}, {"$set": {"status": "Extinguido"}})

conn = pika.BlockingConnection(pika.ConnectionParameters("localhost"))
ch = conn.channel()

ch.queue_declare(queue="emergencias")
ch.queue_declare(queue="extinguidas")

ch.basic_consume(queue="emergencias", on_message_callback=procesar_emergencia, auto_ack=True)
ch.basic_consume(queue="extinguidas", on_message_callback=procesar_extincion, auto_ack=True)

print("Registro de emergencias en espera...")
ch.start_consuming()

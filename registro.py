# registro.py
import pika
import pymongo
import json
import time
import logging

# Configuración de logging para ver los mensajes en consola
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# --- Configuración de Conexión ---
RABBITMQURL= "amqp://admin123:admin123@10.10.28.18:5672/"
MONGO_DB_URI = 'mongodb://10.10.28.18:27017/'
DB_NAME = 'emergencies_db'
EMERGENCIES_COLLECTION = 'emergencies'
REGISTRATION_QUEUE = 'emergency_registration_queue' # Cola de donde recibe mensajes de asignación/drones

# --- Conexión a MongoDB ---
def connect_to_mongodb(uri, db_name):
    """
    Establece una conexión con MongoDB.
    Reintenta la conexión varias veces si falla.
    """
    client = None
    for i in range(5):
        try:
            logging.info(f"Intentando conectar a MongoDB en {uri} (Intento {i+1}/5)...")
            client = pymongo.MongoClient(uri, serverSelectionTimeoutMS=5000) # Timeout para la selección del servidor
            client.admin.command('ping') # Verificar la conexión
            logging.info("Conexión a MongoDB establecida exitosamente.")
            return client.get_database(db_name)
        except pymongo.errors.ConnectionFailure as e:
            logging.error(f"Fallo al conectar a MongoDB: {e}. Reintentando en {2**(i+1)} segundos...")
            time.sleep(2**(i+1)) # Retardo exponencial
        except Exception as e:
            logging.error(f"Error inesperado al conectar a MongoDB: {e}")
            break
    logging.critical("Fallo definitivo al conectar a MongoDB después de varios reintentos.")
    exit(1) # Salir si no se puede conectar a la base de datos

# --- Conexión a RabbitMQ ---
def connect_to_rabbitmq(url):
    """
    Establece una conexión con RabbitMQ.
    Reintenta la conexión varias veces si falla.
    """
    connection = None
    for i in range(5):
        try:
            logging.info(f"Intentando conectar a RabbitMQ en {url} (Intento {i+1}/5)...")
            parameters = pika.URLParameters(url)
            connection = pika.BlockingConnection(parameters)
            logging.info("Conexión a RabbitMQ establecida exitosamente.")
            return connection
        except pika.exceptions.AMQPConnectionError as e:
            logging.error(f"Fallo al conectar a RabbitMQ: {e}. Reintentando en {2**(i+1)} segundos...")
            time.sleep(2**(i+1)) # Retardo exponencial
        except Exception as e:
            logging.error(f"Error inesperado al conectar a RabbitMQ: {e}")
            break
    logging.critical("Fallo definitivo al conectar a RabbitMQ después de varios reintentos.")
    exit(1) # Salir si no se puede conectar a RabbitMQ

# --- Callback para procesar mensajes de RabbitMQ ---
def callback(ch, method, properties, body, db):
    """
    Función de callback que se ejecuta cuando se recibe un mensaje de RabbitMQ.
    Procesa el mensaje (JSON) y actualiza/inserta la emergencia en MongoDB.
    """
    try:
        emergency_data = json.loads(body)
        emergency_id = emergency_data.get('emergency_id')
        status = emergency_data.get('status')

        if emergency_id is None:
            logging.warning(f"Mensaje recibido sin 'emergency_id'. Ignorando: {body}")
            ch.basic_ack(method.delivery_tag)
            return

        emergencies_collection = db[EMERGENCIES_COLLECTION]

        if status == "En curso":
            # Insertar nueva emergencia (o reemplazar si ya existe con el mismo ID, aunque no debería)
            # Para evitar duplicados en caso de reintentos, usamos upsert.
            # Sin embargo, el problema especifica insertar al principio.
            # `update_one` con `upsert=True` es una forma robusta de manejar "insertar o actualizar".
            # Aquí, solo actualizamos el status y otros campos para la primera vez.
            
            # Buscar si la emergencia ya existe por su ID
            existing_emergency = emergencies_collection.find_one({'emergency_id': emergency_id})
            
            if existing_emergency:
                # Si ya existe, solo actualizamos los campos relevantes si el estado es 'En curso' o si es más reciente
                update_fields = {
                    'name': emergency_data.get('name'),
                    'latitude': emergency_data.get('latitude'),
                    'longitude': emergency_data.get('longitude'),
                    'magnitude': emergency_data.get('magnitude'),
                    'status': status,
                    'last_updated': time.time() # Registrar el último momento de actualización
                }
                result = emergencies_collection.update_one(
                    {'emergency_id': emergency_id},
                    {'$set': update_fields}
                )
                logging.info(f"Emergencia (ID: {emergency_id}) actualizada a 'En curso'. Modificados: {result.modified_count}")
            else:
                # Si no existe, insertar un nuevo documento
                document_to_insert = {
                    'emergency_id': emergency_id,
                    'name': emergency_data.get('name'),
                    'latitude': emergency_data.get('latitude'),
                    'longitude': emergency_data.get('longitude'),
                    'magnitude': emergency_data.get('magnitude'),
                    'status': status,
                    'created_at': time.time(), # Registrar el momento de creación
                    'last_updated': time.time()
                }
                result = emergencies_collection.insert_one(document_to_insert)
                logging.info(f"Nueva emergencia registrada (ID: {emergency_id}, Estado: '{status}'). MongoDB _id: {result.inserted_id}")
            
        elif status == "Extinguido":
            # Actualizar el estado de una emergencia existente a "Extinguido"
            result = emergencies_collection.update_one(
                {'emergency_id': emergency_id},
                {'$set': {'status': status, 'last_updated': time.time()}}
            )
            if result.modified_count > 0:
                logging.info(f"Estado de emergencia (ID: {emergency_id}) actualizado a '{status}'.")
            else:
                logging.warning(f"No se encontró la emergencia (ID: {emergency_id}) para actualizar a '{status}' o ya estaba en ese estado.")
        else:
            logging.warning(f"Estado de emergencia desconocido recibido para ID {emergency_id}: '{status}'. Mensaje: {body}")

        ch.basic_ack(method.delivery_tag) # Confirmar que el mensaje ha sido procesado
    except json.JSONDecodeError as e:
        logging.error(f"Error al decodificar JSON del mensaje: {e}. Mensaje original: {body}")
        ch.basic_nack(method.delivery_tag, requeue=False) # Nack el mensaje, no lo reencolar
    except Exception as e:
        logging.error(f"Error inesperado al procesar mensaje de RabbitMQ: {e}. Mensaje original: {body}")
        ch.basic_nack(method.delivery_tag, requeue=True) # Nack el mensaje y reencolar para reintentar


def main():
    db = connect_to_mongodb(MONGO_DB_URI, DB_NAME)
    connection = connect_to_rabbitmq(RABBITMQURL)

    channel = connection.channel()
    channel.queue_declare(queue=REGISTRATION_QUEUE, durable=False) # La cola no necesita ser durable por ahora

    logging.info(f'[*] Esperando mensajes de emergencia en la cola {REGISTRATION_QUEUE}. Para salir, presiona CTRL+C')

    # Configurar el callback con el objeto db
    # Usamos una función lambda o functools.partial para pasar argumentos adicionales al callback
    channel.basic_consume(
        queue=REGISTRATION_QUEUE,
        on_message_callback=lambda ch, method, properties, body: callback(ch, method, properties, body, db),
        auto_ack=False # Desactivar auto-ack para control manual
    )

    try:
        channel.start_consuming()
    except KeyboardInterrupt:
        logging.info("Deteniendo el servicio de registro de emergencias.")
        channel.stop_consuming()
    except Exception as e:
        logging.critical(f"Error fatal durante el consumo de mensajes: {e}")
    finally:
        if connection:
            connection.close()
            logging.info("Conexión a RabbitMQ cerrada.")

if __name__ == '__main__':
    main()


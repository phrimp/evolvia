import pika
import json
import base64
import logging
from typing import Dict, Any
from datetime import datetime
from config import settings

logger = logging.getLogger(__name__)

class RabbitMQPublisher:
    def __init__(self):
        self.connection = None
        self.channel = None
    
    def connect(self):
        """Establish connection to RabbitMQ"""
        try:
            logger.info(f"Connecting to RabbitMQ at {settings.RABBITMQ_URI}")
            self.connection = pika.BlockingConnection(
                pika.URLParameters(settings.RABBITMQ_URI)
            )
            self.channel = self.connection.channel()
            
            # Declare exchange
            self.channel.exchange_declare(
                exchange=settings.RABBITMQ_EXCHANGE,
                exchange_type='topic',
                durable=True
            )
            
            logger.info(f"Connected to RabbitMQ successfully. Exchange: {settings.RABBITMQ_EXCHANGE}")
            
        except Exception as e:
            logger.error(f"Failed to connect to RabbitMQ: {str(e)}")
            raise
    
    def publish_skill_event(self, content: Dict[str, Any], file_binary: bytes, 
                           filename: str, content_type: str) -> bool:
        """
        Publish input.skill event to RabbitMQ
        """
        try:
            if not self.connection or self.connection.is_closed:
                self.connect()
            
            # Prepare event payload
            event_payload = {
                "event_type": "input.skill",
                "timestamp": datetime.now().isoformat(),
                "service_name": settings.SERVICE_NAME,
                "service_version": settings.SERVICE_VERSION,
                "service_address": settings.SERVICE_ADDRESS,
                "data": {
                    "filename": filename,
                    "content_type": content_type,
                    "extracted_content": content,
                    "file_binary": base64.b64encode(file_binary).decode('utf-8'),
                    "processing_metadata": {
                        "extractor": "python-pptx",
                        "service_name": settings.SERVICE_NAME,
                        "service_version": settings.SERVICE_VERSION,
                        "file_size_bytes": len(file_binary),
                        "processed_at": datetime.now().isoformat()
                    }
                }
            }
            
            # Publish message
            self.channel.basic_publish(
                exchange=settings.RABBITMQ_EXCHANGE,
                routing_key=settings.RABBITMQ_ROUTING_KEY,
                body=json.dumps(event_payload),
                properties=pika.BasicProperties(
                    delivery_mode=2,  # Make message persistent
                    content_type='application/json',
                    headers={
                        'event_type': 'input.skill',
                        'filename': filename,
                        'service_name': settings.SERVICE_NAME,
                        'service_version': settings.SERVICE_VERSION
                    }
                )
            )
            
            logger.info(f"Published skill event for file: {filename} to {settings.RABBITMQ_EXCHANGE}/{settings.RABBITMQ_ROUTING_KEY}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to publish skill event: {str(e)}")
            return False
    
    def close(self):
        """Close RabbitMQ connection"""
        try:
            if self.connection and not self.connection.is_closed:
                self.connection.close()
                logger.info("RabbitMQ connection closed")
        except Exception as e:
            logger.warning(f"Error closing RabbitMQ connection: {str(e)}")

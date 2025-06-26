import pika
import json
import base64
import logging
import time
from typing import Dict, Any, Optional
from datetime import datetime
from config import settings

logger = logging.getLogger(__name__)

class RabbitMQPublisher:
    def __init__(self):
        self.connection = None
        self.channel = None
        self.max_retries = 3
        self.retry_delay = 1  # seconds
    
    def connect(self):
        """Establish connection to RabbitMQ with retry logic"""
        try:
            logger.info(f"Connecting to RabbitMQ at {settings.RABBITMQ_URI}")
            
            # Close existing connection if any
            self._close_connection()
            
            # Create new connection with connection parameters
            parameters = pika.URLParameters(settings.RABBITMQ_URI)
            parameters.heartbeat = 600  # 10 minutes
            parameters.blocked_connection_timeout = 300  # 5 minutes
            
            self.connection = pika.BlockingConnection(parameters)
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
    
    def _ensure_connection(self):
        """Ensure we have a valid connection, reconnect if necessary"""
        try:
            if (self.connection is None or 
                self.connection.is_closed or 
                self.channel is None or 
                self.channel.is_closed):
                logger.info("RabbitMQ connection lost, reconnecting...")
                self.connect()
            else:
                # Test the connection with a heartbeat
                self.connection.process_data_events(time_limit=0)
        except Exception as e:
            logger.warning(f"Connection test failed: {str(e)}, reconnecting...")
            self.connect()
    
    def _close_connection(self):
        """Safely close existing connection"""
        try:
            if self.channel and not self.channel.is_closed:
                self.channel.close()
        except Exception as e:
            logger.warning(f"Error closing channel: {str(e)}")
        
        try:
            if self.connection and not self.connection.is_closed:
                self.connection.close()
        except Exception as e:
            logger.warning(f"Error closing connection: {str(e)}")
        
        self.channel = None
        self.connection = None
    
    def publish_skill_event(self, user_id: str, content: Dict[str, Any], file_binary: bytes, 
                           filename: str, content_type: str, user_email: Optional[str] = None) -> bool:
        """
        Publish input.skill event to RabbitMQ with user context for skill detection
        Includes retry logic for connection failures
        """
        for attempt in range(self.max_retries):
            try:
                # Ensure we have a valid connection
                self._ensure_connection()
                
                # Prepare event payload with user context
                event_payload = {
                    "event_type": "input.skill",
                    "timestamp": datetime.now().isoformat(),
                    "service_name": settings.SERVICE_NAME,
                    "service_version": settings.SERVICE_VERSION,
                    "service_address": settings.SERVICE_ADDRESS,
                    "user_id": user_id,
                    "user_email": user_email,
                    "source": "powerpoint_upload",
                    "source_id": f"{user_id}_{filename}_{int(datetime.now().timestamp())}",
                    "data": {
                        "filename": filename,
                        "content_type": content_type,
                        "extracted_content": content,
                        "file_binary": base64.b64encode(file_binary).decode('utf-8'),
                        "text_for_analysis": content.get("all_text_combined", ""),
                        "processing_metadata": {
                            "extractor": "python-pptx",
                            "service_name": settings.SERVICE_NAME,
                            "service_version": settings.SERVICE_VERSION,
                            "file_size_bytes": len(file_binary),
                            "processed_at": datetime.now().isoformat(),
                            "slide_count": content.get("slide_count", 0),
                            "word_count": content.get("word_count", 0)
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
                            'user_id': user_id,
                            'service_name': settings.SERVICE_NAME,
                            'service_version': settings.SERVICE_VERSION
                        }
                    )
                )
                
                logger.info(f"Published skill event for file: {filename} from user: {user_id} to {settings.RABBITMQ_EXCHANGE}/{settings.RABBITMQ_ROUTING_KEY}")
                return True
                
            except Exception as e:
                logger.error(f"Failed to publish skill event (attempt {attempt + 1}/{self.max_retries}): {str(e)}")
                
                if attempt < self.max_retries - 1:
                    # Close the failed connection
                    self._close_connection()
                    
                    # Wait before retrying
                    time.sleep(self.retry_delay * (attempt + 1))  # Exponential backoff
                    logger.info(f"Retrying RabbitMQ publish in {self.retry_delay * (attempt + 1)} seconds...")
                else:
                    logger.error("All retry attempts failed for RabbitMQ publish")
                    return False
        
        return False
    
    def close(self):
        """Close RabbitMQ connection"""
        try:
            logger.info("Closing RabbitMQ connection...")
            self._close_connection()
            logger.info("RabbitMQ connection closed")
        except Exception as e:
            logger.warning(f"Error closing RabbitMQ connection: {str(e)}")
    
    def health_check(self) -> bool:
        """Check if RabbitMQ connection is healthy"""
        try:
            self._ensure_connection()
            return True
        except Exception as e:
            logger.error(f"RabbitMQ health check failed: {str(e)}")
            return False

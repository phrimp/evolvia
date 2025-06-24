import os
import logging
from pathlib import Path

class Settings:
    # Server Configuration
    SERVICE_NAME = os.getenv("SERVICE_NAME", "input-service")
    SERVICE_ADDRESS = os.getenv("SERVICE_ADDRESS", "input-service")
    SERVICE_VERSION = os.getenv("SERVICE_VERSION", "1.0.0")
    PORT = int(os.getenv("PORT", "9350"))
    HOST = os.getenv("HOST", "0.0.0.0")
    HOSTNAME = os.getenv("HOSTNAME", "input")
    
    # Consul Configuration
    CONSUL_ADDRESS = os.getenv("CONSUL_ADDRESS", "consul-server:8500")
    CONSUL_PORT = int(os.getenv("CONSUL_PORT", "8500"))
    
    # RabbitMQ Configuration
    RABBITMQ_URI = os.getenv("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")
    RABBITMQ_HOST = os.getenv("RABBITMQ_HOST", "rabbitmq")
    RABBITMQ_PORT = int(os.getenv("RABBITMQ_PORT", "5672"))
    RABBITMQ_USER = os.getenv("RABBITMQ_USER", "guest")
    RABBITMQ_PASSWORD = os.getenv("RABBITMQ_PASSWORD", "guest")
    RABBITMQ_VHOST = os.getenv("RABBITMQ_VHOST", "/")
    RABBITMQ_EXCHANGE = os.getenv("RABBITMQ_EXCHANGE", "skills.events")
    RABBITMQ_ROUTING_KEY = os.getenv("RABBITMQ_ROUTING_KEY", "input.skill")
    
    # File Processing Configuration
    MAX_FILE_SIZE = int(os.getenv("MAX_FILE_SIZE", str(50 * 1024 * 1024)))  # 50MB default
    ALLOWED_EXTENSIONS = set(os.getenv("ALLOWED_EXTENSIONS", ".pptx,.ppt").split(","))
    UPLOAD_DIR = os.getenv("UPLOAD_DIR", "/tmp/uploads")
    
    # Logging Configuration
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO").upper()
    LOG_DIR = Path("/evolvia/log/input_service")
    LOG_FILE = LOG_DIR / "input_service.log"
    
    # Application Settings
    DEBUG = os.getenv("DEBUG", "False").lower() == "true"
    
    def __init__(self):
        # Ensure log directory exists
        self.LOG_DIR.mkdir(parents=True, exist_ok=True)

settings = Settings()

# Configure logging to file
def setup_logging():
    """Setup logging configuration to write to files"""
    log_format = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    
    # Create file handler
    file_handler = logging.FileHandler(settings.LOG_FILE)
    file_handler.setLevel(getattr(logging, settings.LOG_LEVEL))
    file_handler.setFormatter(logging.Formatter(log_format))
    
    # Create console handler for development
    console_handler = logging.StreamHandler()
    console_handler.setLevel(logging.INFO)
    console_handler.setFormatter(logging.Formatter(log_format))
    
    # Configure root logger
    logging.basicConfig(
        level=getattr(logging, settings.LOG_LEVEL),
        handlers=[file_handler] + ([console_handler] if settings.DEBUG else [])
    )
    
    return logging.getLogger(__name__)

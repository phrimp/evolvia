from fastapi import FastAPI, File, UploadFile, HTTPException, Header
from fastapi.middleware.cors import CORSMiddleware
import logging
import time
from pathlib import Path
from datetime import datetime
from typing import Optional

from config import settings, setup_logging
from services.ppt_extractor import PowerPointExtractor
from services.rabbitmq_publisher import RabbitMQPublisher
from models import ProcessingResult

# Setup logging
logger = setup_logging()

# Initialize FastAPI app
app = FastAPI(
    title="Input Service",
    description="Microservice for PowerPoint file processing and skill extraction",
    version=settings.SERVICE_VERSION
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure appropriately for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize services
ppt_extractor = PowerPointExtractor()
rabbitmq_publisher = RabbitMQPublisher()

@app.on_event("startup")
async def startup_event():
    """Initialize services on startup"""
    try:
        logger.info(f"Starting {settings.SERVICE_NAME} v{settings.SERVICE_VERSION}")
        logger.info(f"Service configuration: {settings.SERVICE_ADDRESS}:{settings.PORT}")
        
        # Ensure upload directory exists
        Path(settings.UPLOAD_DIR).mkdir(parents=True, exist_ok=True)
        
        # Initialize RabbitMQ connection
        rabbitmq_publisher.connect()
        
        logger.info(f"{settings.SERVICE_NAME} started successfully")
    except Exception as e:
        logger.error(f"Failed to initialize {settings.SERVICE_NAME}: {str(e)}")
        raise

@app.on_event("shutdown")
async def shutdown_event():
    """Cleanup on shutdown"""
    logger.info(f"Shutting down {settings.SERVICE_NAME}")
    rabbitmq_publisher.close()
    logger.info(f"{settings.SERVICE_NAME} shutdown complete")

@app.get("/")
async def root():
    return {
        "service": settings.SERVICE_NAME,
        "version": settings.SERVICE_VERSION,
        "status": "running",
        "timestamp": datetime.now().isoformat()
    }

@app.get("/health")
async def health_check():
    return {
        "status": "healthy",
        "service": settings.SERVICE_NAME,
        "version": settings.SERVICE_VERSION,
        "timestamp": datetime.now().isoformat()
    }

@app.post("/upload-powerpoint", response_model=ProcessingResult)
async def upload_powerpoint(
    file: UploadFile = File(...),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID"),
    x_user_email: Optional[str] = Header(None, alias="X-User-Email")
):
    """
    Upload PowerPoint file, extract content, and publish to RabbitMQ for skill detection
    """
    start_time = time.time()
    
    try:
        logger.info(f"Received PowerPoint upload request: {file.filename} from user: {x_user_id}")
        
        # Validate user context
        if not x_user_id:
            logger.warning("Upload attempt without user context")
            raise HTTPException(status_code=400, detail="User context required (X-User-ID header)")
        
        # Validate file
        if not file.filename:
            logger.warning("Upload attempt with no filename")
            raise HTTPException(status_code=400, detail="No file provided")
        
        file_extension = Path(file.filename).suffix.lower()
        if file_extension not in settings.ALLOWED_EXTENSIONS:
            logger.warning(f"Invalid file extension: {file_extension}")
            raise HTTPException(
                status_code=400, 
                detail=f"File type not supported. Allowed: {settings.ALLOWED_EXTENSIONS}"
            )
        
        # Read file content
        file_content = await file.read()
        
        # Check file size
        if len(file_content) > settings.MAX_FILE_SIZE:
            logger.warning(f"File too large: {len(file_content)} bytes")
            raise HTTPException(
                status_code=413, 
                detail=f"File too large. Max size: {settings.MAX_FILE_SIZE} bytes"
            )
        
        logger.info(f"Processing PowerPoint file: {file.filename} ({len(file_content)} bytes) for user: {x_user_id}")
        
        # Extract content from PowerPoint
        extracted_content = ppt_extractor.extract_content(file_content, file.filename)
        
        # Publish to RabbitMQ with user context
        success = rabbitmq_publisher.publish_skill_event(
            user_id=x_user_id,
            user_email=x_user_email,
            content=extracted_content,
            file_binary=file_content,
            filename=file.filename,
            content_type=file.content_type or "application/vnd.openxmlformats-officedocument.presentationml.presentation"
        )
        
        if not success:
            logger.error(f"Failed to publish RabbitMQ event for {file.filename}")
            raise HTTPException(status_code=500, detail="Failed to publish event to RabbitMQ")
        
        processing_time_ms = int((time.time() - start_time) * 1000)
        
        result = ProcessingResult(
            message="PowerPoint processed successfully and sent for skill analysis",
            filename=file.filename,
            slide_count=extracted_content["slide_count"],
            event_published=True,
            processing_time_ms=processing_time_ms,
            user_id=x_user_id,
            preview={
                "total_slides": extracted_content["slide_count"],
                "word_count": extracted_content.get("word_count", 0),
                "has_text_content": len(extracted_content["all_text_combined"]) > 0,
                "first_slide_preview": (
                    extracted_content["slides"][0]["combined_text"][:200] + "..."
                    if extracted_content["slides"] and extracted_content["slides"][0]["combined_text"]
                    else "No text content found"
                )
            }
        )
        
        logger.info(f"Successfully processed {file.filename} for user {x_user_id}: {extracted_content['slide_count']} slides, {processing_time_ms}ms")
        return result
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error processing PowerPoint {file.filename}: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

if __name__ == "__main__":
    import uvicorn
    logger.info(f"Starting {settings.SERVICE_NAME} on {settings.HOST}:{settings.PORT}")
    uvicorn.run(
        app, 
        host=settings.HOST, 
        port=settings.PORT,
        log_level=settings.LOG_LEVEL.lower()
    )

from pydantic import BaseModel
from typing import Optional, List, Dict, Any
from datetime import datetime

class PowerPointContent(BaseModel):
    filename: str
    slide_count: int
    slides: List[Dict[str, Any]]
    all_text_combined: str
    metadata: Dict[str, Any]

class SkillEvent(BaseModel):
    event_type: str = "input.skill"
    timestamp: datetime
    service_name: str
    service_version: str
    data: Dict[str, Any]
    
    class Config:
        arbitrary_types_allowed = True

class ProcessingResult(BaseModel):
    message: str
    filename: str
    slide_count: int
    event_published: bool
    processing_time_ms: int
    preview: Dict[str, Any]

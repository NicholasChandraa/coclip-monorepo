import os
from typing import Optional
from pydantic import BaseModel
from dotenv import load_dotenv

# Load .env dengan explicit encoding
# utf-8-sig handles BOM (Byte Order Mark) dari file yang pernah disave sebagai UTF-16
load_dotenv(encoding="utf-8-sig")


class Settings(BaseModel):
    # App
    PROJECT_NAME: str = "CoClip Engine"
    API_V1_STR: str = "/api/v1"

    # Whisper Settings
    # Menggunakan model Turbo
    WHISPER_MODEL: str = os.getenv(
        "WHISPER_MODEL", "deepdml/faster-whisper-large-v3-turbo-ct2"
    )
    WHISPER_DEVICE: str = os.getenv(
        "WHISPER_DEVICE", "cuda"
    )  # ganti 'cpu' jika tidak ada GPU
    WHISPER_COMPUTE_TYPE: str = os.getenv(
        "WHISPER_COMPUTE_TYPE", "float16"
    )  # ganti 'int8' jika CPU usage

    # WhisperX Settings
    ENABLE_DIARIZATION: bool = (
        os.getenv("ENABLE_DIARIZATION", "true").lower() == "true"
    )  # Enable/disable speaker detection

    # Gemini / Vertex AI
    GEMINI_API_KEY: str = os.getenv("GEMINI_API_KEY", "")
    GOOGLE_API_KEY: str = os.getenv("GOOGLE_API_KEY", os.getenv("GEMINI_API_KEY", ""))
    GEMINI_PRIMARY_MODEL: str = os.getenv("GEMINI_PRIMARY_MODEL", "gemini-3.1-pro-preview")
    GEMINI_FALLBACK_MODEL: str = os.getenv("GEMINI_FALLBACK_MODEL", "gemini-3-flash-preview")

    # YouTube authentication (fix "Sign in to confirm you're not a bot")
    YOUTUBE_COOKIES_PATH: Optional[str] = os.getenv("YOUTUBE_COOKIES_PATH")
    YOUTUBE_COOKIES_BROWSER: Optional[str] = os.getenv("YOUTUBE_COOKIES_BROWSER")

    # JWT (shared secret dengan auth-service)
    JWT_SECRET_KEY: str = os.getenv("JWT_SECRET_KEY", "")

    # CORS
    CORS_ALLOWED_ORIGINS: list = os.getenv(
        "CORS_ALLOWED_ORIGINS", "http://localhost:3000"
    ).split(",")

    # Redis Configuration (untuk ARQ job queue)
    REDIS_HOST: str = os.getenv("REDIS_HOST", "localhost")
    REDIS_PORT: int = int(os.getenv("REDIS_PORT", "6379"))
    REDIS_DB: int = int(os.getenv("REDIS_DB", "0"))

    # Storage
    TEMP_DIR: str = os.path.join(os.getcwd(), "temp")
    CLIPS_DIR: str = os.path.join(os.getcwd(), "clips")

    # Database
    DATABASE_URL: str = os.getenv(
        "DATABASE_URL",
        "postgresql+asyncpg://postgres:postgres@localhost:5434/model_clip",
    )

    # Video Output Format (tiktok, reels, shorts, square, landscape)
    OUTPUT_FORMAT: str = os.getenv("OUTPUT_FORMAT", "tiktok")

    # Subtitle Settings
    SUBTITLE_UPPERCASE: bool = os.getenv("SUBTITLE_UPPERCASE", "true").lower() == "true"

    # Smart Crop (Face Tracking)
    ENABLE_SMART_CROP: bool = os.getenv("ENABLE_SMART_CROP", "true").lower() == "true"
    CROP_MODE: str = os.getenv("CROP_MODE", "smart")  # "smart" | "center" | "none"

    # Hook Generation
    ENABLE_HOOKS: bool = os.getenv("ENABLE_HOOKS", "true").lower() == "true"

    # TTS
    TTS_DATA_DIR: str = os.path.join(os.getcwd(), "model-tts")

    # Auth-service (for social upload token retrieval)
    AUTH_SERVICE_URL: str = os.getenv("AUTH_SERVICE_URL", "http://localhost:8005")
    AUTH_SERVICE_TOKEN: str = os.getenv("AUTH_SERVICE_TOKEN", "")

    # TikTok Playwright — persistent Chromium session directory
    TIKTOK_USER_DATA_DIR: str = os.getenv(
        "TIKTOK_USER_DATA_DIR",
        os.path.join(os.getcwd(), "tiktok_session"),
    )


settings = Settings()

# Log settings saat module di-import (sekali saja)
import logging as _logging
_cfg_logger = _logging.getLogger("coclip")
_cfg_logger.info(
    f"Settings loaded: device={settings.WHISPER_DEVICE}, "
    f"compute={settings.WHISPER_COMPUTE_TYPE}, "
    f"format={settings.OUTPUT_FORMAT}, "
    f"smart_crop={settings.ENABLE_SMART_CROP}, "
    f"diarization={settings.ENABLE_DIARIZATION}"
)

"""Social media upload endpoints."""

import asyncio
import os
import shutil
import uuid
from datetime import datetime, timezone

import httpx
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy import select

from app.core.config import settings
from app.core.database import async_session
from app.middleware.auth import CurrentUser, get_current_user
from app.models import Clip, ClipUpload
from app.utils.youtube_uploader import upload_to_youtube
from app.utils.tiktok_playwright_uploader import (
    import_cookies_from_json,
    is_tiktok_session_valid,
    setup_tiktok_session,
    upload_to_tiktok_playwright,
)
from app.utils.logging import logger

router = APIRouter()


class UploadRequest(BaseModel):
    clip_id: str
    platform: str  # "youtube"
    title: str
    description: str | None = None
    tags: list[str] | None = None
    privacy: str = "private"  # public | unlisted | private
    scheduled_time: datetime | None = None


@router.get("/social/tiktok/connected")
async def tiktok_connected(
    current_user: CurrentUser = Depends(get_current_user),
):
    """Return whether a TikTok Playwright session exists on this server."""
    return {"connected": is_tiktok_session_valid(settings.TIKTOK_USER_DATA_DIR)}


@router.post("/social/tiktok/setup")
async def tiktok_setup(
    current_user: CurrentUser = Depends(get_current_user),
):
    """
    Open a headed Chromium window on the server for manual TikTok login.
    Blocks until the user logs in (up to 3 minutes), then saves the session.
    """
    logger.info(f"[TikTok setup] initiated by user {current_user.user_id}")
    try:
        await setup_tiktok_session(settings.TIKTOK_USER_DATA_DIR)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"TikTok setup failed: {e}")
    return {"status": "ok", "message": "TikTok session saved successfully"}


@router.post("/social/tiktok/import-cookies")
async def tiktok_import_cookies(
    payload: dict,
    current_user: CurrentUser = Depends(get_current_user),
):
    """
    Accept a Cookie-Editor JSON export and save it as a Playwright session.
    Payload: { "cookies_json": "<raw JSON string from Cookie-Editor>" }
    """
    raw = payload.get("cookies_json", "")
    if not raw:
        raise HTTPException(status_code=400, detail="cookies_json is required")
    try:
        count = import_cookies_from_json(settings.TIKTOK_USER_DATA_DIR, raw)
    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))
    return {"status": "ok", "cookies_saved": count}


@router.delete("/social/tiktok/session")
async def tiktok_disconnect(
    current_user: CurrentUser = Depends(get_current_user),
):
    """Delete the saved TikTok Playwright session so the user can log in with a different account."""
    user_data_dir = settings.TIKTOK_USER_DATA_DIR
    if os.path.exists(user_data_dir):
        shutil.rmtree(user_data_dir)
        logger.info(f"[TikTok] session deleted by user {current_user.user_id}")
    return {"status": "ok", "message": "TikTok session removed"}


@router.post("/social/upload")
async def start_upload(
    req: UploadRequest,
    current_user: CurrentUser = Depends(get_current_user),
):
    """Trigger a clip upload to a social platform. Returns upload_id for polling."""
    async with async_session() as session:
        # Verify clip exists
        result = await session.execute(
            select(Clip).where(Clip.clip_id == req.clip_id)
        )
        clip = result.scalar_one_or_none()
        if not clip:
            raise HTTPException(status_code=404, detail="Clip not found")

        # TikTok uses Playwright (no OAuth token needed)
        if req.platform == "tiktok":
            if not is_tiktok_session_valid(settings.TIKTOK_USER_DATA_DIR):
                raise HTTPException(
                    status_code=400,
                    detail="TikTok not set up. Please run POST /api/v1/social/tiktok/setup first.",
                )
            access_token = None
            open_id = None
            platform_username = None
        else:
            # Get valid access token from auth-service
            try:
                token_data = await _get_token(current_user.user_id, req.platform)
                access_token = token_data["access_token"]
                open_id = token_data.get("open_id")
                platform_username = token_data.get("platform_username")
            except Exception as e:
                raise HTTPException(status_code=400, detail=str(e))

        # Create upload record
        upload_id = str(uuid.uuid4())
        upload = ClipUpload(
            id=upload_id,
            clip_id=req.clip_id,
            user_id=current_user.user_id,
            platform=req.platform,
            status="uploading",
            title=req.title,
            description=req.description,
            tags=req.tags,
            privacy=req.privacy,
            scheduled_time=req.scheduled_time,
        )
        session.add(upload)
        await session.commit()
        clip_path = clip.file_path

    # Fire background task (don't await — returns immediately)
    asyncio.create_task(
        _run_upload(
            upload_id=upload_id,
            access_token=access_token,
            open_id=open_id,
            platform_username=platform_username,
            clip_path=clip_path,
            req=req,
        )
    )

    return {"upload_id": upload_id, "status": "uploading"}


@router.get("/social/upload/{upload_id}")
async def get_upload_status(
    upload_id: str,
    current_user: CurrentUser = Depends(get_current_user),
):
    """Poll upload status by upload_id."""
    async with async_session() as session:
        upload = await session.get(ClipUpload, upload_id)
        if not upload or upload.user_id != current_user.user_id:
            raise HTTPException(status_code=404, detail="Upload not found")
        return {
            "upload_id": upload.id,
            "status": upload.status,
            "platform_video_id": upload.platform_video_id,
            "platform_url": upload.platform_url,
            "error": upload.error,
        }


@router.get("/social/uploads")
async def list_uploads(
    current_user: CurrentUser = Depends(get_current_user),
):
    """List all upload history for the current user."""
    async with async_session() as session:
        result = await session.execute(
            select(ClipUpload)
            .where(ClipUpload.user_id == current_user.user_id)
            .order_by(ClipUpload.created_at.desc())
            .limit(50)
        )
        uploads = result.scalars().all()
        return [
            {
                "upload_id": u.id,
                "clip_id": u.clip_id,
                "platform": u.platform,
                "status": u.status,
                "platform_url": u.platform_url,
                "title": u.title,
                "created_at": u.created_at.isoformat() if u.created_at else None,
            }
            for u in uploads
        ]


async def _get_token(user_id: str, platform: str) -> str:
    """Fetch a valid access token from auth-service internal endpoint."""
    async with httpx.AsyncClient(timeout=10.0) as client:
        resp = await client.get(
            f"{settings.AUTH_SERVICE_URL}/api/v1/internal/social/token/{user_id}/{platform}",
            headers={"X-Service-Token": settings.AUTH_SERVICE_TOKEN},
        )
        if resp.status_code == 404:
            raise HTTPException(
                status_code=400,
                detail=f"{platform} account not connected. Go to Settings to connect.",
            )
        resp.raise_for_status()
        return resp.json()


async def _run_upload(
    upload_id: str,
    access_token: str,
    open_id: str | None,
    platform_username: str | None,
    clip_path: str,
    req: UploadRequest,
) -> None:
    """Background task: upload video and update ClipUpload status."""
    async with async_session() as session:
        try:
            if req.platform == "youtube":
                result = await upload_to_youtube(
                    access_token=access_token,
                    clip_path=clip_path,
                    title=req.title,
                    description=req.description or "",
                    tags=req.tags or [],
                    privacy=req.privacy,
                    scheduled_time=req.scheduled_time,
                )
                platform_video_id = result["video_id"]
                platform_url = result["url"]
            elif req.platform == "tiktok":
                caption = req.description or req.title
                if req.tags:
                    # Normalize tags (strip leading # to avoid ##tag)
                    # Only append tags not already present in the caption
                    existing = caption.lower()
                    new_tags = [
                        f"#{t.lstrip('#')}"
                        for t in req.tags
                        if t.lstrip('#').lower() not in existing
                    ]
                    if new_tags:
                        caption += "\n" + " ".join(new_tags)

                result = await upload_to_tiktok_playwright(
                    clip_path=clip_path,
                    title=caption,
                    user_data_dir=settings.TIKTOK_USER_DATA_DIR,
                )

                platform_video_id = None
                platform_url = result.get("url", "https://www.tiktok.com")
            else:
                raise Exception(f"Unsupported platform: {req.platform}")

            upload = await session.get(ClipUpload, upload_id)
            if upload:
                upload.status = "completed"
                upload.platform_video_id = platform_video_id
                upload.platform_url = platform_url
                upload.completed_at = datetime.now(timezone.utc)
                await session.commit()
            logger.info(f"Upload completed for {req.platform}")
        except Exception as e:
            logger.error(f"Upload failed for {upload_id}: {e}")
            async with async_session() as err_session:
                upload = await err_session.get(ClipUpload, upload_id)
                if upload:
                    upload.status = "failed"
                    upload.error = str(e)
                    await err_session.commit()

import httpx
import os
import asyncio
import math
import json
import logging

logger = logging.getLogger(__name__)

async def upload_to_tiktok(
    access_token: str,
    open_id: str,
    clip_path: str,
    title: str,
    privacy_level: str = "SELF_ONLY",
) -> dict:
    """
    Uploads a video to TikTok using the Content Posting API v2.
    
    Args:
        access_token: The TikTok access token for the user.
        open_id: The TikTok open_id for the user.
        clip_path: Local path to the physical video file.
        title: Title/caption for the TikTok video.
        privacy_level: "PUBLIC", "MUTUAL_FOLLOW", "FOLLOWER_OF_CREATOR", or "SELF_ONLY".
        
    Returns:
        dict: A dictionary containing the upload status and any relevant IDs.
    """
    
    if not os.path.exists(clip_path):
        raise FileNotFoundError(f"Clip file not found: {clip_path}")

    file_size = os.path.getsize(clip_path)

    # Note: Unaudited apps can only post SELF_ONLY. We'll leave it configurable but default to SELF_ONLY.
    init_url = "https://open.tiktokapis.com/v2/post/publish/video/init/"
    
    headers = {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json; charset=UTF-8",
    }
    
    # 1. Initialize the upload
    # TikTok V2 Requirements:
    # - chunk_size: 5MB to 64MB (last can be up to 128MB)
    # - total_chunk_count: floor(video_size / chunk_size)
    # We'll use 10MB as our base chunk size.
    base_chunk_size = 10 * 1024 * 1024 # 10MB
    
    if file_size <= base_chunk_size:
        total_chunks = 1
        request_chunk_size = file_size
    else:
        total_chunks = math.floor(file_size / base_chunk_size)
        request_chunk_size = base_chunk_size

    init_payload = {
        "post_info": {
            "title": title[:2000] if title else "Video from Coclip",
            "privacy_level": privacy_level,
            "disable_comment": False,
            "disable_duet": False,
            "disable_stitch": False,
        },
        "source_info": {
            "source": "FILE_UPLOAD",
            "video_size": file_size,
            "chunk_size": request_chunk_size,
            "total_chunk_count": total_chunks
        }
    }
    logger.info(f"Initializing TikTok upload for open_id {open_id}. Payload: {json.dumps(init_payload)}")
    
    async with httpx.AsyncClient(timeout=300.0) as client:
        init_resp = await client.post(init_url, json=init_payload, headers=headers)
        
        if init_resp.status_code != 200:
            error_data = init_resp.text
            try:
                error_data = init_resp.json()
            except Exception:
                pass
            logger.error(f"TikTok init upload failed: {init_resp.status_code} - {error_data}")
            raise Exception(f"Failed to initialize TikTok upload: {error_data}")

        init_data = init_resp.json()
        
        if init_data.get("error", {}).get("code") != "ok" or not init_data.get("data"):
             error_msg = init_data.get("error", {}).get("message", "Unknown error")
             logger.error(f"TikTok init API error: {error_msg}")
             raise Exception(f"TikTok initialization failed: {error_msg}")

        publish_id = init_data["data"]["publish_id"]
        upload_url = init_data["data"]["upload_url"]

        logger.info(f"Uploading file chunks to TikTok for publish_id: {publish_id}")
        
        # 2. Upload the file chunks to the provided upload_url
        with open(clip_path, "rb") as f:
            for i in range(total_chunks):
                if i == total_chunks - 1:
                    # Last chunk is "oversized" and contains all remaining bytes
                    chunk_data = f.read()
                else:
                    chunk_data = f.read(base_chunk_size)
                
                start_byte = i * base_chunk_size
                end_byte = start_byte + len(chunk_data) - 1
                
                upload_headers = {
                    "Content-Type": "video/mp4",
                    "Content-Range": f"bytes {start_byte}-{end_byte}/{file_size}",
                }
                
                upload_resp = await client.put(upload_url, content=chunk_data, headers=upload_headers)
                
                if upload_resp.status_code not in (200, 201, 204, 206, 308): # 206 is Partial Content, which is expected for chunks
                    logger.error(f"TikTok file chunk {i+1} upload failed: {upload_resp.status_code} - {upload_resp.text}")
                    raise Exception(f"Failed to upload file chunks to TikTok: {upload_resp.text}")
            
        logger.info(f"Successfully uploaded video to TikTok. Publish ID: {publish_id}")
        
        # 3. Poll for the final video_id (optional but helpful for working links)
        video_id = None
        max_retries = 30 # Up to 3 minutes coverage
        for i in range(max_retries):
            await asyncio.sleep(6) # TikTok processing takes time
            try:
                status_data = await get_tiktok_publish_status(access_token, publish_id)
                if status_data.get("error", {}).get("code") == "ok":
                    data = status_data.get("data", {})
                    status = data.get("status")
                    logger.info(f"TikTok status check for {publish_id}: {json.dumps(status_data)}")
                    if status in ("SUCCESS", "PUBLISH_COMPLETE"):
                        # Try multiple possible ID fields. TikTok V2 uses publicaly_available_post_id
                        video_id = data.get("publicaly_available_post_id") or data.get("public_item_id") or data.get("video_id") or data.get("item_id")
                        if video_id:
                            logger.info(f"TikTok video is now public. Video ID: {video_id}")
                        else:
                            # Note: Unaudited apps posting SELF_ONLY might not get a public ID
                            logger.warning(f"TikTok status is {status} but no public ID found (likely private). Data: {json.dumps(data)}")
                        break
                    elif status == "FAILED":
                        reason = data.get("fail_reason", "Unknown reason")
                        logger.error(f"TikTok video processing failed: {reason}")
                        break
                    else:
                        logger.info(f"TikTok video still processing (Status: {status}). Retry {i+1}/{max_retries}")
                else:
                    logger.warning(f"TikTok status API returned error: {status_data}")
            except Exception as e:
                logger.warning(f"Error polling TikTok status: {e}")
        
        return {
            "status": "success",
            "publish_id": publish_id,
            "video_id": video_id
        }

async def get_tiktok_publish_status(access_token: str, publish_id: str) -> dict:
    """
    Queries the publish status of a TikTok video using the publish_id.
    
    Returns:
        dict: The response from the TikTok Status API.
    """
    url = "https://open.tiktokapis.com/v2/post/publish/status/fetch/"
    headers = {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json; charset=UTF-8",
    }
    payload = {"publish_id": publish_id}
    
    async with httpx.AsyncClient(timeout=30.0) as client:
        resp = await client.post(url, json=payload, headers=headers)
        if resp.status_code != 200:
            logger.error(f"TikTok status fetch failed: {resp.status_code} - {resp.text}")
            return {"error": resp.text}
        
        return resp.json()

        return resp.json()

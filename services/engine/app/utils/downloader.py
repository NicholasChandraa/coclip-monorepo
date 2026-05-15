"""
YouTube Video Downloader - yt-dlp integration for Coclip.

Downloads YouTube videos to local storage for pipeline processing.
"""

import os
import re
import json
import asyncio
import signal
import tempfile
from typing import Optional, Callable
from dataclasses import dataclass

from app.utils.logging import logger
from app.core.config import settings

# Global reference to active download process for cancellation
_active_downloads: dict = {}  # job_id -> yt_dlp.YoutubeDL instance


def _ensure_node_in_path():
    """Ensure Node.js is in PATH for yt-dlp signature solving."""
    # Common Node.js paths on Windows (especially for this user)
    node_paths = [
        "C:\\nvm4w\\nodejs",
        "C:\\Program Files\\nodejs",
        os.path.expandvars("%AppData%\\npm"),
    ]
    path_env = os.environ.get("PATH", "")
    for p in node_paths:
        if os.path.exists(p) and p not in path_env:
            os.environ["PATH"] = p + os.pathsep + os.environ["PATH"]
            path_env = os.environ["PATH"]


def _get_cookies_path() -> Optional[str]:
    """
    Return a valid Netscape-format cookies path for yt-dlp.
    If the configured file is JSON (Cookie-Editor format), auto-convert it
    to a temp Netscape file so yt-dlp can read it.
    """
    path = settings.YOUTUBE_COOKIES_PATH
    if not path or not os.path.exists(path):
        return path

    with open(path, "r", encoding="utf-8") as f:
        first_char = f.read(1)

    if first_char != "[" and first_char != "{":
        # Already Netscape format
        return path

    # JSON format (Cookie-Editor export) — convert to Netscape
    logger.info("[Cookies] Detected JSON cookies file, converting to Netscape format…")
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)

    if isinstance(data, dict) and "cookies" in data:
        cookies = data["cookies"]
    elif isinstance(data, list):
        cookies = data
    else:
        logger.warning("[Cookies] Unrecognised JSON cookie format, skipping")
        return None

    lines = ["# Netscape HTTP Cookie File\n"]
    for c in cookies:
        domain = c.get("domain", "")
        include_sub = "TRUE" if domain.startswith(".") else "FALSE"
        path_val = c.get("path", "/")
        secure = "TRUE" if c.get("secure", False) else "FALSE"
        expiry = int(c.get("expirationDate", c.get("expires", 0)) or 0)
        name = c.get("name", "")
        value = c.get("value", "")
        lines.append(f"{domain}\t{include_sub}\t{path_val}\t{secure}\t{expiry}\t{name}\t{value}\n")

    tmp = tempfile.NamedTemporaryFile(
        mode="w", suffix=".txt", delete=False, encoding="utf-8"
    )
    tmp.writelines(lines)
    tmp.close()
    logger.info(f"[Cookies] Converted {len(cookies)} cookies → {tmp.name}")
    return tmp.name


@dataclass
class DownloadResult:
    """Result of a YouTube video download."""

    file_path: str
    title: str
    duration: float  # seconds
    uploader: str
    file_size: int  # bytes


# Regex patterns for YouTube URL validation
YOUTUBE_PATTERNS = [
    r"^https?://(www\.)?youtube\.com/watch\?v=[\w-]+",
    r"^https?://youtu\.be/[\w-]+",
    r"^https?://(www\.)?youtube\.com/shorts/[\w-]+",
]


def validate_youtube_url(url: str) -> bool:
    """Check if URL is a valid YouTube URL."""
    return any(re.match(pattern, url) for pattern in YOUTUBE_PATTERNS)


async def get_video_info(url: str) -> Optional[dict]:
    """
    Fetch video metadata without downloading, including estimated file size.

    Uses the same format selector as download_video so the size estimate
    reflects what will actually be downloaded.

    Returns:
        dict with title, duration, uploader, resolution, estimated_size_bytes
    """
    import yt_dlp

    _ensure_node_in_path()

    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
        "skip_download": True,
        "noplaylist": True,
        "ignoreconfig": True,
        "js_runtimes": {"node": {"path": "C:/nvm4w/nodejs/node.exe"}},
        "remote_components": {"ejs:github"},
        "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "extractor_args": {
            "youtube": {
                "player_client": ["android_vr"],
            }
        },
    }

    # Add cookie options if configured
    cookies_path = _get_cookies_path()
    if cookies_path:
        ydl_opts["cookiefile"] = cookies_path
    if settings.YOUTUBE_COOKIES_BROWSER:
        ydl_opts["cookiesfrombrowser"] = (settings.YOUTUBE_COOKIES_BROWSER,)

    def _extract():
        import subprocess
        try:
            node_v = subprocess.check_output(["node", "-v"], text=True).strip()
            logger.info(f"  [GET_INFO] Found Node.js: {node_v} | yt-dlp version: {yt_dlp.version.__version__}")
        except Exception as e:
            logger.warning(f"  [GET_INFO] Runtime check: {e}")

        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            # use process=False to get basic info without resolving formats
            # This ensures info is always returned even if formats fail resolution
            return ydl.extract_info(url, download=False, process=False)

    try:
        info = await asyncio.to_thread(_extract)

        # Sum filesize across all requested formats (video + audio tracks)
        estimated_bytes: Optional[int] = None
        requested = info.get("requested_formats") or []
        if requested:
            total = 0
            for fmt in requested:
                sz = fmt.get("filesize") or fmt.get("filesize_approx") or 0
                total += sz
            if total > 0:
                estimated_bytes = total
        if estimated_bytes is None:
            # Fallback: single-format download
            estimated_bytes = info.get("filesize") or info.get("filesize_approx")

        result = {
            "title": info.get("title", "Unknown"),
            "duration": info.get("duration", 0),
            "uploader": info.get("uploader", "Unknown"),
            "thumbnail": info.get("thumbnail"),
            "width": info.get("width"),
            "height": info.get("height"),
            "estimated_size_bytes": estimated_bytes,
        }

        logger.info(
            f"[VIDEO YOUTUBE INFO]: {result['title']} "
            f"({result['duration']}s, "
            f"~{(estimated_bytes or 0) / 1024 / 1024:.1f} MB)"
        )

        return result
    except Exception as e:
        logger.error(f"Failed to get video info: {e}")
        return None


async def download_video(
    url: str,
    output_dir: str,
    job_id: str,
    progress_callback: Optional[Callable] = None,
) -> DownloadResult:
    """
    Download YouTube video using yt-dlp.

    Args:
        url: YouTube video URL
        output_dir: Directory to save the video
        job_id: Job ID used as filename
        progress_callback: Optional async callback(percent: float) for progress updates

    Returns:
        DownloadResult with file path and metadata

    Raises:
        Exception if download fails
    """
    import yt_dlp

    os.makedirs(output_dir, exist_ok=True)
    output_path = os.path.join(output_dir, f"{job_id}.mp4")
    output_template = os.path.join(output_dir, f"{job_id}.%(ext)s")

    last_percent = [0.0]

    def _progress_hook(d):
        if d["status"] == "downloading":
            pct_str = d.get("_percent_str", "0%").strip().replace("%", "")
            try:
                pct = float(pct_str)
                if pct - last_percent[0] >= 5:  # update every 5%
                    last_percent[0] = pct
                    logger.info(
                        f"  [Job {job_id}] Downloading: {pct:.0f}% "
                        f"| Speed: {d.get('_speed_str', '?')} "
                        f"| ETA: {d.get('_eta_str', '?')}"
                    )
            except ValueError:
                pass
        elif d["status"] == "finished":
            logger.info(f"  [Job {job_id}] Download finished, merging...")

    _ensure_node_in_path()
    # Explicit Node.js path for EJS challenge solver (yt-dlp 2026+)
    node_path = "C:/nvm4w/nodejs/node.exe"

    # WHY android_vr only:
    #   - web client:    n-challenge solving fails with Node.js v24 (EJS returncode:1) → only storyboard images
    #   - tv client:     DRM experiment active on some videos → formats skipped
    #   - android:       requires GVS PO Token → formats skipped
    #   - android_vr:    jsless client (no n-challenge needed), provides full DASH up to 1080p ✓
    ydl_opts = {
        # Prefer 1080p; format_sort avoids 4K/1440p (large file, no quality gain after no-upscale crop)
        "format": "bestvideo+bestaudio/best",
        "format_sort": ["res:1080", "codec:h264", "ext:mp4"],
        "outtmpl": output_template,
        "merge_output_format": "mp4",
        "quiet": True,
        "no_warnings": True,
        "progress_hooks": [_progress_hook],
        "ignoreconfig": True,
        # js_runtimes format: dict of {runtime: config_dict_or_None}
        "js_runtimes": {"node": {"path": node_path}},
        # Download EJS challenge solver script from GitHub (needed even if n-challenge ultimately fails)
        "remote_components": {"ejs:github"},
        "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "extractor_args": {
            "youtube": {
                "player_client": ["android_vr"],
            }
        },
    }

    # Add cookie options if configured (disabled by default — cookies cause worse results with android_vr)
    cookies_path = _get_cookies_path()
    if cookies_path:
        ydl_opts["cookiefile"] = cookies_path
    if settings.YOUTUBE_COOKIES_BROWSER:
        ydl_opts["cookiesfrombrowser"] = (settings.YOUTUBE_COOKIES_BROWSER,)

    def _download(use_cookies=True):
        opts = ydl_opts.copy()
        if not use_cookies:
            opts.pop("cookiefile", None)
            opts.pop("cookiesfrombrowser", None)
            # android_vr works fine without cookies for public videos
            logger.info(f"  [Job {job_id}] Retrying download WITHOUT cookies...")
        else:
            # Stage 1: Try with cookies + full clients
            import subprocess
            try:
                node_v = subprocess.check_output(["node", "-v"], text=True).strip()
                logger.info(f"  [Job {job_id}] Found Node.js: {node_v}")
            except Exception as e:
                logger.warning(f"  [Job {job_id}] Node.js not found: {e}")

        with yt_dlp.YoutubeDL(opts) as ydl:
            _active_downloads[job_id] = ydl
            try:
                return ydl.extract_info(url, download=True)
            except Exception as e:
                # If we were using cookies, try one more time without them
                if use_cookies:
                    logger.warning(f"  [Job {job_id}] Download with cookies failed, attempting mobile fallback: {e}")
                    return _download(use_cookies=False)
                
                # If already failed without cookies, propagate clean error
                error_msg = str(e)
                if "No video formats found" in error_msg:
                    error_msg = "YouTube blocked this video's formats. Mobile fallback also failed."
                raise RuntimeError(error_msg) from None
            finally:
                _active_downloads.pop(job_id, None)

    logger.info(f"[Job {job_id}] Starting YouTube download: {url}")

    info = await asyncio.to_thread(_download)

    # yt-dlp may save with different extension before merging
    if not os.path.exists(output_path):
        # Check for webm or other format
        for ext in ["mp4", "mkv", "webm"]:
            candidate = os.path.join(output_dir, f"{job_id}.{ext}")
            if os.path.exists(candidate):
                output_path = candidate
                break

    if not os.path.exists(output_path):
        raise FileNotFoundError(
            f"Downloaded file not found at {output_path}"
        )

    file_size = os.path.getsize(output_path)
    title = info.get("title", "Unknown")
    duration = info.get("duration", 0)
    uploader = info.get("uploader", "Unknown")
    width = info.get("width") or info.get("requested_downloads", [{}])[0].get("width")
    height = info.get("height") or info.get("requested_downloads", [{}])[0].get("height")
    resolution = f"{width}x{height}" if width and height else "unknown resolution"

    logger.info(
        f"[Job {job_id}] Download complete: "
        f"'{title}' ({duration:.0f}s, {file_size / 1024 / 1024:.1f} MB, {resolution})"
    )

    return DownloadResult(
        file_path=output_path,
        title=title,
        duration=duration,
        uploader=uploader,
        file_size=file_size,
    )


def cancel_download(job_id: str) -> bool:
    """Cancel an active download for a job.

    Returns True if a download was found and cancelled.
    """
    ydl = _active_downloads.pop(job_id, None)
    if ydl is not None:
        try:
            # yt-dlp checks _download_retcode to abort
            ydl._download_retcode = 1
            logger.info(f"[Job {job_id}] Download cancelled")
            return True
        except Exception as e:
            logger.warning(f"[Job {job_id}] Failed to cancel download: {e}")
    return False

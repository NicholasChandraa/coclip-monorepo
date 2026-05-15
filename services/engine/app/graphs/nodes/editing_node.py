"""
Video editing node for LangGraph video processing pipeline.

Implements Phase 3B: FFmpeg video editing (58% → 80%)
Cuts video clips, burns word-level subtitles, and prepends hook intros.
"""

import os
import json
import asyncio
from typing import Literal, Optional, List, Tuple
from langgraph.types import Command
from app.schemas.graph_schemas import VideoProcessingState
from app.schemas.transcription import TranscriptionResultDetailed
from app.utils.progress_tracker import create_progress_tracker
from app.utils.abort_checker import AbortError, check_aborted, raise_if_aborted
from app.utils.subtitle_generator import generate_ass_subtitle
from app.utils.video_formats import get_format, VideoFormat
from app.utils.tts_engine import TTSEngine
from app.utils.logging import logger
from app.core.config import settings
from redis import asyncio as aioredis
from app.utils.filename import sanitize_filename


async def _detect_video_resolution(video_path: str) -> Tuple[int, int]:
    """
    Detect video resolution using ffprobe.

    Args:
        video_path: Path to video file

    Returns:
        Tuple of (width, height)
    """
    cmd = [
        "ffprobe",
        "-v",
        "error",
        "-select_streams",
        "v:0",
        "-show_entries",
        "stream=width,height",
        "-of",
        "json",
        video_path,
    ]

    try:
        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()

        if process.returncode == 0:
            data = json.loads(stdout.decode())
            stream = data.get("streams", [{}])[0]
            width = stream.get("width", 1920)
            height = stream.get("height", 1080)
            return (width, height)
        else:
            logger.warning(f"ffprobe failed, using default 1920x1080")
            return (1920, 1080)
    except Exception as e:
        logger.warning(f"ffprobe error: {e}, using default 1920x1080")
        return (1920, 1080)


async def _detect_audio_sample_rate(video_path: str) -> int:
    """
    Detect audio sample rate using ffprobe.
    
    Args:
        video_path: Path to video file
        
    Returns:
        Sample rate in Hz (e.g., 44100, 48000). Falls back to 48000 on error.
    """
    cmd = [
        "ffprobe",
        "-v",
        "error",
        "-select_streams",
        "a:0",
        "-show_entries",
        "stream=sample_rate",
        "-of",
        "json",
        video_path,
    ]

    try:
        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()

        if process.returncode == 0:
            data = json.loads(stdout.decode())
            stream = data.get("streams", [{}])[0]
            # Some files might not have sample_rate or return "N/A"
            rate_str = stream.get("sample_rate", "48000")
            if rate_str and rate_str.isdigit():
                return int(rate_str)
        return 48000
    except Exception as e:
        logger.warning(f"ffprobe audio det error: {e}, using default 48000")
        return 48000


def escape_ffmpeg_text(text: str) -> str:
    """Melakukan escaping ketat pada karakter khusus agar FFmpeg drawtext tidak rusak."""
    if not text:
        return ""
    # Backslash harus luput duluan supaya tidak numpuk
    text = text.replace('\\', '\\\\')
    text = text.replace(':', '\\:')
    text = text.replace("'", "\\'")
    text = text.replace('%', '\\%')
    # Actual newline → FFmpeg drawtext \n escape (AFTER backslash escaping so
    # the new \ is not double-escaped)
    text = text.replace('\n', '\\n')
    return text


def _calculate_crop_filter(
    input_width: int,
    input_height: int,
    target_format: VideoFormat,
    crop_x_override: Optional[int] = None,
) -> Optional[str]:
    """
    Calculate FFmpeg crop+scale filter to convert video to target format.

    Args:
        input_width: Input video width
        input_height: Input video height
        target_format: Target video format
        crop_x_override: If provided, use this X position instead of center
                         (for smart crop following active speaker)

    Returns:
        FFmpeg filter string or None if no crop needed
    """
    target_w = target_format.width
    target_h = target_format.height
    target_ratio = target_w / target_h
    input_ratio = input_width / input_height

    # If already correct ratio, just scale
    if abs(input_ratio - target_ratio) < 0.01:
        if input_width != target_w or input_height != target_h:
            return f"scale={target_w}:{target_h}:flags=lanczos"
        return None

    # Need to crop
    if input_ratio > target_ratio:
        # Input is wider → crop width (landscape → portrait)
        crop_h = input_height
        crop_w = int(input_height * target_ratio)
        crop_x = (
            crop_x_override
            if crop_x_override is not None
            else (input_width - crop_w) // 2
        )
        # Clamp to valid range
        crop_x = max(0, min(crop_x, input_width - crop_w))
        crop_y = 0
    else:
        # Input is taller → crop height (portrait → landscape)
        crop_w = input_width
        crop_h = int(input_width / target_ratio)
        crop_x = 0
        crop_y = (input_height - crop_h) // 2

    # Force scale using Lanczos to ensure output is strictly target resolution (1080x1920)
    filter_str = f"crop={crop_w}:{crop_h}:{crop_x}:{crop_y},scale={target_w}:{target_h}:flags=lanczos"
    label = "🎯 Smart" if crop_x_override is not None else "📐"
    logger.info(
        f"{label} Crop {input_width}x{input_height} → {target_w}x{target_h}: {filter_str}"
    )
    return filter_str


def _build_keyframe_crop_filter(
    keyframes: list,
    input_width: int,
    input_height: int,
    target_format: "VideoFormat",
    transition_duration: float,
) -> Optional[str]:
    """
    Build FFmpeg crop filter with animated x-position from keyframes.
    Smoothly interpolates between keyframe positions.

    Args:
        keyframes: List of CropKeyframe(time, crop_x)
        input_width: Input video width
        input_height: Input video height
        target_format: Target video format
        transition_duration: Seconds to lerp between positions

    Returns:
        FFmpeg filter string with animated crop expression
    """
    target_w = target_format.width
    target_h = target_format.height
    target_ratio = target_w / target_h

    crop_h = input_height
    crop_w = int(input_height * target_ratio)
    if crop_w > input_width:
        crop_w = input_width
        crop_h = int(input_width / target_ratio)
    max_x = input_width - crop_w

    if not keyframes:
        x = (input_width - crop_w) // 2
        return f"crop={crop_w}:{crop_h}:{x}:0,scale={target_w}:{target_h}:flags=lanczos"

    if len(keyframes) == 1:
        x = max(0, min(keyframes[0].crop_x, max_x))
        return f"crop={crop_w}:{crop_h}:{x}:0,scale={target_w}:{target_h}:flags=lanczos"

    # Reduce keyframes: only keep ones where crop_x changes by >= 5px
    # FFmpeg crashes with deeply nested expressions, so cap at 60
    MAX_EXPR_KEYFRAMES = 60

    reduced = [keyframes[0]]
    for kf in keyframes[1:]:
        if abs(kf.crop_x - reduced[-1].crop_x) >= 5:
            reduced.append(kf)
    if reduced[-1].time != keyframes[-1].time:
        reduced.append(keyframes[-1])

    # Subsample evenly if still too many
    if len(reduced) > MAX_EXPR_KEYFRAMES:
        step = len(reduced) / (MAX_EXPR_KEYFRAMES - 1)
        subsampled = [reduced[int(i * step)] for i in range(MAX_EXPR_KEYFRAMES - 1)]
        subsampled.append(reduced[-1])
        reduced = subsampled

    logger.info(
        f"🎯 Smart Crop expression: {len(reduced)} keyframes "
        f"(reduced from {len(keyframes)})"
    )

    if len(reduced) == 1:
        x = max(0, min(reduced[0].crop_x, max_x))
        return f"crop={crop_w}:{crop_h}:{x}:0,scale={target_w}:{target_h}:flags=lanczos"

    # Build nested FFmpeg expression
    x_expr = str(max(0, min(reduced[0].crop_x, max_x)))

    for i in range(1, len(reduced)):
        kf = reduced[i]
        prev_kf = reduced[i - 1]
        x_cur = max(0, min(kf.crop_x, max_x))
        x_prev = max(0, min(prev_kf.crop_x, max_x))
        t_start = kf.time

        if x_cur == x_prev:
            continue

        t_trans_end = t_start + transition_duration
        progress = f"(t-{t_start:.3f})/{transition_duration:.3f}"
        lerp = f"{x_prev}+({x_cur}-{x_prev})*min(1\\,max(0\\,{progress}))"

        x_expr = (
            f"if(gte(t\\,{t_start:.3f})\\,"
            f"if(gte(t\\,{t_trans_end:.3f})\\,{x_cur}\\,{lerp})\\,"
            f"{x_expr})"
        )

    filter_str = f"crop={crop_w}:{crop_h}:'{x_expr}':0,scale={target_w}:{target_h}:flags=lanczos"
    return filter_str


async def _cut_clip_ffmpeg(
    input_path: str,
    output_path: str,
    start: float,
    end: float,
    job_id: str,
    clip_index: int,
    subtitle_path: Optional[str] = None,
    crop_filter: Optional[str] = None,
    sample_rate: int = 48000,
) -> dict:
    """
    Cut a single clip using FFmpeg async subprocess, optionally burning subtitles.

    Args:
        input_path: Source video file path
        output_path: Destination clip file path
        start: Start timestamp in seconds
        end: End timestamp in seconds
        job_id: Job ID for logging
        clip_index: Clip number for logging
        subtitle_path: Optional path to .ass subtitle file to burn
        crop_filter: Optional FFmpeg crop/scale filter

    Returns:
        Dict with clip info or error details
    """
    duration = end - start

    # Use input seeking (faster, less accurate but subtitles actually render)
    cmd = [
        "ffmpeg",
        "-hwaccel",
        "cuda",
        "-ss",
        str(start),
        "-i",
        input_path,
        "-t",
        str(duration),
    ]

    # Build video filter chain: crop + subtitle
    video_filters = []

    if crop_filter:
        video_filters.append(crop_filter)

    if subtitle_path and os.path.exists(subtitle_path):
        # Escape backslashes and colons for FFmpeg filter on Windows
        escaped_path = subtitle_path.replace("\\", "/").replace(":", "\\:")
        video_filters.append(f"ass='{escaped_path}'")
        logger.info(f"🔤 [Job {job_id}] Burning subtitles for clip {clip_index}")

    if video_filters:
        filter_chain = ",".join(video_filters)
        cmd.extend(["-vf", filter_chain])

    cmd.extend(
        [
            "-c:v",
            "h264_nvenc",
            "-preset",
            "p6",       # quality-focused (p1=fastest, p7=best quality)
            "-cq",
            "15",       # lower = better quality (was 19)
            "-b:v", "6000k",    # minimum bitrate floor for 1080p
            "-video_track_timescale", "90000",
            "-c:a",
            "aac",
            "-b:a",
            "192k",
            "-ac", "2",
            "-ar", str(sample_rate),
            "-movflags",
            "+faststart",
            "-y",
            output_path,
        ]
    )

    logger.info(
        f"✂️ [Job {job_id}] Cutting clip {clip_index}: "
        f"{start:.1f}s → {end:.1f}s ({duration:.1f}s)"
    )

    try:
        import time as _time
        _ffmpeg_start = _time.time()

        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()
        _ffmpeg_elapsed = _time.time() - _ffmpeg_start

        if process.returncode != 0:
            error_msg = stderr.decode()[-500:]
            logger.error(
                f"❌ [Job {job_id}] FFmpeg failed for clip {clip_index} "
                f"({_ffmpeg_elapsed:.1f}s): {error_msg}"
            )
            return {"success": False, "error": error_msg}

        file_size = os.path.getsize(output_path)
        logger.info(
            f"✅ [Job {job_id}] Clip {clip_index} saved in {_ffmpeg_elapsed:.1f}s: "
            f"{output_path} ({file_size / 1024 / 1024:.1f} MB)"
        )

        return {
            "success": True,
            "file_path": output_path,
            "file_size": file_size,
            "duration": duration,
        }

    except FileNotFoundError:
        logger.error(f"❌ [Job {job_id}] FFmpeg not found! Is it installed?")
        return {"success": False, "error": "FFmpeg not found"}
    except Exception as e:
        logger.error(f"❌ [Job {job_id}] FFmpeg error: {e}")
        return {"success": False, "error": str(e)}


async def _get_audio_duration(audio_path: str) -> float:
    """Get duration of a WAV audio file in seconds."""
    try:
        import wave
        with wave.open(audio_path, "rb") as wf:
            frames = wf.getnframes()
            rate = wf.getframerate()
            return frames / rate
    except Exception:
        return 4.0  # default fallback


async def _detect_clip_fps(clip_path: str) -> float:
    """Detect frame rate of a video clip using ffprobe."""
    cmd = [
        "ffprobe", "-v", "error",
        "-select_streams", "v:0",
        "-show_entries", "stream=r_frame_rate",
        "-of", "json",
        clip_path,
    ]
    try:
        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, _ = await process.communicate()
        if process.returncode == 0:
            data = json.loads(stdout.decode())
            r_str = data.get("streams", [{}])[0].get("r_frame_rate", "30/1")
            num, den = r_str.split("/")
            return float(num) / float(den)
    except Exception:
        pass
    return 30.0


async def _create_hook_and_concat(
    hook: dict,
    cut_clip_path: str,
    final_output_path: str,
    clip_index: int,
    job_id: str,
    target_width: int,
    target_height: int,
    clip_fps: float = 30.0,
    sample_rate: int = 48000,
) -> Optional[str]:
    """
    Create hook intro and concatenate with main clip in a single FFmpeg call.
    Replaces the old _create_hook_intro + _concat_intro_and_clip two-step pipeline,
    eliminating one FFmpeg spawn and one intermediate file read/write per clip.

    Filter graph:
      [0:v] split → [vfreeze] for blurred freeze intro, [vmain] for main content
      [vfreeze] → trim→loop→blur→drawtext→trim(hook_dur) → [intro_v]
      [1:a]/[2:a] → tts+sfx mix → [intro_a]
      concat([intro_v][intro_a][vmain][0:a]) → [outv][outa]

    Returns:
        final_output_path on success, None on failure
    """
    hook_text = hook.get("hook_text", "")
    audio_path = hook.get("audio_path", "")

    if not hook_text:
        return None

    try:
        # Determine intro duration from TTS audio (or default 4s)
        if audio_path and os.path.exists(audio_path):
            tts_duration = await _get_audio_duration(audio_path)
            hook_duration = 1.0 + tts_duration + 1.0
        else:
            hook_duration = 4.0

        # Build drawtext filter (uppercase lines, Arial Black)
        safe_text = hook_text.upper()
        max_chars_per_line = 15
        words = safe_text.split()
        lines = []
        current_line = ""
        for word in words:
            if len(current_line) + len(word) + 1 > max_chars_per_line and current_line:
                lines.append(current_line)
                current_line = word
            else:
                current_line = f"{current_line} {word}" if current_line else word
        if current_line:
            lines.append(current_line)

        font_size = int(target_height * 0.044)
        line_spacing = int(font_size * 0.15)
        _arial_black = "C\\:/Windows/Fonts/ariblk.ttf"
        total_text_h = len(lines) * font_size + (len(lines) - 1) * line_spacing
        _dt_parts = []
        for _i, _line in enumerate(lines):
            _esc = escape_ffmpeg_text(_line)
            _y = f"(h-{total_text_h})/2+{_i * (font_size + line_spacing)}"
            _dt_parts.append(
                f"drawtext=text='{_esc}'"
                f":fontfile='{_arial_black}'"
                f":fontsize={font_size}"
                f":fontcolor=white"
                f":borderw=4.5:bordercolor=black"
                f":x=(w-text_w)/2:y={_y}"
            )
        drawtext_filter = ",".join(_dt_parts)

        has_audio = bool(audio_path and os.path.exists(audio_path))
        sfx_path = os.path.join(os.getcwd(), "music", "sound-effect-1.mp3")
        has_sfx = os.path.exists(sfx_path)

        # Build inputs: [0]=cut_clip, [1]=tts or anullsrc, [2]=sfx (optional)
        cmd = ["ffmpeg", "-y", "-i", cut_clip_path]
        if has_audio:
            cmd.extend(["-i", audio_path])
        else:
            cmd.extend(["-f", "lavfi", "-i", f"anullsrc=r={sample_rate}:cl=stereo"])
        if has_sfx:
            cmd.extend(["-i", sfx_path])

        # Intro video: split [0:v], freeze first frame, blur, text, trim to hook_duration
        intro_v_filter = (
            f"[0:v]split=2[vfreeze][vmain];"
            f"[vfreeze]trim=0:0.04,loop=-1:1,setpts=N/FRAME_RATE/TB,"
            f"boxblur=20:5,{drawtext_filter},"
            f"trim=0:{hook_duration:.3f},setpts=PTS-STARTPTS[intro_v]"
        )

        # Intro audio: tts + sfx mix, trimmed to hook_duration, ensured STEREO
        if has_audio and has_sfx:
            intro_a_filter = (
                f"[1:a]adelay=1000:all=1,volume=3.0,aformat=channel_layouts=stereo,aresample={sample_rate}[tts];"
                f"[2:a]atrim=0:2,volume=0.7,aformat=channel_layouts=stereo,aresample={sample_rate}[sfx];"
                f"[tts][sfx]amix=inputs=2:duration=longest,atrim=0:{hook_duration:.3f}[intro_a]"
            )
        elif has_audio:
            intro_a_filter = (
                f"[1:a]adelay=1000:all=1,volume=3.0,aformat=channel_layouts=stereo,aresample={sample_rate},"
                f"atrim=0:{hook_duration:.3f}[intro_a]"
            )
        elif has_sfx:
            intro_a_filter = (
                f"[2:a]atrim=0:2,volume=0.7,aformat=channel_layouts=stereo,aresample={sample_rate}[intro_a]"
            )
        else:
            intro_a_filter = f"[1:a]aformat=channel_layouts=stereo,aresample={sample_rate},atrim=0:{hook_duration:.3f}[intro_a]"

        # Concat intro + main clip
        concat_filter = "[intro_v][intro_a][vmain][0:a]concat=n=2:v=1:a=1[outv][outa]"

        filter_complex = f"{intro_v_filter};{intro_a_filter};{concat_filter}"

        cmd.extend([
            "-filter_complex", filter_complex,
            "-map", "[outv]",
            "-map", "[outa]",
            "-c:v", "h264_nvenc",
            "-preset", "p6",
            "-cq", "15",
            "-b:v", "6000k",
            "-video_track_timescale", "90000",
            "-c:a", "aac",
            "-b:a", "192k",
            "-ac", "2",
            "-ar", str(sample_rate),
            "-pix_fmt", "yuv420p",
            "-r", str(clip_fps),
            "-movflags", "+faststart",
            final_output_path,
        ])

        process = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()

        if process.returncode != 0:
            logger.warning(
                f"[Job {job_id}] Hook+concat failed for clip {clip_index}: "
                f"{stderr.decode()[-300:]}"
            )
            return None

        logger.info(f"[Job {job_id}] Hook+concat complete: {final_output_path} ({hook_duration:.1f}s intro)")
        return final_output_path

    except Exception as e:
        logger.warning(f"[Job {job_id}] Hook+concat error for clip {clip_index}: {e}")
        return None


async def editing_node(
    state: VideoProcessingState, redis: aioredis.Redis
) -> Command[Literal["finalization"]]:
    """
    Phase 3B: Video editing using FFmpeg with subtitle burning + hook intros.

    Pipeline steps (58% → 80%):
    1. Create output directory for job clips
    2. Generate ASS subtitles per clip (word-level from WhisperX)
    3. Cut each clip + burn subtitles using FFmpeg
    4. Prepend hook intro (if available) to each clip
    5. Collect metadata and route to finalization

    Args:
        state: Current LangGraph state
        redis: Async Redis connection for progress tracking

    Returns:
        Command with generated clips routing to finalization
    """
    job_id = state["job_id"]

    # Abort check sebelum mulai
    if await check_aborted(redis, job_id):
        logger.info(f"⏭️ [Job {job_id}] Aborted before editing")
        return Command(update={"status": "aborted"}, goto="finalization")

    video_path = state.get("video_path", "")
    clip_candidates = state.get("clip_candidates", [])
    hooks = state.get("hooks") or []
    transcription: Optional[TranscriptionResultDetailed] = state.get(
        "transcription_result"
    )

    # Initialize progress tracker
    tracker = create_progress_tracker(redis, job_id)

    try:
        logger.info(f"🎬 [Job {job_id}] Starting Phase 3B: Video Editing + Subtitles")
        await tracker.update_progress(58, "editing", "Phase 3B: Video Editing")

        # Validate input
        if not video_path or not os.path.exists(video_path):
            raise FileNotFoundError(f"Video file not found: {video_path}")

        if not clip_candidates:
            logger.warning(f"⚠️ [Job {job_id}] No clip candidates to edit")
            return Command(
                update={
                    "clips": [],
                    "progress": 80,
                    "status": "finalizing",
                    "current_phase": "Phase 4: Finalization",
                },
                goto="finalization",
            )

        # Create output directory: clips/{job_id}/
        job_clips_dir = os.path.join(settings.CLIPS_DIR, job_id)
        os.makedirs(job_clips_dir, exist_ok=True)

        # Get transcription segments for subtitle generation
        segments = transcription.segments if transcription else []

        # Detect video resolution and get target format
        logger.info(f"🔍 [Job {job_id}] Detecting video resolution...")
        input_width, input_height = await _detect_video_resolution(video_path)
        logger.info(f"📐 [Job {job_id}] Input: {input_width}x{input_height}")

        # Get target output format from config
        try:
            target_format = get_format(settings.OUTPUT_FORMAT)
            logger.info(
                f"🎯 [Job {job_id}] Target format: {target_format.description} "
                f"({target_format.width}x{target_format.height})"
            )
        except ValueError:
            # Fallback to TikTok if invalid format
            logger.warning(
                f"⚠️ [Job {job_id}] Invalid OUTPUT_FORMAT '{settings.OUTPUT_FORMAT}', "
                "using TikTok (1080x1920)"
            )
            target_format = get_format("tiktok")

        # ── Smart Crop: Active Speaker Detection ───────────────────────────
        speaker_positions = None
        is_landscape_to_portrait = (input_width / input_height) > (
            target_format.width / target_format.height
        )

        if (
            settings.ENABLE_SMART_CROP
            and settings.CROP_MODE == "smart"
            and is_landscape_to_portrait
        ):
            try:
                logger.info(
                    f"🎯 [Job {job_id}] Smart Crop: Detecting faces..."
                )
                await tracker.update_progress(
                    52, "editing", "Detecting faces for smart crop..."
                )

                from app.utils.speaker_detector import (
                    SpeakerDetector,
                    TRANSITION_DURATION,
                )

                crop_target_ratio = target_format.width / target_format.height
                crop_w_in_source = int(input_height * crop_target_ratio)

                detection_clips = [
                    {"start_time": c["start"], "end_time": c["end"]}
                    for c in clip_candidates
                ]

                detector = SpeakerDetector(device="cuda")
                detector.load()

                loop = asyncio.get_event_loop()
                speaker_positions = await loop.run_in_executor(
                    None,
                    lambda: detector.detect_active_speakers(
                        video_path=video_path,
                        clip_candidates=detection_clips,
                        frame_width=input_width,
                        frame_height=input_height,
                        target_width=crop_w_in_source,
                        target_height=input_height,
                        segments=transcription.segments if transcription else None,
                    )
                )

                detector.unload()

                logger.info(
                    f"✅ [Job {job_id}] Smart Crop: Detected positions for "
                    f"{len(speaker_positions)} clips"
                )

            except Exception as e:
                logger.warning(
                    f"⚠️ [Job {job_id}] Smart Crop failed, falling back to center crop: {e}"
                )
                speaker_positions = None

        # Calculate default (center) crop filter
        default_crop_filter = _calculate_crop_filter(
            input_width, input_height, target_format
        )

        # Detect source fps and sample rate once (used for hook intro matching)
        source_fps = await _detect_clip_fps(video_path)
        source_sample_rate = await _detect_audio_sample_rate(video_path)
        logger.info(f"[Job {job_id}] Source matching: {source_fps:.2f} FPS / {source_sample_rate} Hz")

        # Cut each clip
        generated_clips = []
        total_clips = len(clip_candidates)
        errors = []

        # TTS engine shared across clips — lock inside TTSEngine serializes GPU inference
        # while allowing TTS(clip N) to overlap with FFmpeg(clip N-1)
        language = transcription.language if transcription else "en"
        tts = TTSEngine() if hooks else None

        # Parallel clip processing: semaphore limits concurrent GPU workloads
        # (TTS serialized internally by TTSEngine._lock, FFmpeg can run in parallel)
        semaphore = asyncio.Semaphore(5)
        completed_count = 0
        progress_lock = asyncio.Lock()

        async def _process_single_clip(i, candidate, hooks_list):
            nonlocal completed_count
            # Abort check per clip
            if await check_aborted(redis, job_id):
                return None, f"Clip {i + 1} aborted"
            clip_num = i + 1
            clip_start = candidate["start"]
            clip_end = candidate["end"]

            # Step A: Generate ASS subtitle for this clip
            subtitle_path = None
            if segments:
                ass_filename = f"clip_{clip_num}.ass"
                ass_path = os.path.join(job_clips_dir, ass_filename)

                try:
                    subtitle_path = generate_ass_subtitle(
                        segments=segments,
                        clip_start=clip_start,
                        clip_end=clip_end,
                        output_path=ass_path,
                        job_id=job_id,
                        video_width=target_format.width,
                        video_height=target_format.height,
                        font_size=target_format.subtitle_size,
                        margin_bottom=target_format.subtitle_margin_bottom,
                    )
                    if not subtitle_path:
                        logger.warning(f"⚠️ [Job {job_id}] Clip {clip_num}: subtitle generation returned None")
                except Exception as e:
                    logger.warning(f"⚠️ [Job {job_id}] Clip {clip_num}: subtitle generation failed: {e}")
                    subtitle_path = None

            # Step B: Determine crop filter (smart keyframe or center)
            clip_crop_filter = default_crop_filter
            if speaker_positions and i < len(speaker_positions):
                pos = speaker_positions[i]
                if not pos.is_fallback and pos.keyframes:
                    clip_crop_filter = _build_keyframe_crop_filter(
                        keyframes=pos.keyframes,
                        input_width=input_width,
                        input_height=input_height,
                        target_format=target_format,
                        transition_duration=TRANSITION_DURATION,
                    )
                    logger.info(
                        f"🎯 [Job {job_id}] Clip {clip_num}: "
                        f"dynamic crop with {len(pos.keyframes)} keyframes"
                    )

            # Step C: Cut clip + burn subtitles with FFmpeg (limited by semaphore)
            # Step C: Generate descriptive filename (e.g. "Viral_Moment_1.mp4") with fallback
            clip_title = candidate.get("title", "")
            safe_title = sanitize_filename(clip_title)
            
            if safe_title:
                clip_filename = f"{safe_title}_{clip_num}.mp4"
            else:
                clip_filename = f"clip_{clip_num}.mp4"
                
                
            output_path = os.path.join(job_clips_dir, clip_filename)

            async with semaphore:
                # Step C: Synthesize TTS hook audio (serialized by TTSEngine._lock,
                # but overlaps with FFmpeg from previously completed clips)
                hook = next((h for h in hooks_list if h.get("clip_index") == i), None)
                if hook and hook.get("hook_text") and tts:
                    tts_audio_path = os.path.join(job_clips_dir, f"hook_{clip_num}.wav")
                    loop = asyncio.get_event_loop()
                    result_path = await loop.run_in_executor(
                        None, tts.synthesize, hook["hook_text"], language, tts_audio_path
                    )
                    if result_path:
                        hook = {**hook, "audio_path": result_path}

                # Step D: Cut clip + burn subtitles
                result = await _cut_clip_ffmpeg(
                    input_path=video_path,
                    output_path=output_path,
                    start=clip_start,
                    end=clip_end,
                    job_id=job_id,
                    clip_index=clip_num,
                    subtitle_path=subtitle_path,
                    crop_filter=clip_crop_filter,
                    sample_rate=source_sample_rate,
                )

                # Step E: Prepend hook intro + concat in single FFmpeg call
                if result["success"] and hook and hook.get("hook_text"):
                    final_path = os.path.join(job_clips_dir, f"final_{clip_filename}")
                    hook_result = await _create_hook_and_concat(
                        hook=hook,
                        cut_clip_path=output_path,
                        final_output_path=final_path,
                        clip_index=clip_num,
                        job_id=job_id,
                        target_width=target_format.width,
                        target_height=target_format.height,
                        clip_fps=source_fps,
                        sample_rate=source_sample_rate,
                    )
                    if hook_result:
                        os.remove(output_path)
                        os.rename(final_path, output_path)
                        logger.info(f"[Job {job_id}] Clip {clip_num}: hook intro prepended")
                    elif os.path.exists(final_path):
                        os.remove(final_path)

            # Update progress after each clip completes
            async with progress_lock:
                completed_count += 1
                clip_progress = 58 + int((completed_count / total_clips) * 22)
                await tracker.update_progress(
                    clip_progress,
                    phase=f"Completed clip {completed_count}/{total_clips}: {candidate.get('title', '')}",
                )

            if result["success"]:
                # hook is already resolved from the TTS synthesis step above
                hook_text = hook.get("hook_text", "") if hook else ""
                
                # Get the full transcript text for this clip timeframe
                transcript_text = ""
                if segments:
                    clip_segs = [s.text.strip() for s in segments if s.end >= clip_start and s.start <= clip_end]
                    transcript_text = " ".join(clip_segs)

                return {
                    "clip_id": f"{job_id}_clip_{clip_num}",
                    "clip_number": clip_num,
                    "start": clip_start,
                    "end": clip_end,
                    "duration": result["duration"],
                    "title": candidate.get("title", f"Clip {clip_num}"),
                    "reasoning": candidate.get("reasoning", ""),
                    "viral_score": candidate.get("viral_score", 0),
                    "suggested_caption": candidate.get("suggested_caption", ""),
                    "hook_text": hook_text,
                    "transcript_text": transcript_text,
                    "tags": candidate.get("tags", []),
                    "file_path": output_path,
                    "file_size": result["file_size"],
                    "has_subtitles": subtitle_path is not None,
                    "subtitle_path": subtitle_path,
                    "status": "ready",
                }, None
            else:
                error_msg = f"Clip {clip_num} failed: {result['error']}"
                logger.error(f"❌ [Job {job_id}] {error_msg}")
                return None, error_msg

        # Launch all clips in parallel — TTS(clip N) overlaps with FFmpeg(clip N-1)
        logger.info(f"🚀 [Job {job_id}] Processing {total_clips} clips in parallel (max 5 concurrent, TTS overlaps FFmpeg)")
        tasks = [_process_single_clip(i, c, hooks) for i, c in enumerate(clip_candidates)]
        results = await asyncio.gather(*tasks)

        # Free TTS GPU memory after all clips are done
        if tts:
            tts.unload()

        # Collect results
        for clip_metadata, error in results:
            if clip_metadata:
                generated_clips.append(clip_metadata)
            if error:
                errors.append(error)

        # Sort by clip_number to maintain order
        generated_clips.sort(key=lambda x: x["clip_number"])

        await tracker.update_progress(80, "finalizing", "Phase 3 complete")

        logger.info(
            f"✅ [Job {job_id}] Phase 3 COMPLETE! "
            f"Cut {len(generated_clips)}/{total_clips} clips with subtitles"
        )

        # Build state update
        update = {
            "clips": generated_clips,
            "progress": 80,
            "status": "finalizing",
            "current_phase": "Phase 4: Finalization",
        }

        if errors:
            update["errors"] = errors

        return Command(update=update, goto="finalization")

    except AbortError:
        return Command(update={"status": "aborted"}, goto="finalization")

    except Exception as e:
        error_msg = f"Video editing failed: {str(e)}"
        logger.error(f"❌ [Job {job_id}] {error_msg}", exc_info=True)
        await tracker.set_error(error_msg)

        return Command(
            update={
                "progress": 58,
                "status": "failed",
                "current_phase": "Failed",
                "errors": [error_msg],
            },
            goto="finalization",
        )

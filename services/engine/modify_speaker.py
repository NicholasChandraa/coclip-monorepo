import os

filepath = "app/utils/speaker_detector.py"
with open(filepath, "r", encoding="utf-8") as f:
    text = f.read()

# 1. Add lock in __init__
old_init = """        self.face_detector = None
        self._loaded = False
        self._stats = {}"""
new_init = """        self.face_detector = None
        self._loaded = False
        self._stats = {}
        self._detector_lock = threading.Lock()"""
text = text.replace(old_init, new_init)

# 2. Add locks in detect methods
old_raw = """                bboxes = self.face_detector.detect_faces(frame, conf_th=conf_threshold)"""
new_raw = """                with self._detector_lock:
                    bboxes = self.face_detector.detect_faces(frame, conf_th=conf_threshold)"""
text = text.replace(old_raw, new_raw)

old_scaled = """                bboxes = self.face_detector.detect_faces(small_frame, conf_th=conf_threshold)"""
new_scaled = """                with self._detector_lock:
                    bboxes = self.face_detector.detect_faces(small_frame, conf_th=conf_threshold)"""
text = text.replace(old_scaled, new_scaled)

# 3. Refactor detect_active_speakers
old_loop_start = """        results = []
        center_x = (frame_width - target_width) // 2
        has_diarization = segments is not None and len(segments) > 0

        total_clips = len(clip_candidates)
        logger.info(
            f"Starting face detection: {total_clips} clips, "
            f"video={frame_width}x{frame_height}, "
            f"detection_width={min(frame_width, DETECTION_MAX_WIDTH)}px, "
            f"diarization={'enabled' if has_diarization else 'disabled (largest face)'}"
        )

        for clip_idx, clip in enumerate(clip_candidates):"""

new_loop_start = """        results = [None] * len(clip_candidates)
        center_x = (frame_width - target_width) // 2
        has_diarization = segments is not None and len(segments) > 0

        total_clips = len(clip_candidates)
        logger.info(
            f"Starting face detection: {total_clips} clips, "
            f"video={frame_width}x{frame_height}, "
            f"detection_width={min(frame_width, DETECTION_MAX_WIDTH)}px, "
            f"diarization={'enabled' if has_diarization else 'disabled (largest face)'}"
        )

        def process_clip(clip_idx, clip):
            local_stats = {}"""

text = text.replace(old_loop_start, new_loop_start)

# Shift indentation of the loop body
# The loop body is between "            start_time = clip.get("start_time", 0)"
# and "        return results"
old_loop_body_start = text.find('            start_time = clip.get("start_time", 0)')
old_loop_body_end = text.find('        return results', old_loop_body_start)

loop_body = text[old_loop_body_start:old_loop_body_end]

# Modify the loop body
new_loop_body = loop_body.replace('self._stats = {}  # reset per clip', '')
new_loop_body = new_loop_body.replace('self._stats["s3fd_ok"]', 'local_stats["s3fd_ok"]')
new_loop_body = new_loop_body.replace('self._stats["s3fd_fail"]', 'local_stats["s3fd_fail"]')
new_loop_body = new_loop_body.replace('self._stats.get("s3fd_fail", 0)', 'local_stats.get("s3fd_fail", 0)')

# Handle early returns (continue statements)
early_return_old = """            if duration <= 0:
                results.append(
                    self._center_position(clip_idx, frame_width, target_width)
                )
                continue"""
early_return_new = """            if duration <= 0:
                return clip_idx, self._center_position(clip_idx, frame_width, target_width)"""
new_loop_body = new_loop_body.replace(early_return_old, early_return_new)

# Handle final return
final_return_old = """            results.append(
                SpeakerPosition(
                    clip_index=clip_idx,
                    crop_x=primary_crop_x,
                    active_speaker_bbox=best_face,
                    confidence=best_face.confidence if best_face else 0.0,
                    is_fallback=len(smoothed_keyframes) == 0,
                    keyframes=smoothed_keyframes,
                )
            )"""
final_return_new = """            pos = SpeakerPosition(
                clip_index=clip_idx,
                crop_x=primary_crop_x,
                active_speaker_bbox=best_face,
                confidence=best_face.confidence if best_face else 0.0,
                is_fallback=len(smoothed_keyframes) == 0,
                keyframes=smoothed_keyframes,
            )
            return clip_idx, pos"""
new_loop_body = new_loop_body.replace(final_return_old, final_return_new)

text = text[:old_loop_body_start] + new_loop_body + """
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(process_clip, i, c) for i, c in enumerate(clip_candidates)]
            for future in as_completed(futures):
                idx, pos = future.result()
                results[idx] = pos

""" + text[old_loop_body_end:]

with open(filepath, "w", encoding="utf-8") as f:
    f.write(text)

print("Modification complete!")

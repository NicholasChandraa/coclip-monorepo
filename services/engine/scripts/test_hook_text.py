"""
Test hook intro text rendering with FFmpeg drawtext.
Verifies that multiline text wraps correctly without box characters.

Run:
    uv run python -X utf8 scripts/test_hook_text.py
"""

import subprocess
import sys
import os

# Same escape function as editing_node.py (must stay in sync)
def escape_ffmpeg_text(text: str) -> str:
    if not text:
        return ""
    text = text.replace('\\', '\\\\')
    text = text.replace(':', '\\:')
    text = text.replace("'", "\\'")
    text = text.replace('%', '\\%')
    # Actual newline → FFmpeg drawtext \n escape (AFTER backslash escaping)
    text = text.replace('\n', '\\n')
    return text


def wrap_text(hook_text: str, max_chars_per_line: int = 15) -> str:
    safe_text = hook_text.upper().replace("'", "\u2019").replace(":", "\\:")
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
    return "\n".join(lines)


def build_multiline_drawtext(lines: list, font_file: str, font_size: int, line_spacing: int = 10) -> str:
    """
    Build a chained drawtext filter for multi-line text.
    Uses one drawtext per line with manually calculated y positions.
    Avoids the box-glyph issue when FFmpeg renders LF characters.
    """
    filters = []
    total_height = len(lines) * font_size + (len(lines) - 1) * line_spacing
    start_y = f"(h-{total_height})/2"

    for i, line in enumerate(lines):
        escaped = escape_ffmpeg_text(line)
        y = f"({start_y})+{i * (font_size + line_spacing)}"
        filters.append(
            f"drawtext=text='{escaped}'"
            f":fontfile='{font_file}'"
            f":fontsize={font_size}"
            f":fontcolor=white"
            f":borderw=4.5:bordercolor=black"
            f":x=(w-text_w)/2:y={y}"
        )
    return ",".join(filters)


def test_drawtext(hook_text: str, output_path: str):
    """Generate a test image with hook text using per-line drawtext."""
    wrapped = wrap_text(hook_text)
    lines = wrapped.split("\n")

    font_file = "C\\:/Windows/Fonts/ariblk.ttf"
    target_width, target_height = 1080, 1920
    font_size = int(target_height * 0.044)  # ~84px

    vf = build_multiline_drawtext(lines, font_file, font_size)

    cmd = [
        "ffmpeg", "-y",
        "-f", "lavfi", "-i", f"color=c=0x1a1a2e:s={target_width}x{target_height}:d=1",
        "-vf", vf,
        "-frames:v", "1",
        output_path,
    ]

    print(f"\n{'='*60}")
    print(f"Lines: {lines}")
    print(f"Output: {output_path}")
    print(f"{'='*60}")

    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode == 0:
        size = os.path.getsize(output_path)
        print(f"✅ SUCCESS — {size} bytes")
    else:
        print(f"❌ FAILED")
        stderr_lines = result.stderr.strip().splitlines()
        relevant = [l for l in stderr_lines if "fontconfig" not in l.lower()][-10:]
        print('\n'.join(relevant))


if __name__ == "__main__":
    os.makedirs("tests/hook_clip_output", exist_ok=True)

    hook_text = "Gila! Prilly nekat baca mantra buat dirasuki hantu, coba denger apa yang dia rasain."

    test_drawtext(hook_text, "tests/hook_clip_output/hook_text_result.png")
    print("\nDone. Open tests/hook_clip_output/hook_text_result.png to verify.")

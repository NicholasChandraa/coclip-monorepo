import os
import sys
import asyncio
import logging
import subprocess

# Add the app directory to sys.path to allow imports
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..")))

from app.core.config import settings
from app.utils.logging import logger

def ensure_node():
    """Diagnostic for Node.js visibility."""
    node_paths = [
        "C:\\nvm4w\\nodejs",
        "C:\\Program Files\\nodejs",
        os.path.expandvars("%AppData%\\npm"),
    ]
    path_env = os.environ.get("PATH", "")
    found = False
    for p in node_paths:
        if os.path.exists(p):
            if p not in path_env:
                os.environ["PATH"] = p + os.pathsep + os.environ["PATH"]
                path_env = os.environ["PATH"]
            print(f"✅ Found Node.js path: {p}")
            found = True
    
    try:
        node_v = subprocess.check_output(["node", "-v"], text=True).strip()
        print(f"✅ Node.js Version: {node_v}")
        
        # Test direct execution
        node_test = subprocess.check_output(["node", "-e", "console.log('JS_WORKS')"], text=True).strip()
        print(f"✅ Node.js Execution Test: {node_test}")
    except Exception as e:
        print(f"❌ Node.js NOT working: {e}")
    
    import shutil
    node_path = shutil.which("node")
    print(f"✅ shutil.which('node'): {node_path}")
    return found

async def run_diagnostic(url: str):
    import yt_dlp
    
    print(f"\n--- Starting Diagnostic for: {url} ---")
    print(f"yt-dlp version: {yt_dlp.version.__version__}")
    
    ensure_node()
    
    # Test 1: Minimal with flat extraction
    print("\n[Test 1] Metadata only (process=False)...")
    ydl_opts_1 = {
        "quiet": True,
        "no_warnings": True,
        "nocheckcertificate": True,
        "ignoreconfig": True,
        "cookiefile": settings.YOUTUBE_COOKIES_PATH if os.path.exists(settings.YOUTUBE_COOKIES_PATH or "") else None,
    }
    
    try:
        with yt_dlp.YoutubeDL(ydl_opts_1) as ydl:
            info = ydl.extract_info(url, download=False, process=False)
            print(f"✅ Metadata retrieved: {info.get('title')}")
            print(f"   Formats found (flat): {len(info.get('formats', []))}")
    except Exception as e:
        print(f"❌ Test 1 Failed: {e}")

    # Test 2: Full Extraction with Mobile Clients (Android)
    print("\n[Test 2] Full format resolution (Android-first)...")
    ydl_opts_2 = {
        "cookiefile": settings.YOUTUBE_COOKIES_PATH if os.path.exists(settings.YOUTUBE_COOKIES_PATH or "") else None,
        "user_agent": "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
        "extractor_args": {
            "youtube": {
                "player_client": ["android", "ios"],
                "include_dash_manifest": True,
            }
        },
        "ignoreconfig": True,
        "verbose": True,
    }
    
    try:
        with yt_dlp.YoutubeDL(ydl_opts_2) as ydl:
            info = ydl.extract_info(url, download=False, process=True)
            formats = info.get("formats", [])
            print(f"✅ Formats resolved: {len(formats)}")
            if formats:
                best = formats[-1]
                print(f"   Best format: {best.get('format_id')} ({best.get('resolution')})")
    except Exception as e:
        print(f"❌ Test 2 Failed: {e}")

    # Test 3: Full Extraction with Web Client
    print("\n[Test 3] Full format resolution (Web-first)...")
    ydl_opts_3 = {
        "cookiefile": settings.YOUTUBE_COOKIES_PATH if os.path.exists(settings.YOUTUBE_COOKIES_PATH or "") else None,
        "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "extractor_args": {
            "youtube": {
                "player_client": ["web"],
            }
        },
        "ignoreconfig": True,
        "verbose": True,
    }
    
    # Test 4: Android WITHOUT cookies (often bypasses n-challenge)
    print("\n[Test 4] Android client WITHOUT cookies...")
    ydl_opts_4 = {
        "user_agent": "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
        "extractor_args": {
            "youtube": {
                "player_client": ["android"],
                "include_dash_manifest": True,
            }
        },
        "ignoreconfig": True,
        "js_runtimes": {"node": {}},
        "verbose": True,
    }
    
    try:
        with yt_dlp.YoutubeDL(ydl_opts_4) as ydl:
            info = ydl.extract_info(url, download=False, process=True)
            formats = info.get("formats", [])
            print(f"✅ Formats resolved: {len(formats)}")
    except Exception as e:
        print(f"❌ Test 4 Failed: {e}")

if __name__ == "__main__":
    url = sys.argv[1] if len(sys.argv) > 1 else "https://www.youtube.com/watch?v=WTrkPGuwSnM"
    asyncio.run(run_diagnostic(url))

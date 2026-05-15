import asyncio
import os
import sys
from dotenv import load_dotenv

# Add the project root to sys.path
sys.path.append(os.getcwd())

load_dotenv(encoding="utf-8-sig")

from app.utils.downloader import get_video_info

async def main():
    url = "https://www.youtube.com/watch?v=WTrkPGuwSnM"  # The blocked video
    if len(sys.argv) > 1:
        url = sys.argv[1]
    
    print(f"Testing video info for: {url}")
    print(f"YOUTUBE_COOKIES_PATH: {os.getenv('YOUTUBE_COOKIES_PATH')}")
    print(f"YOUTUBE_COOKIES_BROWSER: {os.getenv('YOUTUBE_COOKIES_BROWSER')}")
    
    info = await get_video_info(url)
    if info:
        print("Successfully fetched video info!")
        print(f"Title: {info['title']}")
    else:
        print("Failed to fetch video info (Expected if no cookies provided).")

if __name__ == "__main__":
    asyncio.run(main())

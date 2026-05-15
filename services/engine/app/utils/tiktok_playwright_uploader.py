"""TikTok uploader using browser automation (Playwright) as an alternative to the official API."""

import json
import os
import logging

from playwright.async_api import async_playwright

logger = logging.getLogger(__name__)

_CAPTION_LIMIT = 2200
_CHROME_PATH = r"C:\Program Files\Google\Chrome\Application\chrome.exe"
_COOKIES_FILENAME = "cookies.json"  # Playwright storage_state format

_STEALTH_ARGS = [
    "--disable-blink-features=AutomationControlled",
    "--disable-infobars",
    "--no-first-run",
    "--no-default-browser-check",
]

_STEALTH_SCRIPT = """
Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3] });
window.chrome = { runtime: {} };
"""

# Cookie-Editor sameSite values → Playwright values
_SAMESITE_MAP = {
    "no_restriction": "None",
    "lax": "Lax",
    "strict": "Strict",
    "unspecified": "Lax",
}


def _cookies_file(user_data_dir: str) -> str:
    return os.path.join(user_data_dir, _COOKIES_FILENAME)


def is_tiktok_session_valid(user_data_dir: str) -> bool:
    """Return True if a valid, non-expired TikTok session exists."""
    import time

    cookies_path = _cookies_file(user_data_dir)
    if os.path.exists(cookies_path):
        try:
            with open(cookies_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            cookies = data.get("cookies", [])
            now = time.time()
            for c in cookies:
                if c.get("name") == "sessionid":
                    expires = c.get("expires", -1)
                    # expires=-1 means session cookie (no explicit expiry) — treat as valid
                    if expires == -1 or expires > now:
                        return True
                    return False  # sessionid exists but expired
            # No sessionid found → not logged in
            return False
        except Exception:
            return False

    # Browser-login persistent profile (no cookies.json)
    return os.path.exists(os.path.join(user_data_dir, "Default", "Cookies"))


def import_cookies_from_json(user_data_dir: str, raw_json: str) -> int:
    """
    Convert Cookie-Editor JSON export to Playwright storage_state and save.
    Returns the number of TikTok cookies saved.
    """
    cookies_data = json.loads(raw_json)

    # Cookie-Editor exports a plain list; handle both list and storage_state dict
    if isinstance(cookies_data, dict) and "cookies" in cookies_data:
        raw_cookies = cookies_data["cookies"]
    elif isinstance(cookies_data, list):
        raw_cookies = cookies_data
    else:
        raise ValueError("Unrecognised cookie format — expected a JSON array or storage_state object")

    playwright_cookies = []
    for c in raw_cookies:
        domain = c.get("domain", "")
        if "tiktok.com" not in domain:
            continue
        same_site_raw = str(c.get("sameSite", "unspecified")).lower()
        playwright_cookies.append({
            "name": c["name"],
            "value": c["value"],
            "domain": domain,
            "path": c.get("path", "/"),
            "expires": int(c.get("expirationDate", c.get("expires", -1))),
            "httpOnly": bool(c.get("httpOnly", False)),
            "secure": bool(c.get("secure", False)),
            "sameSite": _SAMESITE_MAP.get(same_site_raw, "Lax"),
        })

    if not playwright_cookies:
        raise ValueError("No tiktok.com cookies found in the pasted JSON")

    os.makedirs(user_data_dir, exist_ok=True)
    storage_state = {"cookies": playwright_cookies, "origins": []}
    with open(_cookies_file(user_data_dir), "w", encoding="utf-8") as f:
        json.dump(storage_state, f, indent=2)

    logger.info(f"TikTok: saved {len(playwright_cookies)} cookies to {_cookies_file(user_data_dir)}")
    return len(playwright_cookies)


async def setup_tiktok_session(user_data_dir: str) -> None:
    """
    Open a headed Chrome window for manual TikTok login.
    Blocks until login succeeds (URL leaves /login) or 3 minutes pass.
    Session is persisted in user_data_dir as a Chromium profile.
    """
    os.makedirs(user_data_dir, exist_ok=True)
    async with async_playwright() as p:
        context = await p.chromium.launch_persistent_context(
            user_data_dir=user_data_dir,
            executable_path=_CHROME_PATH,
            headless=False,
            args=_STEALTH_ARGS,
            ignore_default_args=["--enable-automation"],
            viewport={"width": 1280, "height": 800},
        )
        await context.add_init_script(_STEALTH_SCRIPT)
        page = await context.new_page()
        await page.goto("https://www.tiktok.com/login", wait_until="domcontentloaded")
        logger.info("TikTok setup: browser opened — waiting for manual login…")
        try:
            await page.wait_for_function(
                "() => !window.location.href.includes('/login')",
                timeout=180_000,
            )
            logger.info("TikTok setup: login detected, saving session…")
        except Exception:
            logger.warning("TikTok setup: timed out — saving whatever session exists")
        finally:
            await context.close()
    logger.info(f"TikTok session saved to: {user_data_dir}")


async def upload_to_tiktok_playwright(
    clip_path: str,
    title: str,
    user_data_dir: str,
) -> dict:
    """Upload a video to TikTok via browser automation."""
    if not os.path.exists(clip_path):
        raise FileNotFoundError(f"Clip file not found: {clip_path}")

    if not is_tiktok_session_valid(user_data_dir):
        raise Exception(
            "TikTok session not found. Please import cookies or run /social/tiktok/setup first."
        )

    cookies_path = _cookies_file(user_data_dir)
    use_cookies_file = os.path.exists(cookies_path)

    async with async_playwright() as p:
        if use_cookies_file:
            # Imported cookies — use a fresh browser instance with storage_state
            browser = await p.chromium.launch(
                executable_path=_CHROME_PATH,
                headless=False,
                args=_STEALTH_ARGS,
                ignore_default_args=["--enable-automation"],
            )
            context = await browser.new_context(
                storage_state=cookies_path,
                viewport={"width": 1280, "height": 800},
            )
        else:
            # Browser-login persistent profile
            browser = None
            context = await p.chromium.launch_persistent_context(
                user_data_dir=user_data_dir,
                executable_path=_CHROME_PATH,
                headless=False,
                args=_STEALTH_ARGS,
                ignore_default_args=["--enable-automation"],
                viewport={"width": 1280, "height": 800},
            )

        await context.add_init_script(_STEALTH_SCRIPT)

        try:
            page = await context.new_page()

            logger.info("TikTok upload: navigating to TikTok Studio upload page…")
            await page.goto(
                "https://www.tiktok.com/tiktokstudio/upload?lang=en",
                wait_until="load",
                timeout=30_000,
            )

            if "/login" in page.url or "tiktokstudio/upload" not in page.url:
                if "/login" in page.url:
                    raise Exception("TikTok session expired. Please re-import cookies or re-run setup.")
                logger.warning(f"TikTok upload: unexpected URL after navigation: {page.url}")

            await page.wait_for_load_state("domcontentloaded")

            # Try direct page input first, fallback to iframe
            logger.info(f"TikTok upload: setting input file → {clip_path}")
            frame = None
            try:
                await page.locator('input[type="file"]').first.set_input_files(
                    clip_path, timeout=10_000
                )
                logger.info("TikTok upload: file set via direct page input")
            except Exception:
                logger.info("TikTok upload: trying iframe input…")
                frame = page.frame_locator("iframe").first
                await frame.locator('input[type="file"]').first.set_input_files(
                    clip_path, timeout=15_000
                )
                logger.info("TikTok upload: file set via iframe input")

            # Dismiss "New editing features" tutorial popup if present
            try:
                got_it = page.locator('button:has-text("Got it")')
                await got_it.wait_for(state="visible", timeout=5_000)
                await got_it.click()
                logger.info("TikTok upload: dismissed tutorial popup")
            except Exception:
                pass

            # Fill caption WHILE video is uploading in background
            logger.info("TikTok upload: filling caption while video uploads…")
            caption_text = title[:_CAPTION_LIMIT]
            caption_locator = page.locator('.public-DraftEditor-content').first
            await caption_locator.wait_for(state="visible", timeout=30_000)
            await caption_locator.click()
            await page.keyboard.press("Control+a")
            await page.keyboard.press("Backspace")
            for chunk in caption_text.split("\n"):
                await page.keyboard.type(chunk, delay=20)
                await page.keyboard.press("Enter")
            logger.info(f"TikTok upload: caption typed → {caption_text[:80]}")

            # Now wait for Post button to be enabled (upload complete)
            logger.info("TikTok upload: waiting for Post button to be enabled…")
            await page.wait_for_function(
                """() => {
                    const btn = document.querySelector('button[data-e2e="post_video_button"]');
                    if (!btn) return false;
                    return btn.getAttribute('aria-disabled') === 'false'
                        && btn.getAttribute('data-disabled') === 'false';
                }""",
                timeout=180_000,
            )
            logger.info("TikTok upload: Post button enabled — video ready")

            # Click Post
            logger.info("TikTok upload: clicking Post…")
            post_btn = page.locator('button[data-e2e="post_video_button"]').first
            await post_btn.click(timeout=10_000)

            # Handle "Continue to post?" copyright check confirmation dialog
            try:
                post_now_btn = page.locator('button.TUXButton--primary:has-text("Post now")')
                await post_now_btn.wait_for(state="visible", timeout=8_000)
                await post_now_btn.click()
                logger.info("TikTok upload: confirmed copyright popup → Post now")
            except Exception:
                pass

            # Wait for redirect away from upload page
            logger.info("TikTok upload: waiting for post confirmation…")
            await page.wait_for_url(
                lambda url: "/upload" not in url,
                timeout=60_000,
            )

            logger.info("TikTok Playwright upload complete!")
            return {"status": "success", "url": "https://www.tiktok.com"}

        finally:
            await context.close()
            if browser:
                await browser.close()

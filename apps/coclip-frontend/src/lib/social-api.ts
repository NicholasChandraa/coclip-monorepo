// --- Types ---

export interface SocialAccount {
  id: string;
  platform: string;
  platform_user_id: string;
  platform_username: string;
  connected_at: string;
}

export interface UploadStatus {
  upload_id: string;
  status: "uploading" | "processing" | "completed" | "failed";
  platform_video_id?: string;
  platform_url?: string;
  error?: string;
}

export interface UploadRequest {
  clip_id: string;
  platform: string;
  title: string;
  description?: string;
  tags?: string[];
  privacy?: "public" | "unlisted" | "private";
  scheduled_time?: string; // ISO 8601 string
}

type FetchFn = (path: string, options?: RequestInit) => Promise<Response>;

// --- Auth-service calls ---

/** Returns all connected social accounts for the current user. */
export async function getSocialAccounts(authFetch: FetchFn): Promise<SocialAccount[]> {
  const res = await authFetch("/social/accounts");
  if (!res.ok) throw new Error("Failed to fetch connected accounts");
  return res.json();
}

/** Returns the Google OAuth redirect URL — caller should redirect window.location.href to it. */
export async function getYouTubeOAuthUrl(authFetch: FetchFn): Promise<string> {
  const res = await authFetch("/social/auth/youtube/start");
  if (!res.ok) throw new Error("Failed to start YouTube OAuth");
  const data = await res.json();
  return data.url as string;
}

/** Disconnects a social account. */
export async function disconnectAccount(authFetch: FetchFn, platform: string): Promise<void> {
  const res = await authFetch(`/social/accounts/${platform}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`Failed to disconnect ${platform}`);
}

// --- Engine calls ---

/** Starts a clip upload. Returns the upload_id for polling. */
export async function startUpload(
  engineFetch: FetchFn,
  req: UploadRequest,
): Promise<{ upload_id: string }> {
  const res = await engineFetch("/social/upload", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { detail?: unknown };
    throw new Error(typeof err.detail === "string" ? err.detail : "Upload failed");
  }
  return res.json();
}

/** Polls upload status by upload_id. */
export async function getUploadStatus(
  engineFetch: FetchFn,
  uploadId: string,
): Promise<UploadStatus> {
  const res = await engineFetch(`/social/upload/${uploadId}`);
  if (!res.ok) throw new Error("Failed to get upload status");
  return res.json();
}

/** Returns whether a TikTok Playwright session exists on the engine server. */
export async function getTikTokConnected(engineFetch: FetchFn): Promise<boolean> {
  const res = await engineFetch("/social/tiktok/connected");
  if (!res.ok) return false;
  const data = await res.json();
  return data.connected as boolean;
}

/**
 * Triggers TikTok browser-login setup on the engine server.
 * The server opens a headed Chromium window — the user must log in within 3 minutes.
 * This request blocks until login completes (or times out).
 */
export async function startTikTokSetup(engineFetch: FetchFn, cancelSignal?: AbortSignal): Promise<void> {
  const timeout = AbortSignal.timeout(190_000);
  const signal = cancelSignal
    ? AbortSignal.any([timeout, cancelSignal])
    : timeout;
  const res = await engineFetch("/social/tiktok/setup", { method: "POST", signal });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { detail?: string };
    throw new Error(err.detail ?? "TikTok setup failed");
  }
}

/** Import TikTok cookies from a Cookie-Editor JSON export string. */
export async function importTikTokCookies(engineFetch: FetchFn, cookiesJson: string): Promise<number> {
  const res = await engineFetch("/social/tiktok/import-cookies", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ cookies_json: cookiesJson }),
  });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { detail?: string };
    throw new Error(err.detail ?? "Failed to import cookies");
  }
  const data = await res.json();
  return data.cookies_saved as number;
}

/** Delete the saved TikTok session (allows switching accounts). */
export async function disconnectTikTok(engineFetch: FetchFn): Promise<void> {
  const res = await engineFetch("/social/tiktok/session", { method: "DELETE" });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { detail?: string };
    throw new Error(err.detail ?? "Failed to disconnect TikTok");
  }
}

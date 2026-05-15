export const AUTH_BASE =
  process.env.NEXT_PUBLIC_AUTH_URL ?? "http://localhost:8001/api/v1";

export const ENGINE_BASE =
  process.env.NEXT_PUBLIC_ENGINE_URL ?? "http://localhost:8000/api/v1";

// ---- Clip URL builders ----

/** URL for streaming/previewing a clip (used by video elements).
 *  Appends ?token= so <video src> can authenticate without headers/cookies. */
export function getClipUrl(jobId: string, clipNumber: number, token?: string | null): string {
  const base = `${ENGINE_BASE}/transcribe/clips/${jobId}/${clipNumber}`;
  return token ? `${base}?token=${token}` : base;
}

/** URL for downloading a clip. */
export function getClipDownloadUrl(jobId: string, clipNumber: number, token?: string | null): string {
  const base = `${ENGINE_BASE}/transcribe/clips/${jobId}/${clipNumber}`;
  const params = new URLSearchParams({ download: "true" });
  if (token) params.set("token", token);
  return `${base}?${params.toString()}`;
}

/** Sanitize a clip title into a safe filename. */
export function formatDownloadFilename(title: string, clipNumber: number): string {
  return `${title.replace(/[^a-z0-9]/gi, "_").toLowerCase()}_clip${clipNumber}.mp4`;
}

/** Programmatically trigger a browser file download. */
export function triggerDownload(url: string, filename: string): void {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

/** Download a single clip — convenience wrapper. */
export function downloadClip(jobId: string, clipNumber: number, title: string, token?: string | null): void {
  triggerDownload(
    getClipDownloadUrl(jobId, clipNumber, token),
    formatDownloadFilename(title, clipNumber),
  );
}

export interface YoutubeVideoInfo {
  title: string;
  duration: number;       // seconds
  uploader: string;
  thumbnail: string | null;
  width: number | null;
  height: number | null;
  estimated_size_bytes: number | null;
}

/** Fetch YouTube video metadata without downloading. */
export async function getYoutubeInfo(url: string, token: string): Promise<YoutubeVideoInfo> {
  const res = await fetch(
    `${ENGINE_BASE}/transcribe/youtube-info?url=${encodeURIComponent(url)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.detail ?? `Failed to fetch video info (${res.status})`);
  }
  return res.json();
}


"use client";

import { useEffect, useRef, useState } from "react";
import { getClipUrl } from "@/lib/api";
import { formatDuration } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/contexts/auth-context";
import {
  getSocialAccounts,
  startUpload,
  getUploadStatus,
  getTikTokConnected,
  startTikTokSetup,
  disconnectTikTok,
  importTikTokCookies,
  type UploadStatus,
} from "@/lib/social-api";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
import {
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Clapperboard,
  Copy,
  Download,
  Expand,
  Loader2,
  Pause,
  Play,
  Star,
  Tag,
  Volume2,
  VolumeX,
  X,
  Youtube,
  CalendarIcon,
  Globe,
} from "lucide-react";
import { toast } from "sonner";
import Link from "next/link";
import type { Clip, Job } from "../types/types";

const TikTokIcon = ({ className }: { className?: string }) => (
  <svg
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    fill="currentColor"
  >
    <path d="M12.525.02c1.31-.02 2.61-.01 3.91-.02.08 1.53.63 3.09 1.75 4.17 1.12 1.11 2.7 1.62 4.24 1.79v4.03c-1.44-.05-2.89-.35-4.2-.97-.57-.26-1.1-.59-1.62-.93-.01 2.92.01 5.84-.02 8.75-.08 2.23-1.13 4.41-2.9 5.67-1.8 1.3-4.25 1.69-6.38 1.05-2.18-.65-4.04-2.31-4.79-4.43-.84-2.37-.32-5.2 1.4-7.07 1.63-1.78 4.28-2.5 6.6-1.92v4.06c-1.07-.35-2.32-.23-3.21.43-.87.64-1.32 1.74-1.2 2.82.12 1.14.99 2.12 2.1 2.45 1.17.34 2.53.1 3.46-.66.86-.7 1.33-1.81 1.33-2.92V.02z" />
  </svg>
);



// ---- Copy button ----

function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <button
      onClick={copy}
      className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
    >
      {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
      {copied ? "Copied!" : label}
    </button>
  );
}

// ---- Video player (seekable, for modal only) ----



function ModalVideoPlayer({ jobId, clipNumber, autoPlay }: { jobId: string; clipNumber: number; autoPlay?: boolean }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const barRef = useRef<HTMLDivElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [hasError, setHasError] = useState(false);
  const [isLoaded, setIsLoaded] = useState(false);
  const [progress, setProgress] = useState(0);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isMuted, setIsMuted] = useState(false);

  const { getToken } = useAuth();
  const src = getClipUrl(jobId, clipNumber, getToken());

  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    
    // reset playback positions each time the video component mounts
    v.load();
    setIsLoaded(false);
    setProgress(0);
    setCurrentTime(0);

    const onTime = () => {
      if (v.duration) {
        setProgress((v.currentTime / v.duration) * 100);
        setCurrentTime(v.currentTime);
      }
    };
    
    v.addEventListener("timeupdate", onTime);
    return () => v.removeEventListener("timeupdate", onTime);
  }, [src]);

  // Spacebar to play/pause
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== " ") return;
      // ignore if user is typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      e.preventDefault();
      const v = videoRef.current;
      if (!v) return;
      v.paused ? v.play() : v.pause();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const togglePlay = () => {
    const v = videoRef.current;
    if (!v) return;
    v.paused ? v.play() : v.pause();
  };

  const toggleMute = () => {
    const v = videoRef.current;
    if (!v) return;
    v.muted = !v.muted;
    setIsMuted(v.muted);
  };

  const toggleFullscreen = () => {
    const el = containerRef.current;
    if (!el) return;
    if (!document.fullscreenElement) {
      el.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  };

  const seek = (e: React.MouseEvent<HTMLDivElement>) => {
    const v = videoRef.current;
    const bar = barRef.current;
    if (!v || !bar) return;
    const rect = bar.getBoundingClientRect();
    v.currentTime = ((e.clientX - rect.left) / rect.width) * v.duration;
  };

  if (hasError) {
    return (
      <div className="w-full h-full min-h-[400px] bg-muted/40 flex flex-col items-center justify-center gap-2 text-muted-foreground/50">
        <Clapperboard className="w-8 h-8" />
        <span className="text-xs">Preview unavailable</span>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="w-full bg-black relative overflow-hidden group aspect-9/16"
    >
      <video
        ref={videoRef}
        src={src}
        className="w-full h-full object-contain"
        preload="metadata"
        playsInline

        onLoadedMetadata={(e) => {
          const v = e.target as HTMLVideoElement;
          setIsLoaded(true);
          setDuration(v.duration);
          if (autoPlay) v.play();
        }}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
        onEnded={() => setIsPlaying(false)}
        onError={() => setHasError(true)}
      />

      {!isLoaded && (
        <div className="absolute inset-0 bg-muted/40 flex items-center justify-center">
          <Loader2 className="w-6 h-6 animate-spin text-muted-foreground/50" />
        </div>
      )}

      {/* Click area for play/pause */}
      {isLoaded && (
        <button
          onClick={togglePlay}
          className="absolute inset-0 w-full h-full cursor-pointer"
          aria-label={isPlaying ? "Pause" : "Play"}
        />
      )}

      {/* Center play icon — only when paused */}
      {isLoaded && !isPlaying && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
          <div className="w-12 h-12 rounded-full bg-black/60 backdrop-blur-sm flex items-center justify-center border border-white/20">
            <Play className="w-5 h-5 text-white fill-white ml-0.5" />
          </div>
        </div>
      )}

      {/* Control bar — appears on hover */}
      {isLoaded && (
        <div className="absolute bottom-0 left-0 right-0 opacity-0 group-hover:opacity-100 pointer-events-none group-hover:pointer-events-auto transition-opacity">
          {/* Gradient background */}
          <div className="bg-linear-to-t from-black/80 to-transparent pt-8 pb-2 px-2">
            {/* Progress bar */}
            <div
              ref={barRef}
              onClick={seek}
              className="w-full h-1 bg-white/30 rounded-full cursor-pointer hover:h-1.5 transition-all mb-2 group/bar"
            >
              <div className="h-full bg-primary rounded-full transition-none" style={{ width: `${progress}%` }} />
            </div>

            {/* Buttons row */}
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-1">
                <button
                  onClick={togglePlay}
                  className="p-1 text-white hover:text-white/80 transition-colors cursor-pointer"
                >
                  {isPlaying
                    ? <Pause className="w-4 h-4 fill-white" />
                    : <Play className="w-4 h-4 fill-white" />}
                </button>
                <button
                  onClick={toggleMute}
                  className="p-1 text-white hover:text-white/80 transition-colors cursor-pointer"
                >
                  {isMuted
                    ? <VolumeX className="w-4 h-4" />
                    : <Volume2 className="w-4 h-4" />}
                </button>
                <span className="text-white/70 text-[11px] font-mono ml-1 select-none">
                  {formatDuration(currentTime)} / {formatDuration(duration)}
                </span>
              </div>
              <button
                onClick={toggleFullscreen}
                className="p-1 text-white hover:text-white/80 transition-colors cursor-pointer"
              >
                <Expand className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ---- Modal ----

export function ClipDetailModal({
  clip,
  job,
  onClose,
  onDownload,
  isDownloading,
  onPrev,
  onNext,
}: {
  clip: Clip;
  job: Job;
  onClose: () => void;
  onDownload: (clipNumber: number, title: string) => void;
  isDownloading: boolean;
  onPrev?: () => void;
  onNext?: () => void;
}) {
  const { authFetch, engineFetch } = useAuth();
  const [showReasoning, setShowReasoning] = useState(false);
  const [showTranscript, setShowTranscript] = useState(false);
  const [showUpload, setShowUpload] = useState(false);
  const [connectedPlatforms, setConnectedPlatforms] = useState<string[] | null>(null);
  const [uploadPlatform, setUploadPlatform] = useState<"youtube" | "tiktok">("youtube");
  const [uploadTitle, setUploadTitle] = useState(clip.title ?? "");
  const [uploadDesc, setUploadDesc] = useState(clip.suggested_caption ?? "");
  const [uploadPrivacy, setUploadPrivacy] = useState<"public" | "unlisted" | "private">("public");
  const [uploadDate, setUploadDate] = useState("");
  const [uploadTime, setUploadTime] = useState("");
  const [uploadStatus, setUploadStatus] = useState<UploadStatus | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [tiktokConnected, setTiktokConnected] = useState<boolean | null>(null);
  const [isTiktokSetupRunning, setIsTiktokSetupRunning] = useState(false);
  const [showCookieImport, setShowCookieImport] = useState(false);
  const [cookiePasteValue, setCookiePasteValue] = useState("");
  const [isImportingCookies, setIsImportingCookies] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const autoPlayRef = useRef(true);
  const touchStartX = useRef<number | null>(null);

  const handleBackdrop = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) onClose();
  };

  useEffect(() => {
    setShowReasoning(false);
    setShowTranscript(false);
    setShowUpload(false);
    // Reset upload state
    setUploadStatus(null);
    setUploadTitle(clip.title ?? "");
    setUploadDesc(clip.suggested_caption ?? "");
    setUploadPrivacy("private");
    setUploadDate("");
    setUploadTime("");
    setIsUploading(false);
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    setConnectedPlatforms(null);
    setTiktokConnected(null);
    let isMounted = true;

    getSocialAccounts(authFetch)
      .then((accounts) => {
        if (isMounted) setConnectedPlatforms(accounts.map((a) => a.platform));
      })
      .catch((err) => {
        console.error("Failed to fetch social accounts:", err);
        if (isMounted) setConnectedPlatforms([]);
      });

    getTikTokConnected(engineFetch)
      .then((connected) => {
        if (isMounted) setTiktokConnected(connected);
      })
      .catch(() => {
        if (isMounted) setTiktokConnected(false);
      });

    return () => {
      isMounted = false;
    };
  }, [clip.clip_id, authFetch]);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const handleUpload = async () => {
    setIsUploading(true);
    let scheduledIso: string | undefined;

    if (uploadDate && uploadTime) {
      try {
        const [year, month, day] = uploadDate.split("-").map(Number);
        const [hour, minute] = uploadTime.split(":").map(Number);
        const localDate = new Date(year, month - 1, day, hour, minute);
        scheduledIso = localDate.toISOString();
      } catch (e) {
        toast.error("Invalid date or time format");
        setIsUploading(false);
        return;
      }
    }

    try {
      const { upload_id } = await startUpload(engineFetch, {
        clip_id: clip.clip_id,
        platform: uploadPlatform,
        // TikTok: caption goes in description field (no separate title)
        title: uploadPlatform === "tiktok" ? uploadDesc : uploadTitle,
        description: uploadPlatform === "tiktok" ? undefined : uploadDesc,
        tags: clip.tags ?? [],
        privacy: uploadPrivacy,
        scheduled_time: uploadPlatform === "tiktok" ? undefined : scheduledIso,
      });
      setUploadStatus({ upload_id, status: "uploading" });
      pollRef.current = setInterval(async () => {
        try {
          const status = await getUploadStatus(engineFetch, upload_id);
          setUploadStatus(status);
          if (status.status === "completed" || status.status === "failed") {
            clearInterval(pollRef.current!);
            pollRef.current = null;
            setIsUploading(false);
          }
        } catch {
          // ignore poll errors
        }
      }, 4000);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Upload failed");
      setIsUploading(false);
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowLeft") { autoPlayRef.current = true; onPrev?.(); }
      if (e.key === "ArrowRight") { autoPlayRef.current = true; onNext?.(); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, onPrev, onNext]);

  const handleTouchStart = (e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX;
  };

  const handleTouchEnd = (e: React.TouchEvent) => {
    if (touchStartX.current === null) return;
    const delta = e.changedTouches[0].clientX - touchStartX.current;
    touchStartX.current = null;
    if (delta > 60) { autoPlayRef.current = true; onPrev?.(); }
    else if (delta < -60) { autoPlayRef.current = true; onNext?.(); }
  };

  return (
    <div
      className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4"
      onClick={handleBackdrop}
      onTouchStart={handleTouchStart}
      onTouchEnd={handleTouchEnd}
    >
      {/* Prev button — outside modal, left side */}
      <button
        onClick={() => { autoPlayRef.current = true; onPrev?.(); }}
        disabled={!onPrev}
        className="shrink-0 mr-3 w-10 h-10 rounded-full bg-background/10 hover:bg-background/30 hover:scale-110 border border-white/10 hover:border-white/30 flex items-center justify-center text-white cursor-pointer disabled:opacity-20 disabled:cursor-not-allowed transition-all"
        title="Previous clip (←)"
      >
        <ChevronLeft className="w-5 h-5" />
      </button>

      <div className="bg-background border border-border rounded-xl shadow-2xl w-full max-w-4xl max-h-[92vh] overflow-hidden flex flex-col sm:flex-row">

        {/* Left: video — full width on mobile, fixed 9:16 sidebar on desktop */}
        <div className="w-full sm:w-[300px] shrink-0 bg-black flex items-center justify-center">
          <ModalVideoPlayer
            key={clip.clip_number}
            jobId={job.id}
            clipNumber={clip.clip_number}
            autoPlay={autoPlayRef.current}
          />
        </div>

        {/* Right: details */}
        <div className="flex-1 flex flex-col min-h-0 overflow-hidden">

          {/* Header */}
          <div className="flex items-start justify-between gap-2 px-5 pt-4 pb-3 border-b border-border">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Badge className="text-xs bg-black/20 text-foreground border-border shrink-0">
                  #{clip.clip_number}
                </Badge>
                {clip.viral_score != null && (
                  <Badge className="text-xs bg-amber-500/20 text-amber-300 border-amber-500/30 shrink-0">
                    <Star className="w-2.5 h-2.5 mr-1 fill-current" />
                    {clip.viral_score.toFixed(1)}
                  </Badge>
                )}
                <span className="text-xs text-muted-foreground">
                  {formatDuration(clip.start)} – {formatDuration(clip.end)}
                  {clip.file_size && (
                    <span className="ml-2">· {(clip.file_size / 1024 / 1024).toFixed(1)} MB</span>
                  )}
                </span>
              </div>
              <h2 className="text-sm font-semibold leading-snug line-clamp-2">{clip.title}</h2>
              {clip.uploads?.filter(u => u.status === "completed").map((upload, idx) => (
                <div key={idx} className="mt-1">
                  <a 
                    href={upload.url} 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 text-xs font-medium text-red-500 hover:text-red-400 hover:underline transition-colors"
                  >
                    {upload.platform === "youtube" ? <Youtube className="w-3.5 h-3.5" /> : upload.platform === "tiktok" ? <TikTokIcon className="w-3.5 h-3.5" /> : <Globe className="w-3.5 h-3.5" />}
                    View on {upload.platform}
                  </a>
                </div>
              ))}
            </div>
            <button
              onClick={onClose}
              className="text-muted-foreground hover:text-foreground transition-colors mt-0.5 shrink-0 cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* Scrollable content */}
          <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">

            {/* Hook */}
            {clip.hook_text && (
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Hook</p>
                <p className="text-sm italic text-foreground/80 leading-relaxed border-l-2 border-primary/40 pl-3">
                  "{clip.hook_text}"
                </p>
              </div>
            )}

            {/* Caption */}
            {clip.suggested_caption && (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Caption</p>
                  <CopyButton text={clip.suggested_caption} label="Copy Caption" />
                </div>
                <p className="text-sm text-foreground/80 leading-relaxed bg-muted/30 rounded-lg px-3 py-2.5">
                  {clip.suggested_caption}
                </p>
              </div>
            )}

            {/* Tags */}
            {clip.tags && clip.tags.length > 0 && (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide flex items-center gap-1">
                    <Tag className="w-3 h-3" /> Tags
                  </p>
                  <CopyButton text={clip.tags.join(" ")} label="Copy Tags" />
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {clip.tags.map((tag) => (
                    <button
                      key={tag}
                      onClick={() => {
                        navigator.clipboard.writeText(tag);
                        toast.success(`Copied: ${tag}`);
                      }}
                      className="text-xs px-2 py-0.5 rounded-full bg-primary/10 text-primary border border-primary/20 hover:bg-primary/20 transition-colors"
                    >
                      {tag}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Reasoning */}
            {clip.reasoning && (
              <div className="space-y-1.5">
                <button
                  onClick={() => setShowReasoning(!showReasoning)}
                  className="flex items-center gap-1 text-xs font-medium text-muted-foreground uppercase tracking-wide hover:text-foreground transition-colors cursor-pointer"
                >
                  Why this clip?
                  {showReasoning ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                </button>
                {showReasoning && (
                  <p className="text-xs text-muted-foreground leading-relaxed bg-muted/30 rounded-lg px-3 py-2.5">
                    {clip.reasoning}
                  </p>
                )}
              </div>
            )}

            {/* Transcript */}
            {clip.transcript_text && (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => setShowTranscript(!showTranscript)}
                    className="flex items-center gap-1 text-xs font-medium text-muted-foreground uppercase tracking-wide hover:text-foreground transition-colors cursor-pointer"
                  >
                    Transcript
                    {showTranscript ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {showTranscript && <CopyButton text={clip.transcript_text} />}
                </div>
                {showTranscript && (
                  <p className="text-xs text-muted-foreground leading-relaxed bg-muted/30 rounded-lg px-3 py-2.5 whitespace-pre-wrap">
                    {clip.transcript_text}
                  </p>
                )}
              </div>
            )}

              <button
                onClick={() => setShowUpload(!showUpload)}
                className="flex items-center gap-1 text-xs font-medium text-muted-foreground uppercase tracking-wide hover:text-foreground transition-colors cursor-pointer"
              >
                Upload to Social Media
                {showUpload ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
              </button>

              {showUpload && (
                <div className="space-y-3 pl-1">
                  <div className="space-y-1">
                    <label className="text-[11px] font-medium text-muted-foreground block">Platform</label>
                    <Select
                      value={uploadPlatform}
                      onValueChange={(val) => setUploadPlatform(val as "youtube" | "tiktok")}
                      disabled={isUploading}
                    >
                      <SelectTrigger className="w-full h-8 text-xs bg-muted/30 border-border cursor-pointer">
                        <SelectValue placeholder="Select platform" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="youtube" className="text-xs">
                          <div className="flex items-center gap-2"><Youtube className="w-3.5 h-3.5 text-red-500" /> YouTube</div>
                        </SelectItem>
                        <SelectItem value="tiktok" className="text-xs">
                          <div className="flex items-center gap-2"><TikTokIcon className="w-3.5 h-3.5 text-black dark:text-white" /> TikTok</div>
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  {/* Connection check — TikTok uses Playwright session, YouTube uses OAuth */}
                  {uploadPlatform === "tiktok" ? (
                    tiktokConnected === null ? (
                      <p className="text-xs text-muted-foreground">Checking TikTok session…</p>
                    ) : !tiktokConnected ? (
                      <div className="space-y-3">
                        {/* Option 1: Browser login */}
                        <div className="space-y-1.5">
                          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide">Option 1 — Browser Login</p>
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 text-xs gap-1.5 w-full"
                            disabled={isTiktokSetupRunning}
                            onClick={async () => {
                              setIsTiktokSetupRunning(true);
                              try {
                                await startTikTokSetup(engineFetch);
                                setTiktokConnected(true);
                                toast.success("TikTok session saved!");
                              } catch (err) {
                                toast.error(err instanceof Error ? err.message : "TikTok setup failed");
                              } finally {
                                setIsTiktokSetupRunning(false);
                              }
                            }}
                          >
                            {isTiktokSetupRunning ? (
                              <><Loader2 className="w-3 h-3 animate-spin" /> Browser opening… log in within 3 min</>
                            ) : (
                              <><TikTokIcon className="w-3 h-3" /> Open Browser & Login</>
                            )}
                          </Button>
                        </div>

                        {/* Option 2: Cookie import */}
                        <div className="space-y-1.5">
                          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide">Option 2 — Import Cookies</p>
                          <p className="text-[11px] text-muted-foreground">
                            Install{" "}
                            <a href="https://cookie-editor.com" target="_blank" rel="noopener noreferrer" className="text-primary underline-offset-2 hover:underline">Cookie-Editor</a>
                            {" "}→ buka tiktok.com → Export as JSON → paste di bawah.
                          </p>
                          <button
                            className="text-xs text-primary hover:underline cursor-pointer"
                            onClick={() => setShowCookieImport(v => !v)}
                          >
                            {showCookieImport ? "Hide" : "Paste cookies JSON…"}
                          </button>
                          {showCookieImport && (
                            <div className="space-y-2">
                              <textarea
                                value={cookiePasteValue}
                                onChange={e => setCookiePasteValue(e.target.value)}
                                placeholder='[{"name":"sessionid","value":"...","domain":".tiktok.com",...}]'
                                className="w-full text-[11px] rounded-md px-2 py-2 bg-muted/30 border border-border focus:outline-none focus:ring-1 focus:ring-primary min-h-[80px] resize-y font-mono"
                              />
                              <Button
                                size="sm"
                                className="h-7 text-xs w-full"
                                disabled={isImportingCookies || !cookiePasteValue.trim()}
                                onClick={async () => {
                                  setIsImportingCookies(true);
                                  try {
                                    const count = await importTikTokCookies(engineFetch, cookiePasteValue.trim());
                                    setTiktokConnected(true);
                                    setCookiePasteValue("");
                                    setShowCookieImport(false);
                                    toast.success(`TikTok cookies imported (${count} cookies). You can now upload!`);
                                  } catch (err) {
                                    toast.error(err instanceof Error ? err.message : "Import failed");
                                  } finally {
                                    setIsImportingCookies(false);
                                  }
                                }}
                              >
                                {isImportingCookies ? <><Loader2 className="w-3 h-3 animate-spin" /> Importing…</> : "Import & Save"}
                              </Button>
                            </div>
                          )}
                        </div>
                      </div>
                    ) : (
                      /* TikTok connected — show switch account option */
                      <div className="flex items-center justify-between">
                        <p className="text-xs text-emerald-500">TikTok session active</p>
                        <button
                          className="text-[11px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                          onClick={async () => {
                            try {
                              await disconnectTikTok(engineFetch);
                              setTiktokConnected(false);
                              toast.success("TikTok session removed. You can log in with a different account.");
                            } catch (err) {
                              toast.error(err instanceof Error ? err.message : "Failed to disconnect");
                            }
                          }}
                        >
                          Switch account
                        </button>
                      </div>
                    )
                  ) : connectedPlatforms === null ? (
                    <p className="text-xs text-muted-foreground">Checking connection…</p>
                  ) : !connectedPlatforms.includes(uploadPlatform) ? (
                    <p className="text-xs text-muted-foreground">
                      YouTube not connected.{" "}
                      <Link
                        href="/settings"
                        className="text-primary underline-offset-2 hover:underline"
                      >
                        Connect in Settings
                      </Link>
                    </p>
                  ) : null}

                  {/* Upload form — shown when connected */}
                  {(uploadPlatform === "tiktok" ? tiktokConnected : connectedPlatforms?.includes(uploadPlatform)) && (
                    <>
                      {/* Past uploads — show only latest, with re-upload option */}
                      {(() => {
                        const prev = clip.uploads?.filter(u => u.platform === uploadPlatform && u.status === "completed") ?? [];
                        if (prev.length === 0 || uploadStatus?.status === "completed") return null;
                        const latest = prev[prev.length - 1];
                        return (
                          <div className="flex items-center justify-between text-xs bg-emerald-500/10 border border-emerald-500/20 px-2 py-1.5 rounded-md">
                            <span className="text-emerald-500 flex items-center gap-1.5">
                              ✅ Uploaded to {uploadPlatform === "youtube" ? "YouTube" : "TikTok"}
                              {latest.url && uploadPlatform === "youtube" && (
                                <a href={latest.url} target="_blank" rel="noopener noreferrer" className="underline hover:text-emerald-400 font-medium">View</a>
                              )}
                            </span>
                            <span className="text-muted-foreground/60">Re-upload below</span>
                          </div>
                        );
                      })()}

                      {/* Current upload status */}
                      {uploadStatus?.status === "uploading" && (
                        <div className="flex items-center gap-2 text-xs text-blue-400 bg-blue-500/10 border border-blue-500/20 px-2 py-1.5 rounded-md">
                          <Loader2 className="w-3 h-3 animate-spin shrink-0" />
                          <span>{uploadPlatform === "tiktok" ? "Browser uploading to TikTok… (opening Chrome in background)" : "Uploading to YouTube…"}</span>
                        </div>
                      )}

                      {uploadStatus?.status === "completed" && (
                        <div className="flex items-center gap-2 text-xs text-emerald-500 bg-emerald-500/10 border border-emerald-500/20 px-2 py-1.5 rounded-md">
                          <span>✅ Upload complete!</span>
                          {uploadStatus.platform_url && uploadPlatform === "youtube" && (
                            <a href={uploadStatus.platform_url} target="_blank" rel="noopener noreferrer" className="underline hover:text-emerald-400 font-medium">View on YouTube</a>
                          )}
                          {uploadPlatform === "tiktok" && (
                            <span className="text-emerald-400/70">Check your TikTok profile.</span>
                          )}
                        </div>
                      )}

                      {uploadStatus?.status === "failed" && (
                        <div className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 px-2 py-1.5 rounded-md">
                          {uploadStatus.error ?? "Upload failed"}
                        </div>
                      )}

                      <div className="space-y-3 mt-1">
                        {/* TikTok: single Caption field. YouTube: Title + Description */}
                        {uploadPlatform === "tiktok" ? (
                          <div className="space-y-1">
                            <label className="text-[11px] font-medium text-muted-foreground">Caption</label>
                            <textarea
                              value={uploadDesc}
                              onChange={(e) => setUploadDesc(e.target.value)}
                              placeholder="Caption (hashtags auto-included)"
                              disabled={isUploading}
                              rows={3}
                              className="w-full text-xs rounded-md px-3 py-2 bg-muted/30 border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-y disabled:opacity-50"
                            />
                          </div>
                        ) : (
                          <>
                            <div className="space-y-1">
                              <label className="text-[11px] font-medium text-muted-foreground">Title</label>
                              <input
                                type="text"
                                value={uploadTitle}
                                onChange={(e) => setUploadTitle(e.target.value)}
                                placeholder="Video title"
                                disabled={isUploading}
                                className="w-full text-xs rounded-md px-3 py-2 bg-muted/30 border border-border focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50"
                              />
                            </div>
                            <div className="space-y-1">
                              <label className="text-[11px] font-medium text-muted-foreground">Description</label>
                              <textarea
                                value={uploadDesc}
                                onChange={(e) => setUploadDesc(e.target.value)}
                                placeholder="Description (optional)"
                                disabled={isUploading}
                                className="w-full text-xs rounded-md px-3 py-2 bg-muted/30 border border-border focus:outline-none focus:ring-1 focus:ring-primary min-h-[60px] resize-y disabled:opacity-50"
                              />
                            </div>
                          </>
                        )}

                        {/* Visibility + Schedule — YouTube only */}
                        {uploadPlatform === "youtube" && (
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                            <div className="space-y-1">
                              <label className="text-[11px] font-medium text-muted-foreground block">Visibility</label>
                              <Select
                                value={uploadPrivacy}
                                onValueChange={(val) => setUploadPrivacy(val as "public" | "unlisted" | "private")}
                                disabled={isUploading}
                              >
                                <SelectTrigger className="w-full h-8 text-xs bg-muted/30 border-border cursor-pointer">
                                  <SelectValue placeholder="Select visibility" />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="public" className="text-xs">Public</SelectItem>
                                  <SelectItem value="unlisted" className="text-xs">Unlisted</SelectItem>
                                  <SelectItem value="private" className="text-xs">Private</SelectItem>
                                </SelectContent>
                              </Select>
                            </div>
                            <div className="space-y-1">
                              <label className="text-[11px] font-medium text-muted-foreground flex items-center justify-between">
                                Schedule
                                {uploadDate && (
                                  <button
                                    onClick={() => { setUploadDate(""); setUploadTime(""); }}
                                    className="text-[10px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                                  >
                                    Clear
                                  </button>
                                )}
                              </label>
                              <div className="flex flex-col sm:flex-row items-center gap-1.5">
                                <Popover>
                                  <PopoverTrigger asChild>
                                    <Button
                                      variant={"outline"}
                                      disabled={isUploading}
                                      className={cn(
                                        "w-full sm:flex-1 h-8 justify-start text-left font-normal bg-muted/30 hover:bg-muted/50 border-border px-2 text-xs",
                                        !uploadDate && "text-muted-foreground"
                                      )}
                                    >
                                      <CalendarIcon className="mr-1.5 h-3.5 w-3.5" />
                                      {uploadDate ? format(new Date(uploadDate), "PPP") : <span>Date</span>}
                                    </Button>
                                  </PopoverTrigger>
                                  <PopoverContent className="w-auto p-0 cursor-pointer" align="start">
                                    <Calendar
                                      mode="single"
                                      selected={uploadDate ? new Date(uploadDate) : undefined}
                                      onSelect={(day) => {
                                        if (day) {
                                          const localString = `${day.getFullYear()}-${String(day.getMonth()+1).padStart(2, '0')}-${String(day.getDate()).padStart(2, '0')}`;
                                          setUploadDate(localString);
                                        } else {
                                          setUploadDate("");
                                        }
                                      }}
                                      disabled={(date) => date < new Date(new Date().setHours(0,0,0,0))}
                                      initialFocus
                                    />
                                  </PopoverContent>
                                </Popover>
                                <div className="relative w-full sm:w-22 shrink-0">
                                  <Input
                                    type="time"
                                    disabled={isUploading}
                                    className="w-full h-8 bg-muted/30 border-border px-2 text-xs cursor-pointer scheme-dark [&::-webkit-calendar-picker-indicator]:absolute [&::-webkit-calendar-picker-indicator]:inset-0 [&::-webkit-calendar-picker-indicator]:w-full [&::-webkit-calendar-picker-indicator]:h-full [&::-webkit-calendar-picker-indicator]:opacity-0 [&::-webkit-calendar-picker-indicator]:cursor-pointer"
                                    value={uploadTime}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setUploadTime(e.target.value)}
                                  />
                                </div>
                              </div>
                            </div>
                          </div>
                        )}

                        {clip.tags && clip.tags.length > 0 && (
                          <div className="space-y-1">
                            <label className="text-[11px] font-medium text-muted-foreground">
                              Tags <span className="text-muted-foreground/60 font-normal">(auto-included)</span>
                            </label>
                            <div className="flex flex-wrap gap-1">
                              {clip.tags.map((tag) => (
                                <span
                                  key={tag}
                                  className="text-[10px] px-1.5 py-0.5 rounded-full bg-muted/50 text-muted-foreground border border-border/50"
                                >
                                  {tag}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>

                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            disabled={isUploading || (uploadPlatform === "tiktok" ? !uploadDesc.trim() : !uploadTitle.trim())}
                            variant="outline"
                            className="w-full flex-1 cursor-pointer bg-primary text-primary-foreground hover:bg-primary/90"
                          >
                          {isUploading ? (
                            <span className="flex items-center gap-2">
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              Uploading…
                            </span>
                          ) : (
                            <span className="flex items-center gap-2">
                              {uploadPlatform === "youtube" ? <Youtube className="w-3.5 h-3.5" /> : <TikTokIcon className="w-3.5 h-3.5" />}
                              {clip.uploads?.some(u => u.platform === uploadPlatform && u.status === "completed") || uploadStatus?.status === "completed"
                                ? `Re-upload to ${uploadPlatform === "youtube" ? "YouTube" : "TikTok"}`
                                : `Upload to ${uploadPlatform === "youtube" ? "YouTube" : "TikTok"}`}
                            </span>
                          )}
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent className="w-[90vw] max-w-[400px]">
                          <AlertDialogHeader>
                            <AlertDialogTitle>Upload to {uploadPlatform === "youtube" ? "YouTube" : "TikTok"}?</AlertDialogTitle>
                            <AlertDialogDescription>
                              {uploadPlatform === "tiktok"
                                ? <>This will open Chrome in the background and automatically post clip #{clip.clip_number} to your TikTok account.</>
                                : <>This will upload clip #{clip.clip_number} to your YouTube channel as a <strong>{uploadPrivacy}</strong> video{uploadDate && uploadTime ? <span> scheduled for <strong>{uploadDate} at {uploadTime}</strong></span> : ""}.</>
                              }
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                            <AlertDialogAction onClick={handleUpload} className="bg-primary text-primary-foreground">
                              Yes, upload video
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </>
                  )}
                </div>
              )}
            </div>

          {/* Footer */}
          <div className="px-5 py-3 border-t border-border">
            <Button
              className="w-full h-9 text-sm"
              onClick={() => onDownload(clip.clip_number, clip.title)}
              disabled={isDownloading}
            >
              {isDownloading ? (
                <><Loader2 className="w-3.5 h-3.5 mr-2 animate-spin" />Downloading…</>
              ) : (
                <><Download className="w-3.5 h-3.5 mr-2" />Download MP4</>
              )}
            </Button>
          </div>
        </div>
      </div>

      {/* Next button — outside modal, right side */}
      <button
        onClick={() => { autoPlayRef.current = true; onNext?.(); }}
        disabled={!onNext}
        className="shrink-0 ml-3 w-10 h-10 rounded-full bg-background/10 hover:bg-background/30 hover:scale-110 border border-white/10 hover:border-white/30 flex items-center justify-center text-white cursor-pointer disabled:opacity-20 disabled:cursor-not-allowed transition-all"
        title="Next clip (→)"
      >
        <ChevronRight className="w-5 h-5" />
      </button>
    </div>
  );
}

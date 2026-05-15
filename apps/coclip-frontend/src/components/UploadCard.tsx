"use client";

import { useRef, useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Loader2, Upload, Wand2, X, Youtube, Clock, HardDrive, User } from "lucide-react";
import { getYoutubeInfo, YoutubeVideoInfo } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";

export interface UploadSubmitPayload {
  mode: "file" | "youtube";
  file?: File;
  url?: string;
  jobName?: string;
}

interface UploadCardProps {
  isSubmitting: boolean;
  onSubmit: (payload: UploadSubmitPayload) => Promise<void>;
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

const YOUTUBE_RE = /^https?:\/\/(www\.)?(youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/shorts\/)/;

export function UploadCard({ isSubmitting, onSubmit }: UploadCardProps) {
  const { getToken } = useAuth();
  const [uploadMode, setUploadMode] = useState<"file" | "youtube">("youtube");
  const [file, setFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [youtubeUrl, setYoutubeUrl] = useState("");
  const [jobName, setJobName] = useState("");
  const [showConfirm, setShowConfirm] = useState(false);
  const [videoInfo, setVideoInfo] = useState<YoutubeVideoInfo | null>(null);
  const [infoLoading, setInfoLoading] = useState(false);
  const [infoError, setInfoError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Auto-fetch video info when a valid YouTube URL is typed (500ms debounce)
  useEffect(() => {
    setVideoInfo(null);
    setInfoError(null);
    if (!YOUTUBE_RE.test(youtubeUrl.trim())) return;

    const token = getToken();
    if (!token) return;

    const timer = setTimeout(async () => {
      setInfoLoading(true);
      try {
        const info = await getYoutubeInfo(youtubeUrl.trim(), token);
        setVideoInfo(info);
      } catch (e: unknown) {
        setInfoError(e instanceof Error ? e.message : "Could not fetch video info");
      } finally {
        setInfoLoading(false);
      }
    }, 500);

    return () => clearTimeout(timer);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [youtubeUrl]);

  const canSubmit =
    !isSubmitting &&
    (uploadMode === "file" ? !!file : youtubeUrl.trim().length > 0);

  const confirmLabel =
    uploadMode === "file" ? file?.name ?? "" : youtubeUrl.trim();

  const handleFileSelect = (f: File) => {
    setFile(f);
    setShowConfirm(true);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (f) handleFileSelect(f);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = () => setIsDragging(false);

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const f = e.dataTransfer.files?.[0];
    if (f) handleFileSelect(f);
  };

  const handleConfirmSubmit = () => {
    if (canSubmit) setShowConfirm(true);
  };

  const handleSubmit = async () => {
    setShowConfirm(false);
    await onSubmit({
      mode: uploadMode,
      file: uploadMode === "file" ? file ?? undefined : undefined,
      url: uploadMode === "youtube" ? youtubeUrl.trim() : undefined,
      jobName: jobName.trim() || undefined,
    });
    // Reset on success (parent controls isSubmitting)
    setFile(null);
    setYoutubeUrl("");
    setJobName("");
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  return (
    <>
      {/* Confirm dialog */}
      {showConfirm && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border rounded-xl shadow-xl w-full max-w-md p-7 space-y-5">
            <div className="space-y-1">
              <h2 className="text-base font-semibold">Start processing?</h2>
              <p className="text-sm text-muted-foreground line-clamp-2 break-all">
                {videoInfo?.title ?? confirmLabel}
              </p>
            </div>

            {/* Video info summary in confirm dialog */}
            {uploadMode === "youtube" && videoInfo && (
              <div className="flex flex-wrap gap-3 text-xs text-muted-foreground bg-muted/40 rounded-lg px-3 py-2.5">
                <span className="flex items-center gap-1.5">
                  <Clock className="w-3 h-3" />
                  {formatDuration(videoInfo.duration)}
                </span>
                {videoInfo.estimated_size_bytes && (
                  <span className="flex items-center gap-1.5 font-medium text-foreground">
                    <HardDrive className="w-3 h-3" />
                    ~{formatBytes(videoInfo.estimated_size_bytes)}
                  </span>
                )}
                <span className="flex items-center gap-1.5">
                  <User className="w-3 h-3" />
                  {videoInfo.uploader}
                </span>
              </div>
            )}

            <div className="space-y-2 pt-2">
              <label className="text-sm font-medium">Collection Name (Optional)</label>
              <Input
                className="h-10 text-sm"
                placeholder={videoInfo?.title ?? "Custom name..."}
                value={jobName}
                onChange={(e) => setJobName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSubmit()}
                autoFocus
              />
            </div>

            <p className="text-xs text-muted-foreground pt-1 border-t border-border mt-3">
              This will transcribe and analyze the video using AI. Processing may take several minutes depending on video length.
            </p>
            <div className="flex gap-2 justify-end pt-1">
              <Button variant="outline" size="sm" onClick={() => setShowConfirm(false)}>
                Cancel
              </Button>
              <Button size="sm" onClick={handleSubmit} disabled={isSubmitting}>
                {isSubmitting ? (
                  <><Loader2 className="w-3.5 h-3.5 mr-1.5 animate-spin" />Starting…</>
                ) : (
                  <><Wand2 className="w-3.5 h-3.5 mr-1.5" />Generate Clips</>
                )}
              </Button>
            </div>
          </div>
        </div>
      )}

      <Card className="border-border/50 shadow-sm h-full flex flex-col">
        <CardHeader className="pb-3 border-b border-border/50 bg-muted/20">
          <CardTitle className="text-base flex items-center gap-2">
            <Wand2 className="w-4 h-4 text-primary" />
            Create New Clips
          </CardTitle>
          <CardDescription>
            Upload a video file or paste a YouTube URL to get started
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5 pt-5 flex-1 flex flex-col">
          {/* Mode toggle */}
          <div className="flex rounded-lg border border-border p-1 bg-muted/30 w-full sm:w-auto sm:inline-flex">
            <button
              type="button"
              onClick={() => setUploadMode("youtube")}
              className={`flex-1 sm:flex-none px-4 py-2 text-sm font-medium transition-all rounded-md flex items-center justify-center gap-2 ${
                uploadMode === "youtube"
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
              }`}
            >
              <Youtube className="w-3.5 h-3.5" />
              YouTube URL
            </button>
            <button
              type="button"
              onClick={() => setUploadMode("file")}
              className={`flex-1 sm:flex-none px-4 py-2 text-sm font-medium transition-all rounded-md flex items-center justify-center gap-2 ${
                uploadMode === "file"
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
              }`}
            >
              <Upload className="w-3.5 h-3.5" />
              Upload File
            </button>
          </div>

          {/* Input area */}
          {uploadMode === "file" ? (
            <div className="relative flex-1 flex flex-col">
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={`border-2 border-dashed rounded-xl p-10 text-center cursor-pointer transition-all select-none ${
                  isDragging
                    ? "border-primary bg-primary/5"
                    : "border-border hover:border-primary/40 hover:bg-muted/20"
                }`}
              >
                <Upload className="w-8 h-8 text-muted-foreground mx-auto mb-3" />
                {file ? (
                  <div>
                    <p className="text-sm font-medium text-foreground">{file.name}</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {(file.size / 1024 / 1024).toFixed(1)} MB
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-sm font-medium">Drop your video here or click to browse</p>
                    <p className="text-xs text-muted-foreground mt-1">MP4, MOV, MKV, AVI, WebM</p>
                  </div>
                )}
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="video/*,audio/*"
                  className="hidden"
                  onChange={handleFileChange}
                />
              </div>
              {file && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setFile(null);
                    if (fileInputRef.current) fileInputRef.current.value = "";
                  }}
                  className="absolute top-2 right-2 p-1 rounded-full bg-muted hover:bg-destructive/10 hover:text-destructive text-muted-foreground transition-colors cursor-pointer"
                  title="Clear file"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
          ) : (
            <div className="flex-1 flex flex-col gap-2">
              <div className="relative">
                <Input
                  placeholder="https://youtube.com/watch?v=..."
                  value={youtubeUrl}
                  onChange={(e) => setYoutubeUrl(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && canSubmit && handleConfirmSubmit()}
                  className={infoError ? "border-destructive" : ""}
                />
                {infoLoading && (
                  <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 animate-spin text-muted-foreground" />
                )}
              </div>

              {/* Video info preview card */}
              {videoInfo && !infoLoading && (
                <div className="rounded-lg border border-border bg-muted/30 p-3 space-y-1.5">
                  <p className="text-sm font-medium line-clamp-2 leading-snug">{videoInfo.title}</p>
                  <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {formatDuration(videoInfo.duration)}
                    </span>
                    {videoInfo.estimated_size_bytes && (
                      <span className="flex items-center gap-1 font-semibold text-foreground">
                        <HardDrive className="w-3 h-3" />
                        ~{formatBytes(videoInfo.estimated_size_bytes)}
                      </span>
                    )}
                    <span className="flex items-center gap-1">
                      <User className="w-3 h-3" />
                      {videoInfo.uploader}
                    </span>
                  </div>
                </div>
              )}

              {infoError && (
                <p className="text-xs text-destructive">{infoError}</p>
              )}
            </div>
          )}

          <Button className="w-full mt-2" disabled={!canSubmit} onClick={handleConfirmSubmit}>
            Continue
          </Button>
        </CardContent>
      </Card>
    </>
  );
}

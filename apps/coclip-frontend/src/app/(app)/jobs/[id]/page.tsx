"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/auth-context";
import { downloadClip } from "@/lib/api";
import { formatDuration } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ArrowLeft,
  ArrowUpDown,
  Clapperboard,
  Clock,
  Download,
  Film,
  Loader2,
  Play,
  Star,
  CheckSquare,
  Square,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { ClipDetailModal } from "../../../../components/ClipDetailModal";
import { ClipCard } from "@/components/ClipCard";
import type { Clip, Job } from "../../../../types/types";
import { startUpload, getSocialAccounts, getUploadStatus, getTikTokConnected, startTikTokSetup, importTikTokCookies, disconnectTikTok } from "@/lib/social-api";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { BulkUploadSettingsModal, type ClipConfig } from "../../../../components/BulkUploadSettingsModal";

// ---- Helpers ----

const DAILY_UPLOAD_SLOTS = [8, 10, 12, 15, 18, 20, 22]; // jam dalam sehari

function getNextUploadSlot(after: Date): Date {
  for (const hour of DAILY_UPLOAD_SLOTS) {
    const slot = new Date(after);
    slot.setHours(hour, 0, 0, 0);
    if (slot > after) return slot;
  }
  // Semua slot habis → putar balik ke 08:00 di tanggal yang sama
  const wrapped = new Date(after);
  wrapped.setHours(8, 0, 0, 0);
  return wrapped;
}

function StatusBadge({ status }: { status: string }) {
  if (status === "completed")
    return (
      <Badge variant="outline" className="bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
        Completed
      </Badge>
    );
  if (status === "failed" || status === "aborted")
    return <Badge variant="destructive">{status}</Badge>;
  return <Badge variant="outline" className="capitalize">{status}</Badge>;
}

// ---- Page ----

export default function JobDetailPage() {
  const router = useRouter();
  const params = useParams();
  const jobId = params.id as string;
  const { engineFetch, authFetch, getToken } = useAuth();

  const [job, setJob] = useState<Job | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [downloadingClip, setDownloadingClip] = useState<number | null>(null);
  const [selectedClip, setSelectedClip] = useState<Clip | null>(null);
  const [sortBy, setSortBy] = useState<"clip_number" | "viral_score">("clip_number");
  const [filterStatus, setFilterStatus] = useState<"all" | "completed" | "failed">("all");
  const [isDownloadingAll, setIsDownloadingAll] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [isDeletingJob, setIsDeletingJob] = useState(false);

  // Bulk selection state
  const [isSelectionMode, setIsSelectionMode] = useState(false);
  const [selectedClipIds, setSelectedClipIds] = useState<Set<string>>(new Set());
  const [isBulkUploading, setIsBulkUploading] = useState(false);
  const [bulkUploadProgress, setBulkUploadProgress] = useState({ current: 0, total: 0 });
  const [showBulkUploadSettings, setShowBulkUploadSettings] = useState(false);
  const [bulkConfigs, setBulkConfigs] = useState<Record<string, ClipConfig>>({});
  const [showLeaveWarning, setShowLeaveWarning] = useState(false);
  const [showUploadSuccess, setShowUploadSuccess] = useState(false);
  const [uploadSuccessCount, setUploadSuccessCount] = useState(0);
  const uploadCancelledRef = useRef(false);
  const [tiktokConnected, setTiktokConnected] = useState<boolean | null>(null);
  const [isConnectingTikTok, setIsConnectingTikTok] = useState(false);
  const [bulkCookiePaste, setBulkCookiePaste] = useState("");
  const [isImportingBulkCookies, setIsImportingBulkCookies] = useState(false);
  const tiktokSetupAbortRef = useRef<AbortController | null>(null);

  const fetchJob = useCallback(async () => {
    try {
      const res = await engineFetch(`/jobs/${jobId}`);
      if (!res.ok) {
        toast.error("Job not found");
        router.push("/dashboard");
        return;
      }
      setJob(await res.json());
    } catch {
      toast.error("Failed to load job");
      router.push("/dashboard");
    } finally {
      setIsLoading(false);
    }
  }, [jobId, engineFetch, router]);

  useEffect(() => { fetchJob(); }, [fetchJob]);

  // Navigation guard saat upload berlangsung
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isBulkUploading) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    const handlePopState = () => {
      if (isBulkUploading) {
        window.history.pushState(null, "", window.location.href);
        setShowLeaveWarning(true);
      }
    };
    if (isBulkUploading) {
      window.history.pushState(null, "", window.location.href);
      window.addEventListener("beforeunload", handleBeforeUnload);
      window.addEventListener("popstate", handlePopState);
    }
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      window.removeEventListener("popstate", handlePopState);
    };
  }, [isBulkUploading]);

  const handleDeleteJob = async () => {
    setIsDeletingJob(true);
    try {
      const res = await engineFetch(`/jobs/${jobId}`, { method: "DELETE" });
      if (!res.ok) throw new Error("Failed to delete");
      toast.success("Collection deleted");
      router.push("/dashboard");
    } catch {
      toast.error("Failed to delete collection");
      setIsDeletingJob(false);
      setShowDeleteDialog(false);
    }
  };

  const handleDownload = (clipNumber: number, title: string) => {
    downloadClip(jobId, clipNumber, title, getToken());
    toast.success(`Download started for Clip ${clipNumber}`);
  };

  const handleDownloadAll = async (clips: Clip[]) => {
    if (isDownloadingAll) return;
    setIsDownloadingAll(true);
    toast.info(`Starting download for ${clips.length} clips…`);
    const token = getToken();
    for (const clip of clips) {
      downloadClip(jobId, clip.clip_number, clip.title, token);
      await new Promise((r) => setTimeout(r, 800));
    }
    setIsDownloadingAll(false);
    toast.success("All downloads started");
  };

  const toggleSelectionMode = () => {
    setIsSelectionMode(!isSelectionMode);
    setSelectedClipIds(new Set());
  };

  const toggleClipSelection = (clipId: string, checked: boolean) => {
    const newSet = new Set(selectedClipIds);
    if (checked) newSet.add(clipId);
    else newSet.delete(clipId);
    setSelectedClipIds(newSet);
  };

  const handleConnectTikTok = async () => {
    const abort = new AbortController();
    tiktokSetupAbortRef.current = abort;
    setIsConnectingTikTok(true);
    try {
      await startTikTokSetup(engineFetch, abort.signal);
      setTiktokConnected(true);
      toast.success("TikTok connected!");
    } catch (err: any) {
      if (err?.name !== "AbortError") {
        toast.error(err instanceof Error ? err.message : "TikTok setup failed");
      }
    } finally {
      setIsConnectingTikTok(false);
      tiktokSetupAbortRef.current = null;
    }
  };

  const handleDisconnectTikTok = async () => {
    try {
      await disconnectTikTok(engineFetch);
      setTiktokConnected(false);
      toast.success("TikTok session removed.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to disconnect");
    }
  };

  const handleCancelTikTokSetup = () => {
    tiktokSetupAbortRef.current?.abort();
    setIsConnectingTikTok(false);
  };

  const handleImportBulkCookies = async () => {
    if (!bulkCookiePaste.trim()) return;
    setIsImportingBulkCookies(true);
    try {
      await importTikTokCookies(engineFetch, bulkCookiePaste.trim());
      setTiktokConnected(true);
      setBulkCookiePaste("");
      toast.success("TikTok cookies imported!");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to import cookies");
    } finally {
      setIsImportingBulkCookies(false);
    }
  };

  const handleBulkUploadClick = () => {
    if (selectedClipIds.size === 0) return;

    // Fetch TikTok connection status when opening bulk upload
    getTikTokConnected(engineFetch)
      .then(setTiktokConnected)
      .catch(() => setTiktokConnected(false));

    const newConfigs: Record<string, ClipConfig> = {};
    const clipsToUpload = job?.clips.filter(c => selectedClipIds.has(c.clip_id)) || [];

    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(8, 0, 0, 0);
    let currentSlot = tomorrow;

    clipsToUpload.forEach((clip) => {
      const year = currentSlot.getFullYear();
      const month = String(currentSlot.getMonth() + 1).padStart(2, '0');
      const day = String(currentSlot.getDate()).padStart(2, '0');
      const hours = String(currentSlot.getHours()).padStart(2, '0');
      const mins = String(currentSlot.getMinutes()).padStart(2, '0');

      newConfigs[clip.clip_id] = {
        platform: "youtube",
        privacy: "public",
        date: `${year}-${month}-${day}`,
        time: `${hours}:${mins}`
      };

      currentSlot = getNextUploadSlot(currentSlot);
    });

    setBulkConfigs(newConfigs);
    setShowBulkUploadSettings(true);
  };

  const updateClipConfig = (clipId: string, field: keyof ClipConfig, value: string) => {
    setBulkConfigs(prev => ({
      ...prev,
      [clipId]: { ...prev[clipId], [field]: value }
    }));
  };

  // Redistribute YouTube schedules starting from D+daysOffset
  const applySchedulePreset = (daysOffset: number) => {
    if (!job) return;
    const clipsToSchedule = job.clips.filter(c => selectedClipIds.has(c.clip_id));

    const startDay = new Date();
    startDay.setDate(startDay.getDate() + daysOffset);
    startDay.setHours(8, 0, 0, 0);
    let currentSlot = startDay;

    const updated: Record<string, ClipConfig> = { ...bulkConfigs };

    clipsToSchedule.forEach((clip) => {
      const config = updated[clip.clip_id];
      // Only reschedule YouTube-relevant clips
      if (!config || config.platform === "tiktok") return;

      const year = currentSlot.getFullYear();
      const month = String(currentSlot.getMonth() + 1).padStart(2, '0');
      const day = String(currentSlot.getDate()).padStart(2, '0');
      const hours = String(currentSlot.getHours()).padStart(2, '0');
      const mins = String(currentSlot.getMinutes()).padStart(2, '0');

      updated[clip.clip_id] = {
        ...config,
        date: `${year}-${month}-${day}`,
        time: `${hours}:${mins}`,
      };

      currentSlot = getNextUploadSlot(currentSlot);
    });

    setBulkConfigs(updated);
  };

  const handleBulkUploadConfirm = async () => {
    setShowBulkUploadSettings(false);
    if (!job || selectedClipIds.size === 0) return;
    uploadCancelledRef.current = false;
    setIsBulkUploading(true);

    // Check connections for needed platforms
    try {
      const clipsSelected = job.clips.filter(c => selectedClipIds.has(c.clip_id));
      const needsYouTube = clipsSelected.some(c => ["youtube", "both"].includes(bulkConfigs[c.clip_id]?.platform || "youtube"));
      const needsTikTok = clipsSelected.some(c => ["tiktok", "both"].includes(bulkConfigs[c.clip_id]?.platform));

      if (needsYouTube) {
        const accounts = await getSocialAccounts(authFetch);
        const connectedPlatforms = new Set(accounts.map((a) => a.platform));
        if (!connectedPlatforms.has("youtube")) {
          toast.error("YouTube account is not connected. Please connect it in Settings first.");
          setIsBulkUploading(false);
          return;
        }
      }

      if (needsTikTok) {
        // TikTok uses Playwright session, not OAuth — check separately
        const tikTokOk = tiktokConnected ?? await getTikTokConnected(engineFetch).catch(() => false);
        if (!tikTokOk) {
          toast.error("TikTok is not connected. Please connect it before uploading.");
          setIsBulkUploading(false);
          return;
        }
      }
    } catch {
      toast.error("Failed to verify social accounts connection.");
      setIsBulkUploading(false);
      return;
    }

    const clipsToUpload = job.clips.filter(c => selectedClipIds.has(c.clip_id));
    setBulkUploadProgress({ current: 0, total: clipsToUpload.length });

    let successCount = 0;
    let failCount = 0;

    for (let i = 0; i < clipsToUpload.length; i++) {
      if (uploadCancelledRef.current) break;

      const clip = clipsToUpload[i];
      setBulkUploadProgress({ current: i + 1, total: clipsToUpload.length });

      const config = bulkConfigs[clip.clip_id];
      let scheduledIso: string | undefined;

      if (config && config.date && config.time) {
        try {
          const [year, month, day] = config.date.split("-").map(Number);
          const [hour, minute] = config.time.split(":").map(Number);

          const localDate = new Date(year, month - 1, day, hour, minute);
          scheduledIso = localDate.toISOString();
        } catch (e) {
          toast.error(`Invalid date/time for Clip #${clip.clip_number}`);
          continue;
        }
      }

      const platform = config?.platform || "youtube";
      const platforms: ("youtube" | "tiktok")[] = platform === "both" ? ["youtube", "tiktok"] : [platform as "youtube" | "tiktok"];
      const basePayload = {
        clip_id: clip.clip_id,
        title: clip.title || `Clip #${clip.clip_number}`,
        description: clip.suggested_caption || "",
        tags: clip.tags || [],
        privacy: config?.privacy || "public",
      };

      const pollUpload = async (upload_id: string, label: string) => {
        while (true) {
          if (uploadCancelledRef.current) { failCount++; return; }
          await new Promise(r => setTimeout(r, 2000));
          try {
            const status = await getUploadStatus(engineFetch, upload_id);
            if (status.status === "completed") { successCount++; return; }
            if (status.status === "failed") {
              toast.error(`Failed to upload Clip #${clip.clip_number} (${label}): ${status.error || "Unknown Error"}`);
              failCount++;
              return;
            }
          } catch { /* ignore transient errors */ }
        }
      };

      try {
        // Start all platforms in parallel, then poll all in parallel
        const uploadJobs = await Promise.all(
          platforms.map(p =>
            startUpload(engineFetch, {
              ...basePayload,
              platform: p,
              scheduled_time: p === "youtube" ? scheduledIso : undefined,
            }).then(({ upload_id }) => ({ upload_id, platform: p }))
          )
        );
        await Promise.all(uploadJobs.map(({ upload_id, platform: p }) => pollUpload(upload_id, p)));
      } catch (err: any) {
        toast.error(`Failed to start upload for Clip #${clip.clip_number}: ${err.message || "Unknown error"}`);
        failCount++;
      }

      await new Promise(r => setTimeout(r, 1000));
    }

    setIsBulkUploading(false);
    setIsSelectionMode(false);
    setSelectedClipIds(new Set());

    if (!uploadCancelledRef.current) {
      if (failCount === 0) {
        setUploadSuccessCount(successCount);
        setShowUploadSuccess(true);
      } else {
        toast.warning(`Selesai dengan error: ${successCount} berhasil, ${failCount} gagal.`);
      }
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-[calc(100vh-3.5rem)] flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-primary" />
      </div>
    );
  }

  if (!job) return null;

  const sortedClips = [...job.clips].sort((a, b) => {
    if (sortBy === "viral_score") {
      return (b.viral_score ?? -1) - (a.viral_score ?? -1);
    }
    return a.clip_number - b.clip_number;
  });

  const uniqueStatuses = [...new Set(job.clips.map((c) => c.status))];
  const hasMultipleStatuses = uniqueStatuses.length > 1;

  const visibleClips = filterStatus === "all"
    ? sortedClips
    : sortedClips.filter((c) => c.status === filterStatus);

  const selectedIndex = selectedClip
    ? visibleClips.findIndex((c) => c.clip_id === selectedClip.clip_id)
    : -1;

  return (
    <>
      {selectedClip && (
        <ClipDetailModal
          clip={selectedClip}
          job={job}
          onClose={() => setSelectedClip(null)}
          onDownload={handleDownload}
          isDownloading={downloadingClip === selectedClip.clip_number}
          onPrev={selectedIndex > 0 ? () => setSelectedClip(visibleClips[selectedIndex - 1]) : undefined}
          onNext={selectedIndex < visibleClips.length - 1 ? () => setSelectedClip(visibleClips[selectedIndex + 1]) : undefined}
        />
      )}

      <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 space-y-6">
        {/* Header */}
        <div className="flex items-start gap-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => isBulkUploading ? setShowLeaveWarning(true) : router.push("/dashboard")}
            className="text-muted-foreground hover:text-foreground mt-0.5 -ml-2 cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4 mr-1" />
            Back
          </Button>
        </div>

        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
          <div className="min-w-0 flex-1">
            <h1 className="text-xl font-semibold wrap-break-word leading-snug">{job.video_name}</h1>
            <div className="flex flex-wrap items-center gap-4 mt-2 text-sm text-muted-foreground">
              {job.duration && (
                <span className="flex items-center gap-1.5">
                  <Clock className="w-3.5 h-3.5" />
                  {formatDuration(job.duration)}
                </span>
              )}
              <span className="flex items-center gap-1.5">
                <Film className="w-3.5 h-3.5" />
                {job.clips.length} {job.clips.length === 1 ? "clip" : "clips"}
              </span>
              {job.language && (
                <Badge variant="secondary" className="uppercase text-[10px]">
                  {job.language}
                </Badge>
              )}
              {job.source === "youtube" && job.source_url && (
                <a
                  href={job.source_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-primary hover:underline"
                >
                  YouTube source ↗
                </a>
              )}
            </div>
          </div>
          <div className="flex items-center sm:justify-end gap-2 shrink-0 flex-wrap mt-3 sm:mt-0">
            {job.clips.some((c) => c.viral_score != null) && (
              <Button
                variant="outline"
                size="sm"
                className="h-8 text-xs gap-1.5 cursor-pointer"
                onClick={() => setSortBy(sortBy === "clip_number" ? "viral_score" : "clip_number")}
              >
                <ArrowUpDown className="w-3 h-3" />
                {sortBy === "viral_score" ? "By order" : "By score"}
              </Button>
            )}
            {job.clips.length > 0 && (
              <>
                <Button
                  variant={isSelectionMode ? "secondary" : "outline"}
                  size="sm"
                  className="h-8 text-xs gap-1.5 cursor-pointer"
                  onClick={toggleSelectionMode}
                >
                  {isSelectionMode ? <X className="w-3 h-3" /> : <CheckSquare className="w-3 h-3" />}
                  {isSelectionMode ? "Cancel Select" : "Select Clips"}
                </Button>
                {isSelectionMode && (
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8 text-xs gap-1.5 cursor-pointer"
                    onClick={() => {
                      if (selectedClipIds.size === visibleClips.length && visibleClips.length > 0) {
                        setSelectedClipIds(new Set());
                      } else {
                        setSelectedClipIds(new Set(visibleClips.map((c) => c.clip_id)));
                      }
                    }}
                  >
                    {selectedClipIds.size === visibleClips.length && visibleClips.length > 0 ? (
                      <Square className="w-3 h-3" />
                    ) : (
                      <CheckSquare className="w-3 h-3" />
                    )}
                    {selectedClipIds.size === visibleClips.length && visibleClips.length > 0 ? "Deselect All" : "Select All"}
                  </Button>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  className="h-8 text-xs gap-1.5 cursor-pointer"
                  onClick={() => handleDownloadAll(visibleClips)}
                  disabled={isDownloadingAll || visibleClips.length === 0}
                >
                  {isDownloadingAll
                    ? <Loader2 className="w-3 h-3 animate-spin" />
                    : <Download className="w-3 h-3" />}
                  Download All
                </Button>
              </>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="h-8 text-xs gap-1.5 text-muted-foreground hover:text-destructive hover:bg-destructive/10 cursor-pointer"
              onClick={() => setShowDeleteDialog(true)}
            >
              <Trash2 className="w-3.5 h-3.5" />
              Delete
            </Button>
            <StatusBadge status={job.status} />
          </div>
        </div>

        {/* Filter by status — only shown if clips have multiple statuses */}
        {hasMultipleStatuses && (
          <div className="flex items-center gap-1.5 flex-wrap">
            {(["all", "completed", "failed"] as const).map((s) => (
              <button
                key={s}
                onClick={() => setFilterStatus(s)}
                className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                  filterStatus === s
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:text-foreground"
                }`}
              >
                {s === "all" ? `All (${job.clips.length})` : `${s.charAt(0).toUpperCase() + s.slice(1)} (${job.clips.filter((c) => c.status === s).length})`}
              </button>
            ))}
          </div>
        )}

        {/* Clips grid */}
        {job.clips.length === 0 ? (
          <Card className={`border-dashed ${job.status === "failed" || job.status === "aborted" ? "border-destructive/30 bg-destructive/5" : "border-primary/20 bg-primary/5"}`}>
            <CardContent className="py-14 text-center text-muted-foreground">
              <div className={`w-12 h-12 rounded-full flex items-center justify-center mx-auto mb-4 ${job.status === "failed" || job.status === "aborted" ? "bg-destructive/10" : "bg-primary/10"}`}>
                <Clapperboard className={`w-6 h-6 ${job.status === "failed" || job.status === "aborted" ? "text-destructive/70" : "text-primary/70"}`} />
              </div>
              <p className="text-sm font-medium text-foreground">No clips generated</p>
              {job.error ? (
                <p className="text-xs mt-2 max-w-sm mx-auto text-destructive/80 bg-destructive/10 rounded-lg px-3 py-2">
                  {job.error}
                </p>
              ) : (
                <p className="text-xs mt-1">Something may have gone wrong during processing.</p>
              )}
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 relative pb-16">
            {visibleClips.map((clip) => (
              <ClipCard
                key={clip.clip_id}
                clip={clip}
                jobId={job.id}
                onClick={isSelectionMode ? () => toggleClipSelection(clip.clip_id, !selectedClipIds.has(clip.clip_id)) : () => setSelectedClip(clip)}
                onDownload={handleDownload}
                isDownloading={downloadingClip === clip.clip_number}
                selectionMode={isSelectionMode}
                isSelected={selectedClipIds.has(clip.clip_id)}
                onSelectChange={(checked) => toggleClipSelection(clip.clip_id, checked)}
              />
            ))}
          </div>
        )}

        {/* Batch Action Bar */}
        {isSelectionMode && selectedClipIds.size > 0 && (() => {
          const hasUploadedSelected = Array.from(selectedClipIds).some(id => {
            const clip = job.clips.find(c => c.clip_id === id);
            return clip?.uploads?.some(u => u.platform === "youtube" && u.status === "completed");
          });

          return (
          <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 bg-background border border-border shadow-2xl rounded-full px-4 py-3 flex items-center gap-4 animate-in slide-in-from-bottom-10">
            <div className="flex items-center gap-2">
              <span className="w-6 h-6 rounded-full bg-primary/20 text-primary flex items-center justify-center text-xs font-semibold">
                {selectedClipIds.size}
              </span>
              <span className="text-sm font-medium text-muted-foreground mr-2">selected</span>
            </div>
            
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="ghost"
                onClick={toggleSelectionMode}
                disabled={isBulkUploading}
                className="rounded-full text-muted-foreground hover:text-foreground cursor-pointer"
              >
                Cancel
              </Button>
              <Button
                size="sm"
                onClick={handleBulkUploadClick}
                disabled={isBulkUploading}
                className="rounded-full shadow-md hover:shadow-lg transition-all cursor-pointer"
              >
                {isBulkUploading ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  Uploading {bulkUploadProgress.current} / {bulkUploadProgress.total}...
                </>
              ) : (
                <>
                  <Upload className="w-4 h-4 mr-2" />
                  {hasUploadedSelected ? "Re-upload to Social Media" : "Upload to Social Media"}
                </>
              )}
            </Button>
            </div>
          </div>
          );
        })()}
      </div>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={showDeleteDialog} onOpenChange={(val) => !isDeletingJob && setShowDeleteDialog(val)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. This will permanently delete this collection and remove
              all clips from our servers.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeletingJob}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => { e.preventDefault(); handleDeleteJob(); }}
              disabled={isDeletingJob}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isDeletingJob ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <Trash2 className="w-4 h-4 mr-2" />
              )}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Bulk Upload Settings Dialog */}
      <BulkUploadSettingsModal
        isOpen={showBulkUploadSettings}
        onOpenChange={setShowBulkUploadSettings}
        clips={job?.clips || []}
        selectedClipIds={selectedClipIds}
        configs={bulkConfigs}
        onConfigChange={updateClipConfig}
        onConfirm={handleBulkUploadConfirm}
        onApplySchedulePreset={applySchedulePreset}
        tiktokConnected={tiktokConnected}
        onConnectTikTok={handleConnectTikTok}
        onCancelTikTokSetup={handleCancelTikTokSetup}
        onDisconnectTikTok={handleDisconnectTikTok}
        isConnectingTikTok={isConnectingTikTok}
        cookiePaste={bulkCookiePaste}
        onCookiePasteChange={setBulkCookiePaste}
        onImportCookies={handleImportBulkCookies}
        isImportingCookies={isImportingBulkCookies}
      />

      {/* Leave Warning Dialog */}
      <AlertDialog open={showLeaveWarning} onOpenChange={setShowLeaveWarning}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Upload sedang berlangsung</AlertDialogTitle>
            <AlertDialogDescription>
              Anda tidak bisa meninggalkan halaman ini saat upload berlangsung. Mohon tunggu sebentar.
              Apakah yakin ingin keluar dan membatalkan upload?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setShowLeaveWarning(false)}>
              Tetap di Sini
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                uploadCancelledRef.current = true;
                setShowLeaveWarning(false);
                router.push("/dashboard");
              }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Keluar &amp; Batalkan Upload
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Upload Success Modal */}
      <AlertDialog open={showUploadSuccess} onOpenChange={setShowUploadSuccess}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Upload Berhasil! 🎉</AlertDialogTitle>
            <AlertDialogDescription>
              {uploadSuccessCount} clip berhasil dijadwalkan untuk diupload ke YouTube.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogAction onClick={() => setShowUploadSuccess(false)}>
              OK
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/auth-context";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
} from "@/components/ui/card";
import {
  ChevronLeft,
  ChevronRight,
  Clapperboard,
  Film,
  Loader2,
  Trash2,
  Video,
  Youtube,
} from "lucide-react";
import { toast } from "sonner";
import { ClipDetailModal } from "@/components/ClipDetailModal";
import { ClipCard } from "@/components/ClipCard";
import Link from "next/link";
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
import { useJob, PROCESSING_STATUSES } from "@/contexts/job-context";
import { downloadClip } from "@/lib/api";
import { UploadCard, type UploadSubmitPayload } from "../../../components/UploadCard";
import { ActiveJobBanner } from "../../../components/ActiveJobBanner";
import type { Clip, Job } from "@/types/types";

// ---- Constants ----

const JOBS_PER_PAGE = 12;

const STATUS_LABELS: Record<string, string> = {
  queued: "Waiting in queue…",
  downloading: "Downloading video…",
  transcribing: "Transcribing audio…",
  analyzing: "Analyzing content…",
  editing: "Editing clips…",
  finalizing: "Finalizing…",
  completed: "Completed",
  failed: "Failed",
};

// ---- Sub-components ----

function StatusBadge({ status }: { status: string }) {
  if (status === "completed") {
    return (
      <Badge
        variant="outline"
        className="bg-emerald-500/10 text-emerald-400 border-emerald-500/20 text-xs"
      >
        Completed
      </Badge>
    );
  }
  if (status === "failed" || status === "aborted") {
    return (
      <Badge variant="destructive" className="text-xs">
        {status === "aborted" ? "Aborted" : "Failed"}
      </Badge>
    );
  }
  if (PROCESSING_STATUSES.has(status)) {
    return (
      <Badge
        variant="outline"
        className="bg-amber-500/10 text-amber-400 border-amber-500/20 text-xs"
      >
        <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse mr-1.5 inline-block" />
        Processing
      </Badge>
    );
  }
  return (
    <Badge variant="secondary" className="text-xs capitalize">
      {status}
    </Badge>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// ---- Page ----

export default function DashboardPage() {
  const router = useRouter();
  const { engineFetch, user, getToken } = useAuth();
  const { activeJob, startJobPolling, refreshTrigger } = useJob();

  // Submission state (owned at page level — UploadCard reports up)
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Job history + pagination
  const [jobs, setJobs] = useState<Job[]>([]);
  const [isLoadingJobs, setIsLoadingJobs] = useState(true);
  const [page, setPage] = useState(0);
  const [totalJobs, setTotalJobs] = useState(0);

  // Top Clips state
  const [topClips, setTopClips] = useState<Clip[]>([]);
  const [isLoadingClips, setIsLoadingClips] = useState(true);
  const [selectedClip, setSelectedClip] = useState<Clip | null>(null);

  // Warn on browser/tab close while job is active
  useEffect(() => {
    if (!activeJob) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [activeJob]);

  const fetchJobs = useCallback(
    async (pageIndex = 0) => {
      try {
        const offset = pageIndex * JOBS_PER_PAGE;
        const res = await engineFetch(`/jobs?limit=${JOBS_PER_PAGE}&offset=${offset}`);
        if (!res.ok) return;
        const data = await res.json();
        setJobs(data.jobs ?? []);
        setTotalJobs(data.total ?? 0);
      } catch {
        // silently fail — job list is non-critical
      } finally {
        setIsLoadingJobs(false);
      }
    },
    [engineFetch],
  );

  const fetchTopClips = useCallback(async () => {
    try {
      const res = await engineFetch(`/jobs/clips/top?limit=8`);
      if (res.ok) setTopClips(await res.json());
    } catch {
      /* silently fail */
    } finally {
      setIsLoadingClips(false);
    }
  }, [engineFetch]);

  // On mount
  useEffect(() => {
    fetchJobs(0);
    fetchTopClips();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-refresh when a job completes/fails/aborts
  useEffect(() => {
    if (refreshTrigger === 0) return;
    fetchJobs(0);
    setPage(0);
    fetchTopClips();
  }, [refreshTrigger, fetchJobs, fetchTopClips]);

  // ---- Upload handler ----
  const handleUploadSubmit = async (payload: UploadSubmitPayload) => {
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      let jobId: string;

      if (payload.mode === "file") {
        if (!payload.file) return;
        const formData = new FormData();
        formData.append("file", payload.file);
        if (payload.jobName) formData.append("job_name", payload.jobName);
        const res = await engineFetch("/transcribe-async", {
          method: "POST",
          body: formData,
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          throw new Error(typeof err.detail === "string" ? err.detail : "Upload failed");
        }
        jobId = (await res.json()).job_id;
      } else {
        if (!payload.url) return;
        const res = await engineFetch("/transcribe-youtube", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            url: payload.url,
            ...(payload.jobName ? { job_name: payload.jobName } : {}),
          }),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          throw new Error(typeof err.detail === "string" ? err.detail : "Invalid YouTube URL");
        }
        jobId = (await res.json()).job_id;
      }

      toast.success("Job queued! Processing will start shortly.");
      startJobPolling(jobId);
      fetchJobs(0);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to submit");
    } finally {
      setIsSubmitting(false);
    }
  };

  // ---- Delete handler ----
  const [deletingJobId, setDeletingJobId] = useState<string | null>(null);
  const [jobToDelete, setJobToDelete] = useState<string | null>(null);

  const handleDeleteJob = (jobId: string) => {
    setJobToDelete(jobId);
  };

  const confirmDeleteJob = async () => {
    if (!jobToDelete) return;
    setDeletingJobId(jobToDelete);
    const id = jobToDelete;

    try {
      const res = await engineFetch(`/jobs/${id}`, { method: "DELETE" });
      if (!res.ok) throw new Error("Failed to delete");
      setJobs((prev) => prev.filter((j) => j.id !== id));
      setTopClips((prev) => prev.filter((c) => c.job_id !== id));
      setTotalJobs((prev) => prev - 1);
      toast.success("Collection deleted");
    } catch {
      toast.error("Failed to delete collection");
    } finally {
      setDeletingJobId(null);
      setJobToDelete(null);
    }
  };

  return (
    <>
      {selectedClip && (
        <ClipDetailModal
          clip={selectedClip}
          job={{ id: selectedClip.job_id } as Job}
          onClose={() => setSelectedClip(null)}
          onDownload={(clipNumber: number, title: string) => {
            downloadClip(selectedClip.job_id, clipNumber, title);
          }}
          isDownloading={false}
        />
      )}

      {/* Top Section Layout */}
      <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 space-y-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">

          {/* Left Col: Hero + Stats */}
          <div className="lg:col-span-1 flex flex-col gap-6">
            <div className="bg-linear-to-br from-primary/10 via-primary/5 to-transparent p-6 rounded-2xl border border-primary/10">
              <h1 className="text-2xl font-bold tracking-tight bg-linear-to-br from-foreground to-foreground/70 bg-clip-text text-transparent">
                Hey, {user?.full_name?.split(" ")[0] ?? user?.username} 👋
              </h1>
              <p className="text-muted-foreground mt-2 text-sm leading-relaxed">
                Ready to find your next viral moment? Drop a video and let AI do the heavy lifting.
              </p>
            </div>

            {/* Stats Row */}
            <div className="grid grid-cols-2 gap-3">
              <div className="bg-card border border-border/50 rounded-xl p-4 flex flex-col justify-center">
                <div className="flex items-center gap-2 text-muted-foreground mb-2">
                  <Video className="w-4 h-4 text-primary" />
                  <span className="text-xs font-medium uppercase tracking-wider">Collections</span>
                </div>
                <p className="text-2xl font-bold">{totalJobs}</p>
              </div>

              <div className="bg-card border border-border/50 rounded-xl p-4 flex flex-col justify-center">
                <div className="flex items-center gap-2 text-muted-foreground mb-2">
                  <Clapperboard className="w-4 h-4 text-primary" />
                  <span className="text-xs font-medium uppercase tracking-wider">Recent Clips</span>
                </div>
                <p className="text-2xl font-bold">
                  {jobs.reduce((acc, job) => acc + job.clips_count, 0)}
                </p>
              </div>
            </div>
          </div>

          {/* Right Col: Upload Card */}
          <div className="lg:col-span-2">
            <UploadCard isSubmitting={isSubmitting} onSubmit={handleUploadSubmit} />
          </div>
        </div>
      </div>

      {/* Active Job Banner */}
      <ActiveJobBanner />

      {/* Job History */}
      <div className="max-w-6xl mx-auto px-4 sm:px-6 pb-12 space-y-8">
        <div>
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-base font-semibold">
              Your Collections
              {totalJobs > 0 && (
                <span className="ml-2 text-xs font-normal text-muted-foreground">
                  ({totalJobs})
                </span>
              )}
            </h2>
            {totalJobs > JOBS_PER_PAGE && (
              <div className="flex items-center gap-1 text-sm text-muted-foreground">
                <span className="text-xs">
                  {page * JOBS_PER_PAGE + 1}–{Math.min((page + 1) * JOBS_PER_PAGE, totalJobs)} of {totalJobs}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  disabled={page === 0}
                  onClick={() => { const next = page - 1; setPage(next); fetchJobs(next); }}
                >
                  <ChevronLeft className="w-4 h-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  disabled={(page + 1) * JOBS_PER_PAGE >= totalJobs}
                  onClick={() => { const next = page + 1; setPage(next); fetchJobs(next); }}
                >
                  <ChevronRight className="w-4 h-4" />
                </Button>
              </div>
            )}
          </div>

          {isLoadingJobs ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="rounded-xl border border-border/50 p-4 space-y-3 animate-pulse">
                  <div className="h-4 bg-muted rounded w-3/4" />
                  <div className="h-3 bg-muted rounded w-1/2" />
                  <div className="flex items-center justify-between pt-1">
                    <div className="h-3 bg-muted rounded w-16" />
                    <div className="h-3 bg-muted rounded w-20" />
                  </div>
                </div>
              ))}
            </div>
          ) : jobs.length === 0 ? (
            <Card className="border-dashed border-primary/20 bg-primary/5">
              <CardContent className="py-14 text-center text-muted-foreground">
                <div className="w-12 h-12 rounded-full bg-primary/10 flex items-center justify-center mx-auto mb-4">
                  <Clapperboard className="w-6 h-6 text-primary/70" />
                </div>
                <p className="text-sm font-medium text-foreground">No collections yet</p>
                <p className="text-xs mt-1">
                  Upload a video or paste a YouTube link above to let AI find your best clips.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {jobs.map((job) => (
                <div key={job.id} className="relative group">
                  <Link href={`/jobs/${job.id}`} prefetch={true}>
                    <Card className="cursor-pointer hover:border-primary/30 transition-all hover:bg-card/90 flex flex-col h-full">
                    <div className="pb-2 pt-4 px-6">
                      <div className="flex items-start justify-between gap-2">
                        <h3 className="text-sm font-medium line-clamp-2 flex-1 leading-snug min-h-[40px]">
                          {job.video_name}
                        </h3>
                        <StatusBadge status={job.status} />
                      </div>
                    </div>
                    <CardContent className="pb-4 mt-auto">
                      <div className="flex items-center justify-between text-xs text-muted-foreground">
                        <div className="flex items-center gap-3">
                          <span
                            className="flex items-center gap-1"
                            title={job.source === "youtube" ? "YouTube Video" : "Uploaded Video File"}
                          >
                            {job.source === "youtube" ? (
                              <Youtube className="w-3.5 h-3.5" />
                            ) : (
                              <Film className="w-3.5 h-3.5" />
                            )}
                          </span>
                          <span className="flex items-center gap-1">
                            <Clapperboard className="w-3 h-3" />
                            {job.clips_count}
                          </span>
                        </div>
                        <span>{formatDate(job.created_at)}</span>
                      </div>
                    </CardContent>
                    </Card>
                  </Link>
                  <button
                    onClick={() => handleDeleteJob(job.id)}
                    disabled={deletingJobId === job.id}
                    className="absolute bottom-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded hover:text-destructive hover:bg-destructive/10 cursor-pointer disabled:cursor-not-allowed"
                    title="Delete collection"
                  >
                    {deletingJobId === job.id ? (
                      <Loader2 className="w-3 h-3 animate-spin" />
                    ) : (
                      <Trash2 className="w-3 h-3" />
                    )}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Top Clips Section */}
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold">Your Top Clips</h2>
          </div>

          {isLoadingClips ? (
            <div className="grid grid-cols-2 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <div
                  key={i}
                  className="rounded-xl border border-border/50 aspect-9/16 max-h-[400px] animate-pulse bg-muted/20"
                />
              ))}
            </div>
          ) : topClips.length === 0 ? (
            <Card className="border-dashed border-primary/20 bg-primary/5">
              <CardContent className="py-10 text-center text-muted-foreground">
                <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center mx-auto mb-3">
                  <Video className="w-5 h-5 text-primary/70" />
                </div>
                <p className="text-sm font-medium text-foreground">No clips yet</p>
                <p className="text-xs mt-1">
                  Clips you generate will appear here sorted by their viral score.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {topClips.map((clip) => (
                <ClipCard
                  key={clip.clip_id}
                  clip={clip}
                  jobId={clip.job_id}
                  onClick={() => setSelectedClip(clip)}
                  onDownload={(clipNumber, title) =>
                    downloadClip(clip.job_id, clipNumber, title)
                  }
                  isDownloading={false}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!jobToDelete} onOpenChange={(val) => !val && setJobToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. This will permanently delete your collection and remove
              your clips from our servers.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingJobId !== null}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => { e.preventDefault(); confirmDeleteJob(); }}
              disabled={deletingJobId !== null}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deletingJobId ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <Trash2 className="w-4 h-4 mr-2" />
              )}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

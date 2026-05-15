"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { useAuth } from "./auth-context";

// ---- Types ----
export interface ActiveJob {
  id: string;
  status: string;
  progress: number;
  phase?: string;
}

interface JobContextType {
  activeJob: ActiveJob | null;
  startJobPolling: (jobId: string) => void;
  abortJob: () => Promise<void>;
  clearActiveJob: () => void;
  isAborting: boolean;
  refreshTrigger: number;
}

const JobContext = createContext<JobContextType | null>(null);

export const ACTIVE_JOB_KEY = "coclip_active_job";
export const PROCESSING_STATUSES = new Set([
  "queued",
  "downloading",
  "transcribing",
  "analyzing",
  "editing",
  "finalizing",
  "processing",
]);

export function JobProvider({ children }: { children: ReactNode }) {
  const { engineFetch, isAuthenticated } = useAuth();
  const router = useRouter();

  // Optimistic restore from localStorage so banner appears instantly on refresh.
  // The resume effect below will validate the real status once auth is ready
  // and call clearActiveJob() if the job is no longer processing.
  const [activeJob, setActiveJob] = useState<ActiveJob | null>(() => {
    if (typeof window === "undefined") return null;
    const savedId = localStorage.getItem(ACTIVE_JOB_KEY);
    return savedId ? { id: savedId, status: "queued", progress: 0 } : null;
  });
  const [isAborting, setIsAborting] = useState(false);
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearActiveJob = useCallback(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = null;
    localStorage.removeItem(ACTIVE_JOB_KEY);
    setActiveJob(null);
  }, []);

  const startJobPolling = useCallback(
    (jobId: string) => {
      if (pollRef.current) clearInterval(pollRef.current);
      localStorage.setItem(ACTIVE_JOB_KEY, jobId);

      pollRef.current = setInterval(async () => {
        try {
          const res = await engineFetch(`/transcribe/status/${jobId}`);
          if (!res.ok) return;
          const data = await res.json();

          setActiveJob({
            id: jobId,
            status: data.status,
            progress: data.progress ?? 0,
            phase: data.current_phase,
          });

          if (data.status === "completed") {
            clearActiveJob();
            setRefreshTrigger((r) => r + 1);
            
            // Global Notification!
            toast.success(
              <div className="flex flex-col gap-2 relative">
                <span className="font-semibold text-sm">Clip is Ready! 🎉</span>
                <span className="text-xs text-muted-foreground">Your video has been processed successfully.</span>
                <button 
                  className="mt-1 bg-primary text-primary-foreground text-xs font-semibold py-1.5 px-3 rounded-md hover:bg-primary/90 transition-colors w-fit shadow-sm border border-primary/20"
                  onClick={() => router.push(`/jobs/${jobId}`)}
                >
                  Click to view
                </button>
              </div>,
              { duration: 15000, position: "bottom-right" }
            );

            // Optional: Play a tiny ping sound if we had an audio file, 
            // but we'll stick to a highly visible toast with a CTA for now.
            
          } else if (data.status === "failed" || data.status === "aborted") {
            clearActiveJob();
            setRefreshTrigger((r) => r + 1);
            toast.error(`Job ${data.status}: ${data.error ?? "Unknown error"}`);
          }
        } catch {
          // ignore transient errors
        }
      }, 3000); // Polling faster (3s) for better UX
    },
    [engineFetch, clearActiveJob, router]
  );

  // Resume polling on mount if active job exists
  useEffect(() => {
    if (!isAuthenticated) return;
    const savedId = localStorage.getItem(ACTIVE_JOB_KEY);
    if (!savedId) return;

    engineFetch(`/transcribe/status/${savedId}`)
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (!data) { clearActiveJob(); return; }
        if (PROCESSING_STATUSES.has(data.status)) {
          setActiveJob({ id: savedId, status: data.status, progress: data.progress ?? 0, phase: data.current_phase });
          startJobPolling(savedId);
        } else {
          clearActiveJob();
        }
      })
      .catch(() => clearActiveJob());
  }, [engineFetch, startJobPolling, clearActiveJob, isAuthenticated]);

  // Clean up
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  // Abort Handler
  const abortJob = async () => {
    if (!activeJob || isAborting) return;
    setIsAborting(true);
    try {
      const res = await engineFetch(`/transcribe/abort/${activeJob.id}`, {
        method: "POST",
      });
      if (!res.ok) throw new Error("Failed to abort");
      const data = await res.json();
      if (data.aborted) {
        toast.info("Job aborted");
        clearActiveJob();
        setRefreshTrigger((r) => r + 1);
        // Since we are global, we might need to tell dash to refetch Jobs, 
        // but it will just show as aborted next time they visit.
      }
    } catch {
      toast.error("Failed to abort job");
    } finally {
      setIsAborting(false);
    }
  };

  return (
    <JobContext.Provider value={{ activeJob, startJobPolling, abortJob, clearActiveJob, isAborting, refreshTrigger }}>
      {children}
    </JobContext.Provider>
  );
}

export const useJob = () => {
  const ctx = useContext(JobContext);
  if (!ctx) throw new Error("useJob must be used within JobProvider");
  return ctx;
};

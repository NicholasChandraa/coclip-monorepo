"use client";

import { useEffect, useState } from "react";
import { Loader2, Wand2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useJob } from "@/contexts/job-context";

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

const FUN_FACTS = [
  "Tip: Short hooks perform 40% better on TikTok.",
  "Did you know? Our AI analyzes silence and speech spikes.",
  "AI is scanning for the best moments to clip...",
  "Formatting subtitles for maximum retention...",
  "Tip: Vertical videos with captions have 80% higher completion rates.",
];

export function ActiveJobBanner() {
  const { activeJob, isAborting, abortJob } = useJob();
  const [factIndex, setFactIndex] = useState(0);

  const activeJobId = activeJob?.id ?? null;
  useEffect(() => {
    if (!activeJobId) return;
    const int = setInterval(() => {
      setFactIndex((prev) => (prev + 1) % FUN_FACTS.length);
    }, 10000);
    return () => clearInterval(int);
  }, [activeJobId]);

  if (!activeJob) return null;

  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 pb-8">
      <Card className="border-primary bg-primary/5 overflow-hidden relative shadow-md shadow-primary/5">
        {/* Animated background gradient */}
        <div className="absolute inset-0 bg-linear-to-r from-primary/5 via-primary/10 to-primary/5 animate-pulse" />

        <CardContent className="py-6 space-y-4 relative z-10">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
              {/* Pulsing Icon */}
              <div className="relative">
                <div className="absolute inset-0 bg-primary/20 rounded-full animate-ping" />
                <div className="relative w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center border border-primary/30">
                  <Wand2 className="w-5 h-5 text-primary animate-pulse" />
                </div>
              </div>
              <div>
                <h3 className="text-base font-semibold text-foreground flex items-center gap-2">
                  {STATUS_LABELS[activeJob.status] ?? "Processing your video…"}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5 max-w-sm overflow-hidden text-ellipsis whitespace-nowrap min-h-[16px]">
                  {activeJob.phase && (
                    <span className="font-mono text-primary/80 mr-2">[{activeJob.phase}]</span>
                  )}
                  <span className="transition-opacity duration-500">{FUN_FACTS[factIndex]}</span>
                </p>
              </div>
            </div>
            <div className="flex items-center gap-4">
              <div className="text-right">
                <span className="text-sm font-mono font-semibold text-primary block">
                  {activeJob.progress}%
                </span>
                <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Complete</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => abortJob()}
                disabled={isAborting}
                className="h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                title="Abort processing"
              >
                {isAborting ? <Loader2 className="w-4 h-4 animate-spin" /> : <X className="w-4 h-4" />}
              </Button>
            </div>
          </div>

          <div className="relative pt-2">
            <Progress
              value={activeJob.progress}
              className="h-2 bg-primary/10 [&>div]:bg-primary transition-all duration-1000 ease-in-out"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

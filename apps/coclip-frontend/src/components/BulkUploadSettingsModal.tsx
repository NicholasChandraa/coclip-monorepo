import { useState } from "react";
import { format } from "date-fns";
import { AlertCircle, CalendarIcon, Loader2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { Clip } from "../types/types";

export type PrivacyStatus = "public" | "unlisted" | "private";
export type UploadPlatform = "youtube" | "tiktok" | "both";

export interface ClipConfig {
  platform: UploadPlatform;
  privacy: PrivacyStatus;
  date: string;
  time: string;
}

interface BulkUploadSettingsModalProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  clips: Clip[];
  selectedClipIds: Set<string>;
  configs: Record<string, ClipConfig>;
  onConfigChange: (clipId: string, field: keyof ClipConfig, value: string) => void;
  onConfirm: () => void;
  onApplySchedulePreset: (daysOffset: number) => void;
  tiktokConnected: boolean | null;
  onConnectTikTok: () => Promise<void>;
  onCancelTikTokSetup: () => void;
  onDisconnectTikTok: () => Promise<void>;
  isConnectingTikTok: boolean;
  cookiePaste: string;
  onCookiePasteChange: (val: string) => void;
  onImportCookies: () => Promise<void>;
  isImportingCookies: boolean;
}

export function BulkUploadSettingsModal({
  isOpen,
  onOpenChange,
  clips,
  selectedClipIds,
  configs,
  onConfigChange,
  onConfirm,
  onApplySchedulePreset,
  tiktokConnected,
  onConnectTikTok,
  onCancelTikTokSetup,
  onDisconnectTikTok,
  isConnectingTikTok,
  cookiePaste,
  onCookiePasteChange,
  onImportCookies,
  isImportingCookies,
}: BulkUploadSettingsModalProps) {
  const [activePreset, setActivePreset] = useState<number>(1);

  const visibleClips = clips.filter((c) => selectedClipIds.has(c.clip_id));
  const hasTikTokClip = visibleClips.some((c) => ["tiktok", "both"].includes(configs[c.clip_id]?.platform));
  const hasYouTubeClip = visibleClips.some((c) => ["youtube", "both"].includes(configs[c.clip_id]?.platform));

  const SCHEDULE_PRESETS = [
    { label: "Tomorrow", sublabel: "D+1", days: 1 },
    { label: "Day After", sublabel: "D+2", days: 2 },
    { label: "In 3 Days", sublabel: "D+3", days: 3 },
    { label: "In 4 Days", sublabel: "D+4", days: 4 },
  ];

  const setAllPlatforms = (platform: UploadPlatform) => {
    visibleClips.forEach((clip) => {
      onConfigChange(clip.clip_id, "platform", platform);
    });
  };

  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Upload Clips Settings</DialogTitle>
          <DialogDescription>
            Configure privacy and scheduling for the {selectedClipIds.size} selected clip(s).
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className="h-[60vh] pr-4 py-4">
          <div className="flex items-center gap-2 mb-4 bg-muted/50 p-2.5 rounded-lg border border-border">
            <span className="text-xs font-medium text-muted-foreground mr-2">Set all platforms to:</span>
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs cursor-pointer"
              onClick={() => setAllPlatforms("youtube")}
            >
              YouTube
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs cursor-pointer"
              onClick={() => setAllPlatforms("tiktok")}
            >
              TikTok
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs cursor-pointer"
              onClick={() => setAllPlatforms("both")}
            >
              Both
            </Button>
          </div>

          {/* YouTube Schedule Spread Presets */}
          {hasYouTubeClip && (
            <div className="flex flex-col gap-2 mb-4 bg-muted/50 p-2.5 rounded-lg border border-border">
              <span className="text-xs font-medium text-muted-foreground">YouTube — Start scheduling from:</span>
              <div className="flex items-center gap-2 flex-wrap">
                {SCHEDULE_PRESETS.map((preset) => (
                  <button
                    key={preset.days}
                    onClick={() => {
                      onApplySchedulePreset(preset.days);
                      setActivePreset(preset.days);
                    }}
                    className={`flex flex-col items-center justify-center px-3 py-1.5 rounded-lg border transition-all cursor-pointer group ${
                      activePreset === preset.days
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-border bg-background/80 hover:border-primary/60 hover:bg-primary/5"
                    }`}
                  >
                    <span className={`text-xs font-semibold transition-colors ${
                      activePreset === preset.days ? "text-primary" : "group-hover:text-primary"
                    }`}>{preset.label}</span>
                    <span className="text-[10px] text-muted-foreground">{preset.sublabel}</span>
                  </button>
                ))}
                <span className="text-[11px] text-muted-foreground ml-1">× slots: 08·10·12·15·18·20·22</span>
              </div>
            </div>
          )}

          {/* TikTok connection banner */}
          {hasTikTokClip && tiktokConnected === false && (
            <div className="mb-4 rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-3 space-y-3">
              <div className="flex items-start gap-2">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-yellow-500" />
                <p className="text-xs font-medium text-yellow-500">TikTok not connected</p>
              </div>

              {/* Option 1: Browser login */}
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Option 1 — Login via Chrome:</p>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-xs cursor-pointer border-yellow-500/40 hover:bg-yellow-500/10"
                    onClick={onConnectTikTok}
                    disabled={isConnectingTikTok || isImportingCookies}
                  >
                    {isConnectingTikTok ? (
                      <><Loader2 className="mr-1.5 h-3 w-3 animate-spin" />Opening Chrome…</>
                    ) : "Open Chrome Login"}
                  </Button>
                  {isConnectingTikTok && (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 text-xs cursor-pointer text-muted-foreground hover:text-destructive"
                      onClick={onCancelTikTokSetup}
                    >
                      Cancel
                    </Button>
                  )}
                </div>
              </div>

              {/* Option 2: Paste cookies */}
              <div className="space-y-1.5">
                <p className="text-xs text-muted-foreground">Option 2 — Paste cookies JSON (Cookie-Editor):</p>
                <textarea
                  className="w-full h-16 text-xs rounded-md border border-border bg-background/80 px-2.5 py-1.5 resize-none focus:outline-none focus:ring-1 focus:ring-ring"
                  placeholder='[{"name":"sessionid","value":"..."}]'
                  value={cookiePaste}
                  onChange={(e) => onCookiePasteChange(e.target.value)}
                  disabled={isImportingCookies || isConnectingTikTok}
                />
                <Button
                  size="sm"
                  variant="outline"
                  className="h-7 text-xs cursor-pointer border-yellow-500/40 hover:bg-yellow-500/10"
                  onClick={onImportCookies}
                  disabled={!cookiePaste.trim() || isImportingCookies || isConnectingTikTok}
                >
                  {isImportingCookies ? (
                    <><Loader2 className="mr-1.5 h-3 w-3 animate-spin" />Importing…</>
                  ) : "Import Cookies"}
                </Button>
              </div>
            </div>
          )}
          {hasTikTokClip && tiktokConnected === true && (
            <div className="mb-4 flex items-center justify-between rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2.5">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-emerald-500" />
                <p className="text-xs text-emerald-400">TikTok session active</p>
              </div>
              <button
                className="text-[11px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                onClick={onDisconnectTikTok}
              >
                Switch account
              </button>
            </div>
          )}

          <div className="space-y-6">
            {visibleClips.map((clip) => {
              const config = configs[clip.clip_id];
              if (!config) return null;

              const dateObj = config.date ? new Date(config.date) : undefined;

              return (
                <div
                  key={clip.clip_id}
                  className="p-4 rounded-xl border border-border bg-card/50 space-y-4"
                >
                  <h4 className="font-semibold text-sm line-clamp-1">
                    {clip.title || `Clip #${clip.clip_number}`}
                  </h4>

                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                    {/* Platform Selector */}
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-muted-foreground">
                        Platform
                      </label>
                      <Select
                        value={config.platform || "youtube"}
                        onValueChange={(val: any) =>
                          onConfigChange(clip.clip_id, "platform", val)
                        }
                      >
                        <SelectTrigger className="w-full h-8 bg-background/80 text-sm cursor-pointer">
                          <SelectValue placeholder="Select platform" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="youtube">YouTube</SelectItem>
                          <SelectItem value="tiktok">TikTok</SelectItem>
                          <SelectItem value="both">YouTube + TikTok</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-muted-foreground">
                        Privacy Status
                      </label>
                      <Select
                        value={config.privacy}
                        onValueChange={(val: any) =>
                          onConfigChange(clip.clip_id, "privacy", val)
                        }
                      >
                        <SelectTrigger className="w-full h-8 bg-background/80 text-sm cursor-pointer">
                          <SelectValue placeholder="Select privacy status" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="public">Public</SelectItem>
                          <SelectItem value="unlisted">Unlisted</SelectItem>
                          <SelectItem value="private">Private</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {config.platform !== "tiktok" && (
                      <div className="space-y-1.5 sm:col-span-3">
                        <label className="text-xs font-medium text-muted-foreground flex items-center justify-between">
                          {config.platform === "both" ? "Schedule YouTube Publish Time" : "Schedule Publish Time"}
                          {config.date && (
                            <button
                              onClick={() => {
                                onConfigChange(clip.clip_id, "date", "");
                                onConfigChange(clip.clip_id, "time", "");
                              }}
                              className="text-[10px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
                            >
                              Clear
                            </button>
                          )}
                        </label>
                        <div className="flex flex-col sm:flex-row items-center gap-2">
                          <Popover>
                            <PopoverTrigger asChild>
                              <Button
                                variant={"outline"}
                                className={cn(
                                  "w-full sm:flex-1 h-8 justify-start text-left font-normal bg-background/80 hover:bg-background/90 cursor-pointer",
                                  !config.date && "text-muted-foreground"
                                )}
                              >
                                <CalendarIcon className="mr-2 h-4 w-4" />
                                {config.date ? (
                                  format(dateObj!, "PPP")
                                ) : (
                                  <span>Pick a date</span>
                                )}
                              </Button>
                            </PopoverTrigger>
                            <PopoverContent className="w-auto p-0 cursor-pointer" align="start">
                              <Calendar
                                mode="single"
                                selected={dateObj}
                                onSelect={(day) => {
                                  if (day) {
                                    // Preserve local date visually avoiding timezone shift from Date-picker
                                    const localString = `${day.getFullYear()}-${String(
                                      day.getMonth() + 1
                                    ).padStart(2, "0")}-${String(
                                      day.getDate()
                                    ).padStart(2, "0")}`;
                                    onConfigChange(clip.clip_id, "date", localString);
                                  } else {
                                    onConfigChange(clip.clip_id, "date", "");
                                  }
                                }}
                                disabled={(date) =>
                                  date < new Date(new Date().setHours(0, 0, 0, 0))
                                }
                                initialFocus
                              />
                            </PopoverContent>
                          </Popover>

                          <div className="relative w-full sm:w-28">
                            <Input
                              type="time"
                              className="w-full h-8 bg-background/80 px-3 cursor-pointer scheme-dark [&::-webkit-calendar-picker-indicator]:absolute [&::-webkit-calendar-picker-indicator]:inset-0 [&::-webkit-calendar-picker-indicator]:w-full [&::-webkit-calendar-picker-indicator]:h-full [&::-webkit-calendar-picker-indicator]:opacity-0 [&::-webkit-calendar-picker-indicator]:cursor-pointer"
                              value={config.time}
                              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                                onConfigChange(clip.clip_id, "time", e.target.value)
                              }
                            />
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} className="cursor-pointer">
            Cancel
          </Button>
          <Button
            onClick={onConfirm}
            disabled={hasTikTokClip && tiktokConnected === false}
            className="cursor-pointer"
          >
            Start Upload
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

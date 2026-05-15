import { Clock, Download, Loader2, Star, Youtube } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { ClipThumbnail } from "./ClipThumbnail";
import { formatDuration } from "@/lib/utils";
import type { Clip } from "@/types/types";



interface ClipCardProps {
  clip: Clip;
  jobId: string;
  onClick: () => void;
  onDownload: (clipNumber: number, title: string) => void;
  isDownloading: boolean;
  selectionMode?: boolean;
  isSelected?: boolean;
  onSelectChange?: (checked: boolean) => void;
}

export function ClipCard({
  clip,
  jobId,
  onClick,
  onDownload,
  isDownloading,
  selectionMode,
  isSelected,
  onSelectChange,
}: ClipCardProps) {
  return (
    <Card
      className={`overflow-hidden transition-all cursor-pointer group flex flex-col h-full ${
        isSelected
          ? "border-primary bg-primary/5 ring-1 ring-primary"
          : "border-border/50 hover:border-primary/30"
      }`}
      onClick={selectionMode && onSelectChange ? () => onSelectChange(!isSelected) : onClick}
    >
      <div className="relative">
        <ClipThumbnail jobId={jobId} clipNumber={clip.clip_number} />

        <div className="absolute top-2 left-2 z-10 pointer-events-none flex gap-1 items-center">
          {selectionMode && onSelectChange && (
            <div className="pointer-events-auto" onClick={(e) => e.stopPropagation()}>
              <Checkbox
                checked={isSelected}
                onCheckedChange={(checked: boolean | "indeterminate") => onSelectChange(checked === true)}
                className="w-5 h-5 bg-black/60 border-white/50 data-[state=checked]:bg-primary data-[state=checked]:border-primary"
              />
            </div>
          )}
          <Badge className="text-xs bg-black/60 text-white border-0">
            #{clip.clip_number}
          </Badge>
        </div>

        <div className="absolute top-2 right-2 z-10 pointer-events-none flex flex-col items-end gap-1">
          {clip.viral_score != null && (
            <Badge className="text-xs bg-amber-500/20 text-amber-300 border-amber-500/30">
              <Star className="w-2.5 h-2.5 mr-1 fill-current" />
              {clip.viral_score.toFixed(1)}
            </Badge>
          )}
        </div>
      </div>

      <CardHeader className="pb-2 pt-2.5 px-3">
        <CardTitle className="text-xs line-clamp-2 font-medium leading-normal min-h-[38px]">
          {clip.title}
        </CardTitle>
      </CardHeader>

      <CardContent className="px-3 pb-3 space-y-2 flex flex-col flex-1">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <Clock className="w-3 h-3" />
            {formatDuration(clip.duration ?? (clip.end ? clip.end - clip.start : 0))}
          </span>
          {clip.file_size && (
            <span>{(clip.file_size / 1024 / 1024).toFixed(1)} MB</span>
          )}
        </div>

        {clip.suggested_caption && (
          <p className="text-[11px] text-muted-foreground line-clamp-2 italic leading-relaxed">
            &ldquo;{clip.suggested_caption}&rdquo;
          </p>
        )}

        {clip.tags && clip.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {clip.tags.slice(0, 2).map((tag: string) => (
              <span key={tag} className="text-[10px] text-primary/70 leading-none">
                {tag}
              </span>
            ))}
            {clip.tags.length > 2 && (
              <span className="text-[10px] text-muted-foreground leading-none">
                +{clip.tags.length - 2}
              </span>
            )}
          </div>
        )}

        {clip.uploads?.filter(u => u.platform === "youtube" && u.status === "completed").map((upload, idx) => (
          <div key={idx} className="mt-1">
            <a 
              href={upload.url} 
              target="_blank" 
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()} 
              className="inline-flex items-center gap-1.5 text-[11px] font-medium text-red-500 hover:text-red-400 hover:underline transition-colors"
            >
              <Youtube className="w-3 h-3" />
              View on YouTube
            </a>
          </div>
        ))}

        <div className="mt-auto pt-1">
          <Button
            size="sm"
            variant="outline"
            className="w-full h-7 text-xs cursor-pointer"
            onClick={(e) => {
              e.stopPropagation();
              onDownload(clip.clip_number, clip.title);
            }}
            disabled={isDownloading}
          >
            {isDownloading ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : (
              <><Download className="w-3 h-3 mr-1" />Download</>
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

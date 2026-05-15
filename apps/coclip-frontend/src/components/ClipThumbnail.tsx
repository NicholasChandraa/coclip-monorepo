import { useState, useRef } from "react";
import { Clapperboard, Play } from "lucide-react";
import { getClipUrl } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";

export function ClipThumbnail({ jobId, clipNumber }: { jobId: string; clipNumber: number }) {
  const [hasError, setHasError] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const { getToken } = useAuth();
  
  // Menambahkan #t=0.1 pada source URL akan memaksa browser mengunduh detik ke 0.1 saja dari video
  // dan menjadikannya sebagai *thumbnail gambar statis*. Hal ini menghemat bandwidth.
  const src = getClipUrl(jobId, clipNumber, getToken());

  if (hasError) {
    return (
      <div className="aspect-9/16 bg-muted/40 flex flex-col items-center justify-center gap-2 text-muted-foreground/50">
        <Clapperboard className="w-8 h-8" />
        <span className="text-xs">Preview unavailable</span>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="aspect-9/16 bg-black relative overflow-hidden group"
    >
      <video
        ref={videoRef}
        src={`${src}#t=0.1`}
        className="w-full h-full object-contain pointer-events-none"
        preload="metadata"
        playsInline
        muted
        crossOrigin="use-credentials"
        onError={() => setHasError(true)}
      />

      {/* Play icon overlay */}
      <div className="absolute inset-0 flex flex-col items-center justify-center group-hover:bg-black/20 transition-colors duration-300">
        <div className="w-10 h-10 rounded-full bg-black/50 backdrop-blur-sm flex items-center justify-center border border-white/20 transition-transform group-hover:scale-110">
          <Play className="w-4 h-4 text-white fill-white ml-0.5" />
        </div>
      </div>
    </div>
  );
}

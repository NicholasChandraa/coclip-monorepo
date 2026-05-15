export interface Clip {
  clip_id: string;
  clip_number: number;
  job_id: string;
  start: number;
  end: number;
  duration: number;
  title: string;
  viral_score: number | null;
  suggested_caption: string | null;
  hook_text: string | null;
  transcript_text: string | null;
  reasoning: string | null;
  tags: string[] | null;
  has_subtitles: boolean;
  status: string;
  file_path: string | null;
  file_size: number | null;
  uploads?: { platform: string; status: string; url?: string }[];
}

export interface Job {
  id: string;
  video_name: string;
  language: string | null;
  duration: number | null;
  total_segments: number | null;
  status: string;
  error: string | null;
  source: string;
  source_url: string | null;
  clips_count: number;
  created_at: string;
  completed_at: string | null;
  clips: Clip[];
}

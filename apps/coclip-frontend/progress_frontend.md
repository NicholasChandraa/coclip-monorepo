# Frontend Progress — CoClip

Last updated: 2026-02-20

---

## Stack

- Next.js 16 + React 19 + TypeScript
- Tailwind CSS v4
- shadcn/ui (New York style, Zinc base)
- Dark violet theme (`oklch(0.60 0.25 280)`)
- Sonner (toast notifications)

---

## Sudah Selesai ✅

### Auth Layer
- [x] `src/lib/api.ts` — konstanta `AUTH_BASE` (`:8001`) dan `ENGINE_BASE` (`:8000/api/v1`)
- [x] `src/contexts/auth-context.tsx` — full auth context
  - `login`, `register`, `logout`
  - `tokenRef` (useRef) — akses token tanpa stale closure
  - `refreshAccessToken` — silent refresh via HttpOnly cookie
  - `engineFetch` — fetch wrapper ke FastAPI, auto-inject Bearer token, auto-refresh on 401
  - Init on mount: cek localStorage → validasi `/auth/me` → fallback silent refresh via cookie
- [x] `frontend/.env.local` — `NEXT_PUBLIC_AUTH_URL`, `NEXT_PUBLIC_ENGINE_URL`

### Halaman Auth
- [x] `src/app/(auth)/login/page.tsx`
  - Form username + password
  - Dark card dengan violet glow background
  - Link ke register
- [x] `src/app/(auth)/register/page.tsx`
  - Form full name, username, email, password
  - Auto-login setelah register berhasil
  - Link ke login

### App Shell
- [x] `src/app/page.tsx` — root redirect: `/dashboard` kalau logged in, `/login` kalau tidak
- [x] `src/app/(app)/layout.tsx` — auth guard + sticky navbar
  - Auth guard: redirect ke `/login` jika tidak ada user
  - Navbar: logo CoClip, username, tombol Sign Out

### Dashboard (`/dashboard`)
- [x] Hero greeting dengan nama user
- [x] Upload card dengan toggle File / YouTube URL
  - Mode File: drag-and-drop zone, click to browse, tampilkan nama & ukuran file
  - Mode YouTube: input URL, submit dengan Enter
  - Accept: `video/*, audio/*`
- [x] Submit → `POST /transcribe-async` atau `POST /transcribe-youtube`
- [x] Active job progress card
  - Polling setiap 3 detik ke `/transcribe/status/{jobId}`
  - Progress bar (`Progress` component)
  - Label fase: Waiting, Downloading, Transcribing, Analyzing, Editing, Finalizing
  - Toast "Clips are ready!" + action "View clips" saat selesai
  - Toast error jika failed/aborted
  - Auto-hide progress card 3-4 detik setelah selesai
- [x] Job history grid (max 20 jobs)
  - Card per job: nama video, status badge, jumlah clips, tanggal
  - Click card → navigate ke `/jobs/{id}`
  - Status badge: emerald (completed), amber pulse (processing), destructive (failed/aborted)
  - Empty state dengan ilustrasi

### Job Detail (`/jobs/[id]`)
- [x] Fetch job detail dari `GET /jobs/{id}`
- [x] Header: nama video, durasi, jumlah clips, bahasa, status badge
- [x] Tombol back ke dashboard
- [x] Grid clip cards (1/2/3 kolom responsif)
  - Placeholder thumbnail 9:16 ratio
  - Badge nomor clip (kiri atas)
  - Badge viral score amber (kanan atas) — jika ada
  - Badge durasi (kanan bawah)
  - Judul clip
  - Time range (start – end) + ukuran file
  - Suggested caption (jika ada)
  - Tombol Download MP4 → blob download via `engineFetch`
- [x] Empty state jika tidak ada clips

---

## Belum Ada ❌

### Video Preview
- [ ] Player video inline di clip card (sebelum download)
  - Perlu streaming endpoint di FastAPI atau serve static file
  - Saat ini hanya placeholder icon, tidak bisa preview

### Resiliensi Polling
- [ ] Polling tidak resume setelah page refresh
  - Jika user refresh saat job sedang processing, progress bar hilang
  - Job tetap jalan di backend, tapi frontend tidak tahu
  - Fix: cek job aktif saat mount dashboard (cek `jobs` yang status-nya processing, lalu start polling)

### Halaman Profil
- [ ] `src/app/(app)/profile/page.tsx`
  - Update nama, email (`PUT /auth/profile`)
  - Ganti password (`POST /auth/change-password`)
  - Auth-service sudah punya endpoint-nya

### Fitur Lain yang Bisa Ditambah
- [ ] Pagination / infinite scroll di job history (sekarang hard-limit 20)
- [ ] Filter/sort job history (by status, by date)
- [ ] Konfirmasi sebelum navigasi saat upload sedang berjalan
- [ ] Halaman 404 custom
- [ ] Empty state yang lebih informatif di job detail (link langsung ke dashboard)
- [ ] Abort job button di active job card

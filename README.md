# Coclip Monorepo

Coclip is a multi-service app for AI-assisted video/audio clipping, transcription, speaker-aware editing, and social upload workflows.

## Structure

```text
apps/coclip-frontend      Next.js user interface
services/auth-service     Go auth/API service
services/engine           Python FastAPI processing engine
infra/docker              Local PostgreSQL and Redis helpers
scripts                   Shared maintenance scripts
```

This repo was migrated without old Git history, local secrets, dependency folders, model folders, runtime storage, cookies, rendered clips, and test media outputs.

## Prerequisites

- Node.js 22+
- npm for the frontend package
- Go 1.25+
- Python 3.11 for the engine
- uv for Python environment management
- Docker for local PostgreSQL and Redis
- FFmpeg for the engine video pipeline

## Local Setup

Create local env files from examples:

```bash
cp apps/coclip-frontend/.env.example apps/coclip-frontend/.env
cp services/auth-service/.env.example services/auth-service/.env
cp services/engine/.env.example services/engine/.env
```

Use the same `JWT_SECRET_KEY` in `services/auth-service/.env` and `services/engine/.env`.
Use the same `SERVICE_TOKEN` in auth-service and `AUTH_SERVICE_TOKEN` in engine.

Start local dependencies:

```bash
docker compose -f infra/docker/docker-compose.yml up -d
```

If local PostgreSQL already uses port `5432`, start only the dependencies that do not conflict:

```bash
docker compose -f infra/docker/docker-compose.yml up -d postgres-engine redis
```

Then set `DB_PORT=5434` in `services/auth-service/.env` and create `auth_db` in the `postgres-engine` container:

```bash
docker exec docker-postgres-engine-1 createdb -U postgres auth_db
```

## Run Services

Frontend:

```bash
cd apps/coclip-frontend
npm install
npm run dev -- --hostname 127.0.0.1 --port 3000
```

Auth service:

```bash
cd services/auth-service
go run cmd/server/main.go
```

Engine:

```bash
cd services/engine
uv run --python 3.11 python -m compileall app main.py
uv run --python 3.11 python main.py
```

For an existing manually managed engine environment, use that environment's Python 3.11 interpreter instead of running `uv sync` blindly. The engine dependency set is GPU/media-heavy and may need local CUDA, PyTorch, FFmpeg, and model setup.

## Local URLs

```text
Frontend      http://127.0.0.1:3000
Auth service  http://127.0.0.1:8001/health
Engine        http://127.0.0.1:8000/
```

For local browser testing, keep frontend, auth, and engine hosts consistent. If the frontend runs on `127.0.0.1`, include `http://127.0.0.1:3000` in `CORS_ALLOWED_ORIGINS`.

## Notes

- `services/engine/clips/`, `services/engine/temp/`, `services/engine/logs/`, model folders, cookies, and browser sessions are runtime data and intentionally ignored.
- `.env` files are local-only and ignored. Commit only `.env.example` files.
- `services/engine/music/sound-effect-1.mp3` was not migrated automatically because it is a binary asset and needs manual source/licensing review.

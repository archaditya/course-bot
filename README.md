# AI Course Assistant

Architecture and decisions: see `docs/` (start at `docs/README.md`).
UI visual direction / build prompt: see `AI_Course_Assistant_UI_Prompt.md`.

This is a structural scaffold only — folders and placeholder READMEs marking
what belongs where, matching the architecture in `docs/`. No implementation
code has been added yet.

## Layout

- `apps/api` — Go API Gateway
- `apps/worker` — Go background workers (owns the indexing pipeline + all writes)
- `apps/ai-service` — Python AI Service (stateless compute only)
- `internal/domain` — entities + interfaces, no framework deps
- `internal/application` — use cases / orchestration
- `internal/infrastructure` — concrete DB/queue/storage/provider implementations
- `internal/interfaces` — HTTP/gRPC/CLI entrypoints
- `frontend` — Next.js app
- `migrations` — Postgres migrations
- `scripts` — one-off ops scripts
- `docs` — the full architecture doc set + ADRs

# 08 — Security Model

Authentication (who you are) is covered in [01-product-requirements.md](./01-product-requirements.md#authentication). This doc covers everything else: what you can access, and how the system protects itself.

## Workspace Isolation

Every query-bearing table (`Projects`, `Courses`, `Chunks`, `Chats`, `Messages`) is scoped by `workspace_id` (or transitively via `project_id` → `workspace_id`). Every data-access function in `application/` takes the requesting user's workspace context as a required argument — there is no code path that queries these tables without a workspace filter. This is what prevents one user's course content from ever being retrievable by another user's chat, even in a bug.

## Authorization

- Role check happens in `application/` use cases, not in `interfaces/` handlers — so authorization logic is testable independent of HTTP.
- MVP has a single implicit role (owner); the interface is written to support RBAC (Section 3.1, future) without a rewrite — an `authorize(user, action, resource)` call is already the shape used everywhere, it just always returns true for an owner today.

## R2 Signed URLs

The frontend never talks to R2 directly. All uploads and downloads go through short-lived, single-use signed URLs issued by the Go API:
- Upload: API validates the request, issues a signed PUT URL scoped to one object key, expires in minutes.
- Download (e.g. serving a video for playback): API issues a signed GET URL scoped to that object, expires quickly, reissued per session as needed.

## JWT Rotation

- Access tokens: short-lived (minutes), used for all authenticated API calls.
- Refresh tokens: long-lived, but rotated on every use — each refresh invalidates the previous refresh token, so a leaked refresh token has a bounded window of use before rotation cuts it off.
- Refresh tokens are stored hashed in Postgres, never in plaintext.

## Secrets

- All secrets (API keys, DB credentials, signing keys) load from environment variables via the typed `config/` package — see [09-deployment.md](./09-deployment.md#configuration-strategy).
- No secret is ever logged, including in error messages — error handling in [09-deployment.md](./09-deployment.md#error-handling) explicitly redacts known secret-shaped fields before logging.

## Rate Limits

Applied at the Go API layer, backed by Redis:
- Per-user limits on chat message frequency (protects LLM cost from abuse).
- Per-user limits on upload frequency/size (protects storage and worker capacity).
- Stricter limits on unauthenticated endpoints (login attempts).

## File Validation

Every upload is validated before it's stored or queued:
- MIME type and extension must match a supported type.
- Size limits enforced before the file is fully accepted.
- Malformed files (e.g. corrupt SRT) fail fast in the Parser Worker and produce a `FAILED` job (see [04-indexing-pipeline.md](./04-indexing-pipeline.md#event-contracts)) with a plain-language reason — not a silent drop.

## Virus Scan (Future)

Not in MVP scope, but reserved: uploaded files would be scanned before being made available for parsing, likely as an additional stage between "Store Raw" and "Push Job to Redis Stream" in the indexing pipeline, so it slots in without restructuring the pipeline.

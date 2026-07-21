# AI Course Assistant — Go API

Scope of this drop: the Go API only (auth + project/course CRUD), with a
real Postgres schema and self-running migrations. Architecture docs in
`docs/`; this implements pieces of Sprint 1-2 from
`docs/09-deployment.md#milestones--sprint-plan-suggested`.

Module path is `course-assistant` (local-only) — rename in `go.mod` once
this lives in a real repo: `go mod edit -module github.com/<you>/course-assistant`.

## Run it

```bash
cp .env.example .env             # edit POSTGRES_URL etc. if not using the compose defaults
docker compose up -d postgres    # or point POSTGRES_URL at any Postgres 13+
set -a && source .env && set +a
go run cmd/api/main.go
```

That's it — no separate migrate step. On every startup the API connects to
Postgres and applies any `migrations/*.up.sql` file not yet recorded in
`schema_migrations`, then starts serving. A brand-new empty database is
fully bootstrapped by running the command once.

Required env vars (see `.env.example`): `POSTGRES_URL`, `REDIS_URL`,
`JWT_SIGNING_KEY`. Missing any of them fails the process immediately with a
clear message instead of failing later on the first request.

Verified end-to-end against a real local Postgres 16 instance: signup,
duplicate-email rejection, login, protected routes returning 401 without a
token, project/course CRUD, cursor pagination (multi-page), refresh-token
rotation (old token correctly rejected after use), and cross-workspace
isolation (a second user gets 404s and empty lists for the first user's
data, never someone else's rows).

## What's here

```
cmd/api/main.go                        entrypoint: config → DB → migrate → wire → serve
internal/config/                       typed, fail-fast env config
internal/domain/entities/              User, Workspace, Project, Course (+lifecycle),
                                        Module, Lesson, Document, Chunk, Conversation,
                                        Message, Citation, Job (+lifecycle), RefreshToken, AuditLog
internal/domain/repository/            persistence interfaces, workspace-scoped by construction
internal/domain/provider/              LLM/Embedding/Reranker/Guardrail/Parser/Queue/
                                        VectorStore/ObjectStore interfaces (not yet implemented —
                                        needed once the indexing/query pipeline is built)
internal/application/auth/             signup, login, refresh (JWT rotation)
internal/application/project/          project CRUD use cases
internal/application/course/           course CRUD use cases
internal/infrastructure/security/      PBKDF2 password hashing + HS256 JWT (stdlib only —
                                        see note below on why not x/crypto)
internal/infrastructure/postgres/      repository implementations + migration runner
internal/interfaces/http/              handlers, auth middleware, router
migrations/                            000001_init_schema — full schema, up+down
docs/                                  the architecture doc set (for reference)
```

### A note on `internal/infrastructure/security`

`golang.org/x/crypto` (the usual home for bcrypt/scrypt) couldn't be
fetched in the sandbox this was built in — its module path needs an HTTP
redirect lookup against `golang.org`, which wasn't reachable — so password
hashing is hand-rolled PBKDF2-HMAC-SHA256 (RFC 2898, 210k iterations, the
2023 OWASP-recommended minimum) and JWT is a minimal HS256 implementation,
both stdlib-only (`crypto/hmac`, `crypto/sha256`, `crypto/rand`). Both are
tested (constant-time verify, tamper/expiry rejection). If `golang.org` is
reachable wherever you build this, swapping to `x/crypto/bcrypt` is a
one-file change in `password.go` — nothing outside that package would
need to change.

## Endpoints implemented

```
GET   /healthz

POST  /auth/signup                          {email, password}
POST  /auth/login                           {email, password} -> access_token, refresh_token
POST  /auth/refresh                         {refresh_token}    -> rotated pair

POST  /projects                              (auth required)  {name}
GET   /projects?cursor=&limit=
GET   /projects/{id}
PATCH /projects/{id}                         {name}
DELETE /projects/{id}

POST  /projects/{project_id}/courses         {title}
GET   /projects/{project_id}/courses?cursor=&limit=
GET   /courses/{id}
PATCH /courses/{id}                          {title}
DELETE /courses/{id}
```

All protected routes require `Authorization: Bearer <access_token>`. Every
project/course lookup is scoped to the caller's workspace — courses via a
`JOIN projects` transitively, per `docs/08-security.md#workspace-isolation` —
so IDs belonging to another workspace 404 rather than leak.

## Not in this drop

- Google OAuth signup (email/password only for now)
- Upload/indexing pipeline, Go workers, Python AI service, frontend — see
  `docs/` for the full plan; those are separate implementation passes.
- Job status endpoint (`GET /courses/{id}/status`'s job list) — deferred
  until `JobRepository` gets a Postgres implementation.

# 07 — Storage

```
R2 (Object Storage)
├── raw/            → original uploaded files, immutable
└── processed/      → normalized documents, generated artifacts

Postgres (Relational)
└── metadata        → users, projects, courses, chunks (pointers), chats, messages, jobs

Redis
└── queue           → job streams for the worker pipeline
└── cache           → hot session data, rate limiting

Qdrant (Vector Store)
└── vectors         → chunk embeddings + minimal payload (course_id, timestamp, chunk_id)
```

## Rule of Thumb

- **Postgres** is the source of truth for *structure and ownership* — see [03-domain-model.md](./03-domain-model.md).
- **R2** is the source of truth for *bytes* — raw files are never mutated, only ever re-read.
- **Qdrant** is a derived, rebuildable index — it should always be safe to wipe and re-embed from Postgres + R2. See [ADR-002](./decisions/ADR-002-qdrant.md) for why Qdrant specifically.

## Write Ownership

Only Go Workers and the Go API write to storage — see [Component Ownership](./02-system-architecture.md#component-ownership). The Python AI Service never writes to any of these stores directly; it returns results to the caller, who writes them.

## Access Patterns

| Store | Read by | Written by |
|---|---|---|
| Postgres | Go API, Go Workers | Go API, Go Workers |
| R2 | Go API (signed URLs — see [08-security.md](./08-security.md#r2-signed-urls)), Go Workers | Go API (upload), Go Workers (processed artifacts) |
| Redis (queue) | Go Workers | Go API, Go Workers |
| Qdrant | Python AI Service (query-time retrieval, via Go proxy) | Go Workers (indexing-time writes only) |

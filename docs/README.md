# AI Course Assistant — Architecture Docs

This replaces the single monolithic SDD with a proper `/docs` set. Same architectural decisions, split so any one part can grow without the whole file becoming unreadable — and so a new contributor can be pointed at exactly the one doc they need.

## How to read this

| # | Doc | Read this if you're... |
|---|---|---|
| 00 | [Vision](./00-vision.md) | new to the project, need the "why" |
| 01 | [Product Requirements](./01-product-requirements.md) | building UX, deciding what a feature does |
| 02 | [System Architecture](./02-system-architecture.md) | deciding where new code should live |
| 03 | [Domain Model](./03-domain-model.md) | writing migrations, naming things |
| 04 | [Indexing Pipeline](./04-indexing-pipeline.md) | touching upload → chunk → embed |
| 05 | [Query Pipeline](./05-query-pipeline.md) | touching chat / retrieval / generation |
| 06 | [AI Service](./06-ai-service.md) | working inside the Python service |
| 07 | [Storage](./07-storage.md) | touching Postgres, R2, Redis, Qdrant |
| 08 | [Security](./08-security.md) | touching auth, isolation, secrets, rate limits |
| 09 | [Deployment](./09-deployment.md) | touching infra, config, observability |
| 10 | [API Contracts](./10-api-contracts.md) | building against the API |
| 11 | [Frontend Architecture](./11-frontend-architecture.md) | working in the Next.js app |

## Decision records

Point-in-time decisions live separately in [`decisions/`](./decisions/) as ADRs, so the reasoning behind a choice doesn't get silently overwritten when the doc it lives in gets edited later.

- [ADR-001: Go + Python split](./decisions/ADR-001-go-python.md)
- [ADR-002: Qdrant as vector store](./decisions/ADR-002-qdrant.md)
- [ADR-003: Redis Streams as event backbone](./decisions/ADR-003-redis-streams.md)
- [ADR-004: NormalizedDocument abstraction](./decisions/ADR-004-normalized-document.md)

## Rule for maintaining this set

If a change affects more than one doc, update all of them in the same PR. If a decision changes entirely (not just an update — a reversal), add a new ADR rather than editing an old one; the old ADR stays as a historical record marked "Superseded by ADR-00X."

# 00 — Vision

## Problem Statement
Learners buy or collect large amounts of course content (videos, transcripts, PDFs, slides) but have no fast way to *ask* the material questions and jump straight to the relevant moment. Search inside a course is either non-existent or limited to filename search.

## Goals
- Let a user upload a course and chat with it in natural language.
- Ground every answer in the actual course content, with citations that link back to a timestamp or page.
- Keep answers fast, accurate, and safe (no hallucination, no leaking of unrelated data, no prompt-injection takeover).

## Non-Goals (for MVP)
- Multi-tenant enterprise SSO / SCIM.
- Live classroom / real-time collaboration.
- Full video editing or annotation tools.

## MVP Scope
- Single content type: `.srt` transcripts (text-first, cheapest to validate the pipeline).
- Single LLM/embedding/reranker provider — but accessed only through the provider interfaces in [02-system-architecture.md](./02-system-architecture.md#provider-abstraction), so switching later is a config change, not a rewrite.
- One workspace per user (no teams yet).

## Future Scope
- Additional content types (video, PDF, DOCX, GitHub repos, websites) — see [04-indexing-pipeline.md](./04-indexing-pipeline.md#roadmap).
- Teams / shared projects, RBAC.
- Analytics on what learners ask most.

## Related
- [01-product-requirements.md](./01-product-requirements.md) — what this looks like as screens and features.
- [02-system-architecture.md](./02-system-architecture.md) — how it's built.

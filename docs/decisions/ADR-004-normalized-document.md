# ADR-004: NormalizedDocument Abstraction

**Status:** Accepted

## Context
The roadmap ([04-indexing-pipeline.md](../04-indexing-pipeline.md#roadmap)) requires supporting SRT, video, PDF, DOCX, GitHub repos, and websites over time. Without a shared intermediate format, every new content type would require touching chunking, embedding, and retrieval code directly.

## Decision
Every parser converts its source format into a single `NormalizedDocument` shape (metadata, language, source, timeline, segments) before anything downstream touches it. Nothing past the Normalizer stage is aware of the original file type.

## Consequences
- **Positive:** Adding a new content type is exactly one new `DocumentParser` implementation — chunking, embedding, and retrieval code never change.
- **Positive:** Fields that don't apply to a given source type (e.g. `page_number` for a video, `start_ts` for a PDF) are simply nullable, rather than forcing separate schemas per type.
- **Negative:** The shared shape has to be designed generously enough up front to cover future content types reasonably well, or it will need a breaking version bump (see `normalization_version` in [Versioning Strategy](../04-indexing-pipeline.md#versioning-strategy)).
- **Mitigation:** `timeline` (boolean) and nullable timestamp/page fields were chosen specifically so both time-based and page-based sources fit the same schema without a fork.

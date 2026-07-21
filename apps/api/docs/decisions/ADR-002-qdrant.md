# ADR-002: Qdrant as Vector Store

**Status:** Accepted

## Context
The query pipeline ([05-query-pipeline.md](../05-query-pipeline.md)) needs a vector store for semantic retrieval, combined with keyword (BM25) search for hybrid retrieval.

## Decision
Use Qdrant as the vector store, treated as a derived, rebuildable index rather than a system of record.

## Consequences
- **Positive:** Qdrant supports payload filtering (course_id, timestamp) natively, which the hybrid retrieval and workspace-isolation requirements ([08-security.md](../08-security.md#workspace-isolation)) depend on.
- **Positive:** Because Postgres + R2 remain the source of truth ([07-storage.md](../07-storage.md)), Qdrant can be wiped and rebuilt at any time — this de-risks embedding model migrations (see [Versioning Strategy](../04-indexing-pipeline.md#versioning-strategy)).
- **Negative:** Running a separate vector store adds an operational component beyond Postgres.
- **Revisit if:** Postgres's own vector extensions become sufficient for the query patterns needed, at which point Qdrant could be dropped without changing anything upstream of the `VectorStore`-shaped write in the Worker.

# Go Background Workers

Owns: the entire indexing pipeline AND all persistence (Postgres, R2, Qdrant writes).
Calls the AI Service for compute but always does the writing itself.

See: docs/04-indexing-pipeline.md, docs/02-system-architecture.md#component-ownership

# Repository Interfaces

e.g. CourseRepository, ChunkRepository, ChatRepository — implemented in internal/infrastructure.
application/ depends only on these interfaces, never on a concrete DB package.

See: docs/02-system-architecture.md#module-dependency-diagram

# ADR-003: Redis Streams as Event Backbone

**Status:** Accepted

## Context
The indexing pipeline ([04-indexing-pipeline.md](../04-indexing-pipeline.md)) is a sequence of independent stages (parse, normalize, chunk, embed, index) that need to run asynchronously, be individually retryable, and scale independently.

## Decision
Use Redis Streams with consumer groups as the event backbone, with each pipeline stage as its own consumer group, coordinated via the event contract table.

## Consequences
- **Positive:** Already required in the stack for caching and rate limiting ([08-security.md](../08-security.md#rate-limits)), so no new infrastructure dependency.
- **Positive:** Consumer groups give per-stage scaling and retry semantics for free, which backs the Job state machine ([03-domain-model.md](../03-domain-model.md#job-lifecycle)).
- **Negative:** Redis Streams have weaker durability/replay guarantees than a dedicated message broker (Kafka, SQS) at very high scale.
- **Revisit if:** indexing volume grows to the point where stream retention, replay, or multi-region durability becomes a bottleneck — at that point, only the `infrastructure/queue` implementation needs to change, since `application/` code depends on a `Queue` interface, not Redis directly.

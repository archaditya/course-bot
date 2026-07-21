# ADR-001: Go + Python Split

**Status:** Accepted

## Context
The system needs both high-throughput, low-latency request handling (API, streaming, workers) and access to the strongest available ML/AI tooling (embeddings, reranking, LLM orchestration).

## Decision
- Go owns the API, background workers, and all persistence.
- Python owns AI compute only, as a stateless service (see [06-ai-service.md](../06-ai-service.md)).

## Consequences
- **Positive:** Go's concurrency model suits the worker pipeline and streaming API well; Python's ecosystem (transformers, LangChain-adjacent tooling, provider SDKs) is the strongest for AI compute.
- **Positive:** The stateless boundary means the Python service can be scaled, replaced, or even swapped for a different language later without touching Go orchestration logic.
- **Negative:** Two languages means two toolchains, two sets of dependency management, and a gRPC/HTTP boundary to maintain between them.
- **Mitigation:** The boundary is narrow and explicit — the provider interfaces in [02-system-architecture.md](../02-system-architecture.md#provider-abstraction) are the only contract that crosses the language boundary.

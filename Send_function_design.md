# Pipeline Overview

The function implements a **multi-route query pipeline**: a fast path for trivial greetings, a cache-hit path for repeated questions, and the full 10-step RAG pipeline for knowledge queries.

---

### Step 0 — Validation & User Message Persistence (Lines 107–124)

- Records `startTime` for telemetry.
- Verifies the conversation exists and belongs to the workspace (authz check).
- Persists the user's message to Postgres immediately, regardless of which route is taken.

### Step 0.5 — Fast Intent Classification (Lines 126–142)

A **string-prefix heuristic** checks if the input is a simple greeting (`hi`, `hello`, `hey`, `thanks`, `how are you`). If so, `isFastRoute = true` → the entire RAG pipeline (guardrails, embeddings, vector search, ranking) is **skipped**. A static context string is set and execution jumps directly to Step 9.

### Cache Hit Path (Lines 144–179)

If the cache is configured and the exact `(conversationID, userContent)` pair has been seen before:
- Deserializes the cached `MessageResult`.
- **Streams the cached content word-by-word** into `tokenCh` for a natural UX (no goroutine here — synchronous).
- Persists the assistant message and returns early. **No goroutines involved.**

---

## Full RAG Pipeline (Knowledge Route)

### Step 1 — Guardrails (Lines 183–196)

Two sequential calls to the AI service (via [s.guardrails](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L29)):
1. **Prompt injection detection** — rejects if flagged.
2. **PII detection** — rejects if personal data is found.

Both are synchronous/blocking. Failures are logged but non-fatal (soft guardrails).

### Step 2 & 3 — 🔀 Concurrent Query Enhancement + HyDE + Memory Search (Lines 198–238)

> **This is the first goroutine fan-out.**

Three goroutines run in parallel via `sync.WaitGroup` (count = 3):

| Goroutine | What it does | Line |
|---|---|---|
| **1** | `EnhanceQuery` — rewrites the query, generates sub-queries, step-back question | [208–211](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L208-L211) |
| **2** | `HydeDocument` — generates a hypothetical ideal answer document for better embedding | [212–215](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L212-L215) |
| **3** | `SearchMemory` — fetches long-term memory (Mem0) facts for the user/workspace | [216–219](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L216-L219) |

`wg.Wait()` at [line 220](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L220) blocks until all three complete. Results are then merged into `queryVariants` (original query + step-back + rewritten + sub-queries + HyDE doc).

**Why parallel?** These are independent LLM calls. Running them concurrently cuts latency from `~sum` to `~max`.

### Step 4 — Batch Embedding (Lines 240–244)

All `queryVariants` are embedded in a single batch call to the embedding provider. This is synchronous.

### Step 5 — 🔀 Parallel Vector Search (Lines 246–284)

> **Second goroutine fan-out.**

For each embedding vector, a goroutine fires a Qdrant search:

```
for _, vec := range allVecs {
    go func(v) { ... s.vectors.Search(ctx, conversationID, v, 20) ... }(vec)
}
```

- A **buffered channel** `searchCh` (capacity = `len(allVecs)`) collects results.
- A **separate closer goroutine** at [lines 263–266](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L263-L266) waits on `searchWg` then closes `searchCh`, enabling the `range searchCh` loop to terminate.
- Results are filtered by a **relevance threshold** (cosine similarity ≥ 0.35).

**Pattern:** Fan-out/fan-in with a WaitGroup + buffered channel + closer goroutine.

### Step 6 — Reciprocal Rank Fusion (Lines 286–288)

All filtered result sets are merged into a single ranked list using RRF. Top 20 results. This is pure in-memory computation — no goroutines.

### Fallback — Web Search via Tavily (Lines 289–362)

If RRF yields **zero results**, the function falls back to live web search (`s.aiClient.WebSearch`). If web results exist, it:
- Builds web context and citations.
- **Streams the LLM response** into `tokenCh` (synchronous range over token stream).
- Persists and returns early.

If web search also fails, sends a "couldn't find anything" message.

### Step 7 — Fetch Chunks from Postgres (Lines 364–376)

Loads the full chunk content by IDs from Postgres. Builds a lookup map `chunkByID`.

### Step 8 — Ranking + Deduplication (Lines 378–411)

- Takes top-K chunks (max 5) from RRF-ranked results.
- Deduplicates by `documentID:content` key.
- Builds the context string (capped at ~8000 chars).

### Step 9 — Conversational History (STM + LTM) (Lines 414–440)

- Prepends long-term memory facts (from the Mem0 goroutine in Step 3) to the context.
- Loads the last 10 messages from Postgres as short-term conversational history.
- Constructs the final `conversationHistory` prompt messages array.

### LLM Streaming with Retry (Lines 442–491)

- A `defer` at [lines 447–451](file:///b:/Personal-Projects/GenAI/Course-Bot/internal/application/chat/service.go#L447-L451) ensures `tokenCh` **always** gets a `Done` signal, even if all retries fail (prevents the HTTP handler from hanging).
- Retries up to `s.maxRetries` times. On each attempt:
  - Calls `s.aiClient.Stream()` to get a token channel.
  - Ranges over the token stream, forwarding each token to `tokenCh` (the SSE stream to the client).
  - On success, breaks out of the retry loop.
- If all retries fail, sends a friendly fallback message.

### 🔥 Fire-and-Forget Goroutine — Mem0 LTM Extraction (Lines 493–500)

> **Third goroutine — fire-and-forget background task.**

```go
go func() {
    addCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    s.aiClient.AddMemory(addCtx, string(ws), userContent, bestContent)
}()
```

This goroutine extracts and stores long-term memory facts from the current turn. Key design choices:
- Uses `context.Background()` (not the request `ctx`) so it **survives after the HTTP response is sent**.
- Has a **15-second timeout** to prevent leaking goroutines.
- Errors are only logged — failure doesn't affect the user response.

### Step 10 — Persist & Cache (Lines 502–569)

- Persists the assistant message to Postgres.
- Creates citation entities in batch.
- Logs structured telemetry (fast-route flag, vector search flag, citation count, latency, output preview).
- Caches the result in Redis with a **15-minute TTL** (only for knowledge-route queries where vector search was used).

---

## Goroutine Summary

| Location | Type | Purpose | Coordination |
|---|---|---|---|
| Lines 208–220 | **Fan-out (3)** | Query enhance, HyDE, Memory search | `sync.WaitGroup` — blocks until all 3 done |
| Lines 254–266 | **Fan-out (N)** | Parallel Qdrant vector search per embedding | `sync.WaitGroup` + buffered channel + closer goroutine |
| Lines 494–500 | **Fire-and-forget (1)** | Async Mem0 long-term memory extraction | None — detached with its own timeout context |
# Python AI Service (stateless)

```
apps/ai-service/
├── pyproject.toml
├── requirements.txt
├── main.py
├── config/
│   ├── __init__.py
│   └── settings.py
├── app/
│   ├── __init__.py
│   ├── providers/
│   │   ├── __init__.py
│   │   ├── base.py (interfaces: LLMProvider, EmbeddingProvider, etc.)
│   │   ├── openai/ (implementation)
│   │   └── anthropic/ (future)
│   ├── embedding/
│   │   ├── __init__.py
│   │   └── service.py
│   ├── title_generator/
│   │   ├── __init__.py
│   │   └── service.py
│   ├── summary_generator/
│   │   ├── __init__.py
│   │   └── service.py
│   ├── retriever/
│   │   ├── __init__.py
│   │   ├── qdrant_client.py
│   │   ├── bm25.py
│   │   └── service.py
│   ├── reranker/
│   │   ├── __init__.py
│   │   └── service.py
│   ├── generator/
│   │   ├── __init__.py
│   │   └── service.py
│   ├── evaluator/
│   │   ├── __init__.py
│   │   └── service.py
│   └── guardrails/
│       ├── __init__.py
│       ├── pii.py
│       ├── injection.py
│       └── service.py
├── api/
│   ├── __init__.py
│   ├── server.py (FastAPI/gRPC)
│   └── schemas.py (Pydantic models)
└── tests/
```

# AI Service - Detailed Architecture Explanation

## Core Purpose

The AI Service is a **stateless compute service**. It takes inputs, performs AI computations, and returns results. It **never writes to storage** - that responsibility belongs to the Go Workers and Go API.

**Why this design?**
- **Reusability**: Same compute functions serve indexing (chunk metadata generation) and querying (chat responses)
- **Horizontal scaling**: No state = easy to scale horizontally
- **Clear ownership**: Go decides what happens to results (write to Qdrant, stream to user), Python just computes

---

## Component Architecture

```
AI Service (Python)
├── Provider Layer (Pluggable AI backends)
│   ├── LLMProvider (OpenAI GPT-4o, GPT-4o-mini)
│   ├── EmbeddingProvider (OpenAI text-embedding-3-small)
│   ├── RerankerProvider (OpenAI-based reranking)
│   └── GuardrailProvider (PII + injection detection)
│
├── Service Layer (Business logic)
│   ├── EmbeddingService (text → vectors)
│   ├── TitleGeneratorService (content → short title)
│   ├── SummaryGeneratorService (content → 1-2 sentence summary)
│   ├── RetrieverService (hybrid search: Qdrant + BM25)
│   ├── RerankerService (reorder results by relevance)
│   ├── GeneratorService (streaming chat responses)
│   ├── EvaluatorService (score response quality 1-10)
│   └── GuardrailsService (PII + injection checks)
│
└── API Layer (FastAPI endpoints)
    └── REST endpoints for Go integration
```

---

## Data Flow 1: Indexing Pipeline

**When a user uploads a course:**

```
Go Worker → AI Service → Go Worker → Storage
```

### Step-by-Step:

1. **Go Worker** receives uploaded SRT file, parses it into chunks
2. **Go Worker** calls AI Service for each chunk:
   ```
   POST /embeddings
   Body: {"texts": ["chunk 1 text", "chunk 2 text", ...]}
   Response: {"embeddings": [[0.1, 0.2, ...], [0.3, 0.4, ...]], "model": "text-embedding-3-small"}
   ```
3. **AI Service** calls OpenAI embedding API, returns vectors
4. **Go Worker** calls AI Service for chunk metadata:
   ```
   POST /generate-title
   Body: {"content": "chunk text..."}
   Response: {"title": "Introduction to Neural Networks"}
   
   POST /generate-summary
   Body: {"content": "chunk text..."}
   Response: {"summary": "Explains the basics of neural networks."}
   ```
5. **AI Service** calls OpenAI LLM (mini model), returns title/summary
6. **Go Worker** writes vectors to Qdrant, metadata to Postgres

**Key Point**: AI Service only computes. Go Worker does all the writing.

---

## Data Flow 2: Query Pipeline (Chat)

**When a user asks a question:**

```
Go API → AI Service → Go API → Frontend
```

### Step-by-Step:

1. **User** sends question via frontend
2. **Go API** receives request, calls AI Service with guardrails check:
   ```
   POST /guardrails/check
   Body: {"text": "What is gradient descent?"}
   Response: {"passed": true, "reason": "Passed"}
   ```
3. **AI Service** checks for PII and prompt injection
4. **Go API** calls AI Service for retrieval:
   ```
   POST /retrieve
   Body: {"query": "What is gradient descent?", "course_id": "uuid", "collection_name": "course_123"}
   Response: {"chunks": [{"chunk_id": "abc", "content": "...", "score": 0.9}, ...]}
   ```
5. **AI Service** performs hybrid search:
   - Calls embedding provider to vectorize query
   - Calls Qdrant for vector search (semantic similarity)
   - Calls BM25 for keyword search (exact matches)
   - Combines scores with configurable weight (alpha)
6. **Go API** calls AI Service to rerank:
   ```
   POST /rerank
   Body: {"query": "What is gradient descent?", "candidates": [...]}
   Response: {"ranked_chunks": [...]}
   ```
7. **AI Service** reorders chunks by relevance using LLM
8. **Go API** calls AI Service for generation (streaming):
   ```
   POST /generate
   Body: {"query": "What is gradient descent?", "context": "chunk 1...\nchunk 2..."}
   Response: (streaming tokens)
   ```
9. **AI Service** streams tokens from OpenAI GPT-4o
10. **Go API** streams tokens to frontend via WebSocket
11. **Go API** calls AI Service for evaluation:
    ```
    POST /evaluate
    Body: {"query": "...", "response": "...", "context": "..."}
    Response: {"score": 8.5, "passes_threshold": true}
    ```
12. **AI Service** scores response quality (1-10 scale)
13. **Go API** saves message and citations to Postgres

**Retry Loop**: If score < 7.0, Go API retries generation up to 3 times, returns best attempt with disclaimer.

---

## Key Design Decisions

### 1. Provider Abstraction

```python
# Interface (base.py)
class LLMProvider(ABC):
    @abstractmethod
    async def generate(self, prompt: Prompt) -> Response:
        pass

# Implementation (openai/client.py)
class OpenAILLM(LLMProvider):
    async def generate(self, prompt: Prompt) -> Response:
        # OpenAI-specific code
```

**Benefit**: Switch from OpenAI to Anthropic by changing one config value (`LLM_PROVIDER=anthropic`) and adding a new implementation. No service code changes.

### 2. Prompt Versioning

Every generation request includes `prompt_version: "1.0"`:

```python
prompt = Prompt(
    text="...",
    prompt_version="1.0"  # Logged with result
)
```

**Benefit**: A/B test prompts, roll back bad prompts, trace regressions to specific prompt versions - all without redeploying.

### 3. Stateless Design

```python
# Service has no internal state
class EmbeddingService:
    def __init__(self, provider: EmbeddingProvider):
        self.provider = provider  # Only dependency
    
    async def embed_texts(self, texts: List[str]) -> List[Vector]:
        return await self.provider.embed(texts)  # Pure function
```

**Benefit**: Safe to scale horizontally, no coordination needed between instances.

### 4. Hybrid Retrieval

Combines two search methods:

- **Vector search (Qdrant)**: Semantic similarity - "machine learning" finds "neural networks"
- **BM25 keyword search**: Exact matches - "error code 404" finds "404"

```python
combined_score = alpha * vector_score + (1 - alpha) * bm25_score
```

**Benefit**: Best of both worlds - semantic understanding + exact term matching.

### 5. Guardrails

Two-layer protection:

- **Pre-guardrails**: Check user query for PII and injection before processing
- **Post-guardrails**: Check AI output for PII before sending to user

```python
# PII detection
async def check_pii(self, text: str) -> PIIResult:
    # Returns: {"has_pii": true, "detected_types": ["email", "phone"]}

# Injection detection  
async def check_injection(self, text: str) -> InjectionResult:
    # Returns: {"is_injection": true, "confidence": 0.9, "detected_pattern": "ignore previous instructions"}
```

**Benefit**: Prevents data leaks and prompt injection attacks.

### 6. Evaluator with Bounded Retry

```python
score = await evaluator.evaluate(query, response, context)  # 1-10 scale
if score < 7.0 and attempts < 3:
    retry_generation()
else:
    return_best_attempt()
```

**Benefit**: Ensures quality without infinite cost loops.

---

## API Endpoints Summary

| Endpoint | Purpose | Called By |
|----------|---------|-----------|
| `POST /embeddings` | Generate vectors | Go Worker (indexing) |
| `POST /generate-title` | Generate chunk titles | Go Worker (indexing) |
| `POST /generate-summary` | Generate chunk summaries | Go Worker (indexing) |
| `POST /retrieve` | Hybrid search | Go API (query) |
| `POST /rerank` | Reorder results | Go API (query) |
| `POST /generate` | Streaming chat response | Go API (query) |
| `POST /evaluate` | Score response quality | Go API (query) |
| `POST /guardrails/check` | PII/injection check | Go API (query) |
| `GET /healthz` | Health check | Deploy/monitoring |

---

## Configuration

All behavior controlled via environment variables:

```env
# Provider selection
LLM_PROVIDER=openai
EMBEDDING_PROVIDER=openai

# Model selection
OPENAI_LLM_LARGE_MODEL=gpt-4o
OPENAI_LLM_MINI_MODEL=gpt-4o-mini
OPENAI_EMBEDDING_MODEL=text-embedding-3-small

# Feature flags
GUARDRAILS_ENABLED=true
EVALUATOR_ENABLED=true
MAX_RETRIES=3
EVALUATOR_THRESHOLD=7.0

# Infrastructure
QDRANT_URL=http://localhost:6333
```

**Benefit**: Change behavior per environment (dev/staging/prod) without code changes.

---

## Integration with Go Services

**Communication Pattern:**
```
Go (HTTP client) → AI Service (FastAPI) → Go (processes result)
```

Go code:
```go
// Go calls AI Service
resp, err := http.Post("http://ai-service:8000/embeddings", ...)
embeddings := parseResponse(resp)

// Go writes to storage
qdrantClient.Upsert(embeddings)
postgresClient.SaveChunk(chunk)
```

**Key Principle**: AI Service is a pure function - inputs in, outputs out. Go owns the side effects.

---

## Summary

The AI Service is a **stateless, provider-agnostic compute layer** that:
1. Takes requests from Go services
2. Performs AI operations (embeddings, generation, retrieval, evaluation)
3. Returns results
4. Never touches storage directly

This design enables:
- Easy provider switching
- Horizontal scaling
- Clear separation of concerns
- Reusability across indexing and querying workflows
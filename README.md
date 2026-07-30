# 🧠 Course-Bot (archadiLM) — AI Course & Study Assistant

A production-grade, NotebookLM-inspired RAG (Retrieval-Augmented Generation) system built to transform course materials, documents, YouTube videos, and web pages into an interactive, grounded AI knowledge base.

> 🚀 **Built as part of the GenAI Engineering Course** guided by **Hitesh Choudhary Sir** and **Piyush Garg Sir**.

---

## 🌟 Key Features

- ✨ **NotebookLM-Style Source Guides**: Automatic 2-paragraph AI synthesis, dynamic content-specific questions, and source overviews upon indexing.
- ⚡ **Sub-100ms Redis Query Cache**: Fast response caching with automatic invalidation when sources are added or deleted.
- 📚 **Multi-Format Source Support**: PDF, DOCX, TXT, MD, SRT, VTT, YouTube Video URLs, Web Pages, and `.zip` archives.
- 🎯 **Advanced RAG Pipeline**: Fast-path route for smalltalk + 10-step Knowledge Route (Guardrails, HyDE, Step-Back Queries, Parallel Qdrant Vector Search, RRF Merging, & Cross-Encoder Reranking).
- 💬 **Real-time SSE Streaming**: Live response streaming with grounded citations, document titles, and video timecodes.
- 🔒 **Enterprise Guardrails**: Prompt injection detection and PII scanning via Groq / OpenAI.

---

## 🏗️ Architecture & Service Responsibilities

Course-Bot is designed around a modular microservice architecture:

```
                  ┌────────────────────────┐
                  │   Next.js 15 Frontend  │
                  └───────────┬────────────┘
                              │ SSE / REST
                              ▼
                  ┌────────────────────────┐
                  │    Go API Gateway      │
                  │      (apps/api)        │
                  └─────┬──────┬─────┬─────┘
                        │      │     │
       ┌────────────────┘      │     └────────────────┐
       ▼                       ▼                      ▼
┌──────────────┐      ┌────────────────┐     ┌────────────────┐
│  PostgreSQL  │      │  Redis Stream  │     │ Python AI Svc  │
│  (Data Store)│      │  & Cache       │     │(apps/ai-service)│
└──────────────┘      └───────┬────────┘     └────────────────┘
                              │ Events
                              ▼
                      ┌────────────────┐     ┌────────────────┐
                      │   Go Worker    │─────▶ Qdrant Vector  │
                      │ (apps/worker)  │     │ Store          │
                      └────────────────┘     └────────────────┘
```

### 1. `apps/api` — Go API Gateway
- **Responsibilities**: JWT Authentication, Workspace & Conversation CRUD, File Upload ingestion, SSE Chat Streaming, Guardrail execution, and Redis Response Caching.
- **Port**: `8080`

### 2. `apps/worker` — Go Asynchronous Worker Pipeline
- **Responsibilities**: Event-driven ingestion using Redis Streams (`pipeline:upload`, `pipeline:text-processed`). Handles document parsing, normalization, chunking, AI Source Intelligence generation (`/generate-source-intel`), embedding generation, Qdrant vector upserts, and PostgreSQL writes.

### 3. `apps/ai-service` — Python FastAPI AI Microservice
- **Responsibilities**: Stateless AI compute endpoints for embeddings (`text-embedding-3-small`), HyDE document generation, Query Enhancement (Step-Back + Sub-queries), Intent Classification, Guardrails (PII + Injection), and NotebookLM Source Intelligence Generation (via Groq Llama 3.1 / OpenAI GPT-4o).
- **Port**: `8000`

### 4. `frontend` — Next.js 15 Web Application
- **Responsibilities**: Modern, responsive dark-mode UI with live document status polling, real-time SSE streaming answer view with citation tabs, interactive Source Guide modals, dynamic question pills, and source management sidebars.
- **Port**: `3000`

---

## 💡 Why This Tech Stack? (And a Note on Over-Engineering)

Yes, I admit it — at first glance, building a course study assistant with **Go + Python + Next.js + Redis Streams + PostgreSQL + Qdrant + Cloudflare R2** might feel **over-engineered** for a simple project! 😅

However, this architecture was chosen very intentionally:

### 1. 🚀 Go (Golang) — API Gateway & Asynchronous Workers
- **Why**: Go’s concurrency model (Goroutines & Channels) makes it exceptionally fast for handling real-time Server-Sent Events (SSE) chat streaming with minimal CPU and memory overhead.
- **Role**: `apps/api` handles HTTP request routing, JWT security, SSE stream flushing, and Redis caching. `apps/worker` acts as an event-driven worker consuming Redis Streams for document indexing pipelines without blocking user HTTP requests.

### 2. 🐍 Python (FastAPI) — AI Compute Microservice
- **Why**: Python is the uncontested king of the AI/ML ecosystem.
- **Role**: `apps/ai-service` encapsulates all heavy AI compute — embeddings (`text-embedding-3-small`), HyDE document generation, intent classification, PII/prompt injection guardrails, BM25 keyword matching, PyMuPDF parsing, and Groq/OpenAI orchestration. Keeping AI logic in Python isolates AI library churn from core backend infrastructure.

### 3. ⚛️ Next.js 15 — Modern Web Frontend
- **Why**: React 19, TypeScript, and Next.js App Router provide a fast, type-safe development workflow for building responsive dark-mode interfaces, live status polling, and real-time SSE streaming components.

---

> ✋ **A Quick Note / Confession**:
> Is this stack over-engineered for a simple chatbot? **100% Yes!** 
> 
> I am actively working, building, and testing this kind of stacks as part of my daily working (in IT) and learning journey. Instead of keeping everything in a basic single-file script, I built this hybrid stack to practice **Event-Driven Architecture**, **Redis Stream pipelines**, **Distributed Caching**, and **Polyglot Microservice Orchestration (Go + Python)**. 
> 
> Sorry for the overengineering, but it was an incredible learning experience to build! 🚀

---

## 🔄 System Flow Diagrams

### 1. Document Indexing Pipeline Flow

```mermaid
sequenceDiagram
    autonumber
    actor User as Web Client
    participant API as Go API Gateway
    participant R2 as Cloudflare R2 / Disk
    participant Redis as Redis Stream
    participant Worker as Go Indexer Worker
    participant AI as Python AI Service
    participant Qdrant as Qdrant Vector DB
    participant DB as PostgreSQL

    User->>API: POST /conversations/{id}/documents (Upload File/URL/ZIP)
    API->>R2: Store Raw Source File
    API->>DB: Insert Document Record (status: UPLOADING)
    API->>Redis: XADD pipeline:upload {document_id, conversation_id}
    API-->>User: 202 Accepted {document_id}

    Redis->>Worker: Consume pipeline:upload
    Worker->>Worker: Parse & Normalize (PDF/Video/DOCX/Text)
    Worker->>AI: POST /generate-source-intel (Summary + Questions + Overview)
    AI-->>Worker: JSON {summary, questions, overview}
    Worker->>AI: POST /embeddings (Batch Chunk Texts)
    AI-->>Worker: Vector Embeddings Array
    Worker->>Qdrant: Upsert Points (Chunk Vectors + Payload)
    Worker->>DB: Save Chunks & Update Document (ai_summary, ai_questions, ai_overview, status: INDEXED)
```

---

### 2. Advanced RAG Query Pipeline Flow

```mermaid
sequenceDiagram
    autonumber
    actor User as Web Client
    participant API as Go API Gateway
    participant Cache as Redis Cache
    participant AI as Python AI Service
    participant Qdrant as Qdrant Vector DB
    participant DB as PostgreSQL
    participant LLM as OpenAI / Groq

    User->>API: POST /conversations/{id}/messages (SSE Stream)
    API->>API: Fast Route Check (Smalltalk / Greetings)
    
    alt Cache Hit
        API->>Cache: GET chat:v1:{convID}:{hash}
        Cache-->>API: Hit (Cached Result)
        API-->>User: Stream Cached Tokens + Citations (<100ms)
    else Cache Miss
        API->>AI: POST /guardrails/check (PII & Injection Check)
        
        par Parallel Enhancement
            API->>AI: POST /enhance-query (Step-Back + Sub-queries)
        and HyDE Generation
            API->>AI: POST /hyde-document (Hypothetical Doc)
        end
        
        API->>AI: POST /embeddings (Batch Query Variants)
        AI-->>API: Vector Array Returns
        API->>Qdrant: Parallel Vector Search (Conversation Scoped)
        Qdrant-->>API: Match Candidates
        API->>API: Reciprocal Rank Fusion (RRF) Merge
        API->>DB: Fetch Full Chunks (Content + Metadata + Source Names)
        API->>AI: POST /rerank (Cross-Encoder Top-K Selection)
        
        API->>LLM: Stream Prompt with Context Excerpts
        loop SSE Token Streaming
            LLM-->>API: Tokens
            API-->>User: SSE Data Event (data: token)
        end
        
        API->>DB: Persist Assistant Message + Citations
        API->>Cache: SET chat:v1:{convID}:{hash} (15m TTL)
        API-->>User: SSE Result & Done Event
    end
```

---

## 🛠️ Local Setup Guide (Without Docker)

Follow these steps to run Course-Bot locally without containerization.

### Prerequisites

Ensure you have the following installed on your machine:
- **Go**: `v1.25` or higher
- **Python**: `v3.10` or higher
- **Node.js**: `v18.0` or higher (with `npm`)
- **PostgreSQL**: `v16` running locally on port `5432`
- **Redis**: `v7` running locally on port `6379`
- **Qdrant**: running locally on port `6333` (e.g. `qdrant/qdrant` binary or single container)

---

### Step 1: Environment Configuration

Copy the example environment file:

```bash
cp .env.example .env
```

Ensure `.env` contains your valid credentials:

```env
# Postgres & Redis
POSTGRES_URL=postgres://postgres:564321@localhost:5432/courseassistant?sslmode=disable
REDIS_URL=redis://localhost:6379

# Qdrant & AI Service
QDRANT_URL=http://localhost:6333
AI_SERVICE_URL=http://127.0.0.1:8000

# API Keys
OPENAI_API_KEY=your_openai_api_key
GROQ_API_KEY=your_groq_api_key_optional
JWT_SIGNING_KEY=your_development_secret_key

SERVICE_ENV=development
MIGRATIONS_PATH=migrations
```

---

### Step 2: Database Setup & Migrations

1. Create PostgreSQL database:
```sql
CREATE DATABASE courseassistant;
CREATE USER courseassistant WITH PASSWORD 'courseassistant';
GRANT ALL PRIVILEGES ON DATABASE courseassistant TO courseassistant;
```

2. Run migrations automatically (the Go API Gateway automatically applies schema migrations on startup).

---

### Step 3: Run Python AI Service (`apps/ai-service`)

```bash
# Navigate to AI Service
cd apps/ai-service

# Create and activate virtual environment
python -m venv venv

# On Windows:
venv\Scripts\activate
# On Linux/Mac:
# source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Start the FastAPI server
uvicorn api.server:app --host 127.0.0.1 --port 8000 --reload
```

---

### Step 4: Run Go API Gateway (`apps/api`)

Open a new terminal in the project root:

```bash
# Run API Gateway
go run ./apps/api/cmd/api/main.go
```

The API service will start on `http://localhost:8080`.

---

### Step 5: Run Go Indexer Worker (`apps/worker`)

Open a new terminal in the project root:

```bash
# Run Background Worker
go run ./apps/worker/cmd/worker/main.go
```

---

### Step 6: Run Frontend Web App (`frontend`)

Open a new terminal:

```bash
# Navigate to frontend directory
cd frontend

# Install Node dependencies
npm install

# Start Next.js dev server
npm run dev
```

Open `http://localhost:3000` in your browser to launch the app! 🚀

---

## 📂 Repository Directory Layout

```
Course-Bot/
├── apps/
│   ├── ai-service/        # Python FastAPI AI microservice (Embeddings, RAG, Intel)
│   ├── api/               # Go API Gateway entrypoint (cmd/api/main.go)
│   └── worker/            # Go worker entrypoint (cmd/worker/main.go)
├── internal/
│   ├── application/       # Use case handlers (chat, upload, worker orchestration)
│   ├── domain/            # Entities, repository interfaces, LLM provider interfaces
│   ├── infrastructure/    # Concrete DB, Qdrant, Redis, R2, Sentry implementations
│   └── interfaces/        # HTTP routes, middleware, and handlers
├── frontend/              # Next.js 15 frontend application
├── migrations/            # SQL schema migration files (000001 - 000005)
├── docs/                  # Architecture documentation and ADRs
├── docker-compose.prod.yml# Production deployment configuration
├── .env.example           # Shared environment variables template
└── README.md              # Project documentation
```

---

## 🤝 Acknowledgements & Credits

Special thanks to **Hitesh Choudhary** (@HiteshChoudhary) and **Piyush Garg** (@piyushgarg_dev) for their mentorship and guidance throughout the **GenAI Engineering Course**.

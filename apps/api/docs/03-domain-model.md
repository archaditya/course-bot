# 03 — Domain Model

This is the vocabulary of the whole system. Every table, event, and API contract traces back to one of these objects — if a new concept doesn't fit here, this doc gets updated first, not last.

## Entities

| Entity | Definition |
|---|---|
| **User** | A person with an account. Authenticates via Google or email/password. Owns Workspaces. |
| **Workspace** | The billing/ownership boundary for a user (or, later, a team). Reserved for the multi-team phase; in MVP it's 1:1 with User. |
| **Project** | A folder-like grouping of Courses and Chats — e.g. "Machine Learning Bootcamp." |
| **Course** | A single body of material (e.g. one course's transcripts). Has a lifecycle — see below. |
| **Module** | An optional logical grouping *within* a course (e.g. "Week 3") — reserved in the model for future use, not required for MVP. |
| **Lesson** | The unit that maps to one uploaded source file. A Course contains many Lessons. |
| **Document** | The raw uploaded artifact tied to a Lesson, plus its parsed form. |
| **Chunk** | A retrievable slice of a Document, with its own timestamp range, embedding, and metadata — see [04-indexing-pipeline.md](./04-indexing-pipeline.md#chunk-schema). |
| **Conversation** | A chat thread within a Project, containing an ordered list of Messages. |
| **Citation** | A pointer from a specific Message back to the Chunk(s) it was grounded in. |
| **Job** | A unit of background work (parse, chunk, embed, etc.) tracked through its own lifecycle, independent of Course state. |

```mermaid
flowchart TB
    User --> Workspace
    Workspace --> Project
    Project --> Course
    Project --> Conversation
    Course --> Module
    Module --> Lesson
    Course --> Lesson
    Lesson --> Document
    Document --> Chunk
    Conversation --> Message
    Message --> Citation
    Citation --> Chunk
    Course --> Job
    Document --> Job
```

## State Machines

### Course Lifecycle

```mermaid
stateDiagram-v2
    [*] --> CREATED
    CREATED --> UPLOADING
    UPLOADING --> UPLOADED
    UPLOADED --> PARSING
    PARSING --> NORMALIZING
    NORMALIZING --> CHUNKING
    CHUNKING --> EMBEDDING
    EMBEDDING --> INDEXED
    INDEXED --> [*]

    UPLOADING --> FAILED
    PARSING --> FAILED
    NORMALIZING --> FAILED
    CHUNKING --> FAILED
    EMBEDDING --> FAILED
    FAILED --> PARSING: retry
    FAILED --> [*]: give up / user deletes
```

### Job Lifecycle

Every stage in the Course pipeline is executed by a Job, tracked independently so you can answer "which specific step is stuck" without inferring it from Course status alone.

```mermaid
stateDiagram-v2
    [*] --> QUEUED
    QUEUED --> RUNNING
    RUNNING --> SUCCEEDED
    RUNNING --> RETRYING
    RETRYING --> RUNNING
    RETRYING --> DEAD_LETTERED: max retries exceeded
    SUCCEEDED --> [*]
    DEAD_LETTERED --> [*]
```

Full retry/DLQ policy: [09-deployment.md](./09-deployment.md#error-handling).

### Conversation / Message Lifecycle

```mermaid
stateDiagram-v2
    [*] --> DRAFTED
    DRAFTED --> SENT
    SENT --> STREAMING
    STREAMING --> COMPLETED
    STREAMING --> LOW_CONFIDENCE: evaluator score < threshold after retries
    COMPLETED --> [*]
    LOW_CONFIDENCE --> [*]
```

Full retry/evaluator policy: [05-query-pipeline.md](./05-query-pipeline.md).

## Database Design

```mermaid
erDiagram
    USERS ||--o{ PROJECTS : owns
    PROJECTS ||--o{ COURSES : contains
    COURSES ||--o{ DOCUMENTS : contains
    DOCUMENTS ||--o{ CHUNKS : "split into"
    PROJECTS ||--o{ CHATS : has
    CHATS ||--o{ MESSAGES : contains
    MESSAGES ||--o{ CHUNKS : cites
    COURSES ||--o{ JOBS : "tracked by"

    USERS {
        uuid id PK
        string email
        string auth_provider
        timestamp created_at
    }
    PROJECTS {
        uuid id PK
        uuid user_id FK
        string name
        timestamp created_at
    }
    COURSES {
        uuid id PK
        uuid project_id FK
        string title
        string status
        timestamp created_at
    }
    DOCUMENTS {
        uuid id PK
        uuid course_id FK
        string source_type
        string storage_path
    }
    CHUNKS {
        uuid id PK
        uuid document_id FK
        text content
        int start_ts
        int end_ts
        string embedding_version
        string vector_ref
    }
    CHATS {
        uuid id PK
        uuid project_id FK
        string title
        timestamp created_at
    }
    MESSAGES {
        uuid id PK
        uuid chat_id FK
        string role
        text content
        timestamp created_at
    }
    JOBS {
        uuid id PK
        uuid course_id FK
        string stage
        string status
        int attempts
        timestamp created_at
    }
```

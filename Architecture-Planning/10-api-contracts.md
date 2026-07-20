# 10 — API Contracts

This doesn't enumerate every endpoint — it defines the shape of each major resource so an engineer or AI agent can extrapolate consistently. All endpoints are behind the auth described in [08-security.md](./08-security.md).

## Auth

**`POST /auth/login`**
```json
// Request
{ "email": "user@example.com", "password": "..." }

// Response 200
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "user": { "id": "uuid", "email": "user@example.com" }
}
```

**`POST /auth/refresh`**
```json
// Request
{ "refresh_token": "eyJ..." }

// Response 200
{ "access_token": "eyJ...", "refresh_token": "eyJ..." }
```

## Projects

**`POST /projects`**
```json
// Request
{ "name": "Machine Learning Bootcamp" }

// Response 201
{ "id": "uuid", "name": "Machine Learning Bootcamp", "created_at": "2026-07-21T10:00:00Z" }
```

## Courses

**`POST /projects/:project_id/courses`**
```json
// Request
{ "title": "Week 1 - Linear Regression" }

// Response 201
{ "id": "uuid", "title": "Week 1 - Linear Regression", "status": "CREATED" }
```

**`GET /courses/:id/status`**
```json
// Response 200
{
  "id": "uuid",
  "status": "CHUNKING",
  "jobs": [
    { "stage": "PARSING", "status": "SUCCEEDED" },
    { "stage": "NORMALIZING", "status": "SUCCEEDED" },
    { "stage": "CHUNKING", "status": "RUNNING" }
  ]
}
```
`status` values map directly to the Course Lifecycle state machine in [03-domain-model.md](./03-domain-model.md#course-lifecycle).

## Upload

**`POST /courses/:id/upload`**

`multipart/form-data` body containing the file(s), or a ZIP. Returns immediately — processing is async.

```json
// Response 202
{ "course_id": "uuid", "document_ids": ["uuid1", "uuid2"] }
```

Progress after this point is delivered over WebSocket (see [11-frontend-architecture.md](./11-frontend-architecture.md#websocket-layer)), not polled.

## Chat

**`POST /chats/:id/messages`**
```json
// Request
{ "content": "What does the video say about gradient descent?" }

// Response: streamed (SSE/WebSocket), final assembled shape:
{
  "message_id": "uuid",
  "role": "assistant",
  "content": "Gradient descent is described as...",
  "citations": [
    { "chunk_id": "uuid", "document_id": "uuid", "start_timestamp": 342, "title": "Intro to Optimization" }
  ],
  "confidence": "normal"  // or "low_confidence" — see 05-query-pipeline.md
}
```

## Status / Health

**`GET /healthz`**
```json
// Response 200
{ "status": "ok", "service": "api", "version": "1.4.2" }
```

## Conventions

- All resource IDs are UUIDs.
- All timestamps are ISO 8601 UTC.
- All list endpoints are paginated with `?cursor=` and `?limit=`, never offset-based.
- Errors follow a single shape:
```json
{ "error": { "code": "COURSE_NOT_FOUND", "message": "This course doesn't exist or you don't have access to it." } }
```

# 11 — Frontend Architecture

The visual direction and screen list live in the separate `AI_Course_Assistant_UI_Prompt.md`. This doc covers the engineering structure underneath it.

## Layer Overview

```
App Router (Next.js)
   ↓
Layouts (auth layout, dashboard layout, chat layout)
   ↓
Route Groups ((auth), (dashboard), (chat))
   ↓
React Query (server state: projects, courses, chat history)
   ↓
WebSocket Layer (live state: upload progress, indexing status, streaming chat tokens)
   ↓
Design System (shared components, tokens from the UI prompt)
   ↓
Feature Folders (one folder per product feature, not per component type)
```

## App Router & Route Groups

```
app/
├── (auth)/
│   ├── login/
│   └── signup/
├── (dashboard)/
│   ├── layout.tsx
│   ├── page.tsx                 # dashboard
│   └── projects/[id]/
├── (chat)/
│   └── chats/[id]/
└── layout.tsx                    # root layout
```

Route groups split by *auth requirement and layout shape*, not by feature — a logged-out landing page and a logged-in dashboard should never share a layout component just because they're both "top-level."

## React Query

Used for all server state that isn't live-streamed: project lists, course lists, chat history on load. Each resource from [10-api-contracts.md](./10-api-contracts.md) gets one query hook (`useProjects`, `useCourse`, `useChatHistory`) — components never call `fetch` directly.

## WebSocket Layer

A single WebSocket connection per session multiplexes:
- Course indexing status updates (mirrors the state machine in [03-domain-model.md](./03-domain-model.md#course-lifecycle))
- Chat token streaming (mirrors [05-query-pipeline.md](./05-query-pipeline.md))

A small client-side event bus dispatches incoming messages to the right React Query cache entry (e.g. an `INDEXED` event patches the course's cached status rather than triggering a full refetch), so UI updates feel instant without over-fetching.

## Design System

Shared components (buttons, citation markers, the processing stepper) live in one folder, built from the tokens defined in the UI prompt (palette, type scale, the citation "signature element"). Feature folders consume these components; they never redefine visual primitives locally.

## Feature Folders

```
features/
├── auth/
├── dashboard/
├── upload/
├── course-processing/
├── chat/
└── course-management/
```

Each feature folder owns its own components, hooks, and query definitions for that feature — this keeps a feature deletable/movable as a unit, rather than scattered across a `components/`, `hooks/`, and `pages/` split by file type.

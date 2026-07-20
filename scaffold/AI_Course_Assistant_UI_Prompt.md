# UI Build Prompt — AI Course Assistant

Use this as the brief you paste into an AI design/build tool (Claude, v0, Lovable, Bolt, Cursor, etc.) to generate the frontend. It describes the product, the screens, the interaction rules, and a visual direction — not just a feature list — so the output doesn't default to a generic SaaS template.

---

## 1. Product Context

Build the web UI for **an AI Course Assistant**: a tool where a learner uploads course material (starting with transcripts, later video/PDF/DOCX) and chats with it in natural language, getting answers grounded in the actual content with clickable citations that jump to the exact timestamp or page.

The product's emotional core: **turning a pile of passive course files into something you can have a conversation with.** The UI should feel like a calm, focused study tool — not a dashboard for managing infrastructure. The user is a learner, not a sysadmin, even though the underlying system is complex.

## 2. Screens to Build

1. **Landing page** — explains the product, one clear CTA to sign up.
2. **Login / Signup** — Google OAuth + email/password.
3. **Dashboard** — list of projects, recent chats, storage usage, upload progress.
4. **Create Project** — simple modal or page, name + optional description.
5. **Upload Course** — drag-and-drop zone, supports ZIP or individual files, live progress bar.
6. **Indexing Progress** — a stepper showing: Uploading → Extracting → Normalizing → Chunking → Generating Metadata → Embedding → Indexing → Completed. Should feel alive, not like a frozen loading bar.
7. **AI Chat** — the centerpiece screen. Streaming responses, markdown + code rendering, inline citations, and citations that are clickable and jump the embedded video/audio player to that timestamp.
8. **Course Management** — rename, delete, re-index, and view processing logs for a course.

## 3. Interaction Rules

- Chat responses **stream token by token** — never render a full block on completion, always as it arrives.
- Every factual claim in a chat response should carry a **citation marker** that, on click/hover, shows the source snippet and a "jump to timestamp" action.
- The indexing stepper should show **which stage is active right now**, not just a percentage — the user should be able to tell "it's currently generating embeddings" vs. "it's stuck."
- Errors and empty states speak in the product's voice, plainly: what happened, and what to do next. No vague "Something went wrong."
- Upload and processing states persist across a page refresh — a user leaving mid-upload and coming back should see accurate live state, not a reset.

## 4. Visual Direction

Don't default to the generic AI-tool look (cream background + terracotta accent, or near-black + single neon accent, or hairline-rule broadsheet). Instead:

- **Concept:** think "study room," not "server dashboard." Warm, quiet, legible — closer to a well-designed reading app (Readwise, Notion, a good e-reader) than a devtools console.
- **Palette:** pick 4–6 named hex values built around a paper-like neutral base with one confident accent color tied to "a highlighted passage in a textbook" — not a generic brand-blue.
- **Typography:** a display face with some warmth/character for headings (not a default geometric sans), a highly legible body face for chat content (this is a reading-heavy product), and a monospace/utility face for timestamps, code blocks, and metadata.
- **Signature element:** the timestamp citation itself should be the one memorable, deliberately-designed UI element on the page — treat it like a "sticky note" or "highlighted underline" reference rather than a plain footnote number.
- **Motion:** used sparingly — a satisfying reveal when a course finishes indexing, a subtle typing/streaming indicator in chat. Avoid decorative animation everywhere.
- **Structure:** the processing stepper is a genuine sequence, so numbering/ordering there is earned and correct — don't invent numbered steps elsewhere just for decoration.

## 5. Content & Copy Rules

- Write from the learner's point of view: "Your course is ready" not "Indexing job completed."
- Buttons say exactly what they do: "Upload course," "Ask a question," not "Submit."
- Empty states are invitations, not dead ends: an empty dashboard should prompt "Upload your first course" with the upload action right there, not just say "No projects yet."
- Keep the tone conversational and calm — this is a study companion, not enterprise software.

## 6. Technical Constraints

- Frontend: Next.js (React), responsive down to mobile.
- Visible keyboard focus states everywhere; respect `prefers-reduced-motion`.
- Chat and upload progress both rely on streaming/WebSocket updates from the backend — design components to receive partial/incremental state, not just a final payload.

---

*Paste this whole file as the prompt. If the tool asks for one screen at a time, start with the AI Chat screen — it's the product's core and the one every other screen should visually agree with.*

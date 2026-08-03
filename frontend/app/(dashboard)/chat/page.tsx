"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  BASE,
  apiListConversations,
  apiCreateConversation,
  apiGetConversationMessages,
  apiGetChunk,
  apiDeleteConversation,
  apiUpdateConversationTitle,
  apiListDocuments,
  apiUploadDocument,
  apiAddSource,
  apiDeleteDocument,
  apiCreateLearningNode,
  apiListLearningNodes,
  apiDeleteLearningNode,
  getToken,
  type Conversation,
  type ChunkDetail,
  type DocumentItem,
  type DocumentStatus,
  type LearningNode,
  type ToolType,
} from "@/lib/api";
import { Spinner, SourceTypeIcon } from "@/design-system";

function formatTime(secs?: number): string {
  if (secs == null) return "";
  const m = Math.floor(secs / 60);
  const s = Math.floor(secs % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

function cleanFilename(name: string): string {
  if (!name) return "this source";
  return name
    .replace(/^\d+[\.\-_]\s*/, "") // Strips numeric prefixes like "11. "
    .replace(/\.(mp4|pdf|vtt|srt|docx|txt|html|md)$/i, "") // Strips file extensions only
    .trim();
}

function getSourceColor(sourceType?: string): { color: string; bg: string; border: string } {
  switch (sourceType?.toLowerCase()) {
    case "pdf":
      return { color: "#EF4444", bg: "rgba(239, 68, 68, 0.12)", border: "rgba(239, 68, 68, 0.3)" };
    case "video":
    case "video_url":
    case "youtube":
      return { color: "#3B82F6", bg: "rgba(59, 130, 246, 0.12)", border: "rgba(59, 130, 246, 0.3)" };
    case "srt":
    case "vtt":
      return { color: "#10B981", bg: "rgba(16, 185, 129, 0.12)", border: "rgba(16, 185, 129, 0.3)" };
    case "url":
      return { color: "#8B5CF6", bg: "rgba(139, 92, 246, 0.12)", border: "rgba(139, 92, 246, 0.3)" };
    case "text":
      return { color: "#F59E0B", bg: "rgba(245, 158, 11, 0.12)", border: "rgba(245, 158, 11, 0.3)" };
    case "web_search":
      return { color: "#EC4899", bg: "rgba(236, 72, 153, 0.12)", border: "rgba(236, 72, 153, 0.3)" };
    default:
      return { color: "var(--color-primary)", bg: "var(--color-accent-light)", border: "var(--color-accent-border)" };
  }
}

function parseTimestampInSeconds(chunk: ChunkDetail): number {
  if (typeof chunk.start_timestamp === "number" && chunk.start_timestamp > 0) {
    return chunk.start_timestamp;
  }
  const text = (chunk.content || "") + " " + (chunk.title || "");
  const match = text.match(/\[(\d{1,2}):(\d{2})(?::(\d{2}))?\]/);
  if (match) {
    if (match[3]) {
      return parseInt(match[1], 10) * 3600 + parseInt(match[2], 10) * 60 + parseInt(match[3], 10);
    }
    return parseInt(match[1], 10) * 60 + parseInt(match[2], 10);
  }
  return 0;
}

function extractYouTubeId(chunk: ChunkDetail): string | null {
  const searchStr = `${chunk.source_url || ""} ${chunk.document_name || ""} ${chunk.title || ""} ${chunk.content || ""}`;
  const match = searchStr.match(/(?:v=|\/|embed\/|youtu\.be\/)([a-zA-Z0-9_-]{11})/);
  return match ? match[1] : null;
}

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  confidence?: string;
  citations?: Array<{
    chunk_id: string;
    document_id: string;
    start_timestamp?: number;
    title?: string;
  }>;
}

const STATUS_LABEL: Record<DocumentStatus, string> = {
  UPLOADING: "Uploading",
  UPLOADED: "Queued",
  PARSING: "Extracting",
  NORMALIZING: "Normalizing",
  CHUNKING: "Chunking",
  EMBEDDING: "Embedding",
  INDEXED: "Ready",
  FAILED: "Failed",
};

function StatusPill({ status }: { status: DocumentStatus }) {
  const isDone = status === "INDEXED";
  const isFailed = status === "FAILED";
  const isBusy = !isDone && !isFailed;
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "5px",
        padding: "2px 8px",
        borderRadius: "var(--radius-sm)",
        fontSize: "10.5px",
        fontFamily: "var(--font-geist)",
        fontWeight: 600,
        letterSpacing: "0.02em",
        background: isDone ? "var(--color-tertiary-container)" : isFailed ? "var(--color-error-container)" : "var(--color-surface-container-high)",
        color: isDone ? "var(--color-on-tertiary-container)" : isFailed ? "var(--color-on-error-container)" : "var(--color-on-surface-variant)",
        flexShrink: 0,
      }}
    >
      {isBusy && <Spinner size={9} color="currentColor" />}
      {isDone && <span className="material-symbols-outlined" style={{ fontSize: "12px" }}>check</span>}
      {isFailed && <span className="material-symbols-outlined" style={{ fontSize: "12px" }}>error</span>}
      {STATUS_LABEL[status]}
    </span>
  );
}

// ── Add Document modal ───────────────────────────────────────────────────
function AddDocumentModal({
  onClose,
  onUploadFile,
  onUploadZip,
  onAddVideoUrl,
  onAddWebUrl,
  onAddText,
}: {
  onClose: () => void;
  onUploadFile: (file: File) => Promise<void>;
  onUploadZip: (file: File) => Promise<void>;
  onAddVideoUrl: (url: string) => Promise<void>;
  onAddWebUrl: (url: string) => Promise<void>;
  onAddText: (title: string, content: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<"file" | "url" | "video" | "zip" | "text">("file");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [videoUrl, setVideoUrl] = useState("");
  const [webUrl, setWebUrl] = useState("");
  const [textTitle, setTextTitle] = useState("");
  const [textContent, setTextContent] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const zipInputRef = useRef<HTMLInputElement>(null);

  const tabs = [
    { key: "file" as const, label: "PDF / Doc", icon: "description" },
    { key: "url" as const, label: "Web URL", icon: "language" },
    { key: "video" as const, label: "YouTube URL", icon: "smart_display" },
    { key: "zip" as const, label: "ZIP", icon: "folder_zip" },
    { key: "text" as const, label: "Text", icon: "notes" },
  ];

  async function guarded(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed", inset: 0, background: "rgba(0,0,0,0.6)", zIndex: 100,
        display: "flex", alignItems: "center", justifyContent: "center", padding: "20px",
      }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        transition={{ duration: 0.18 }}
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "100%", maxWidth: "520px", background: "var(--color-surface-container-low)",
          border: "1px solid var(--color-outline-variant)", borderRadius: "var(--radius-xl)",
          boxShadow: "var(--shadow-lg)", overflow: "hidden",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "18px 20px 12px" }}>
          <h3 style={{ margin: 0, fontFamily: "var(--font-geist)", fontSize: "16px", fontWeight: 700 }}>Add a source</h3>
          <button onClick={onClose} style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex" }}>
            <span className="material-symbols-outlined" style={{ fontSize: "20px" }}>close</span>
          </button>
        </div>

        <div style={{ display: "flex", gap: "4px", padding: "0 20px 14px" }}>
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => { setTab(t.key); setError(null); }}
              style={{
                flex: 1, display: "flex", flexDirection: "column", alignItems: "center", gap: "4px",
                padding: "10px 4px", borderRadius: "var(--radius-md)",
                border: `1px solid ${tab === t.key ? "var(--color-accent-border)" : "var(--color-outline-variant)"}`,
                background: tab === t.key ? "var(--color-accent-light)" : "transparent",
                color: tab === t.key ? "var(--color-primary-fixed)" : "var(--color-on-surface-variant)",
                cursor: "pointer", fontFamily: "var(--font-geist)", fontSize: "10px", fontWeight: 600,
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>{t.icon}</span>
              {t.label}
            </button>
          ))}
        </div>

        <div style={{ padding: "4px 20px 20px", minHeight: "140px" }}>
          {tab === "file" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <p style={{ fontSize: "12.5px", color: "var(--color-on-surface-variant)", margin: 0 }}>
                Upload a PDF, Word doc, subtitle (.srt/.vtt), or plain text file.
              </p>
              <input ref={fileInputRef} type="file" accept=".pdf,.docx,.txt,.md,.srt,.vtt" style={{ display: "none" }}
                onChange={(e) => { const f = e.target.files?.[0]; if (f) guarded(() => onUploadFile(f)); }} />
              <button onClick={() => fileInputRef.current?.click()} disabled={busy} style={dropZoneStyle}>
                <span className="material-symbols-outlined" style={{ fontSize: "26px", color: "var(--color-primary)" }}>upload_file</span>
                Click to choose a file
              </button>
            </div>
          )}

          {tab === "url" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <p style={{ fontSize: "12.5px", color: "var(--color-on-surface-variant)", margin: 0 }}>
                Paste any website URL to scrape and index its readable content (powered by Firecrawl).
              </p>
              <input
                value={webUrl}
                onChange={(e) => setWebUrl(e.target.value)}
                placeholder="https://example.com/blog-or-docs"
                className="input-glow"
                style={inputStyle}
              />
              <button
                onClick={() => webUrl.trim() && guarded(() => onAddWebUrl(webUrl.trim()))}
                disabled={busy || !webUrl.trim()}
                style={primaryBtnStyle(busy || !webUrl.trim())}
              >
                {busy ? <Spinner size={14} color="var(--color-on-primary)" /> : "Scrape & add URL"}
              </button>
            </div>
          )}

          {tab === "video" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <p style={{ fontSize: "12.5px", color: "var(--color-on-surface-variant)", margin: 0 }}>Paste a YouTube video URL.</p>
              <input
                value={videoUrl}
                onChange={(e) => setVideoUrl(e.target.value)}
                placeholder="https://www.youtube.com/watch?v=…"
                className="input-glow"
                style={inputStyle}
              />
              <button
                onClick={() => videoUrl.trim() && guarded(() => onAddVideoUrl(videoUrl.trim()))}
                disabled={busy || !videoUrl.trim()}
                style={primaryBtnStyle(busy || !videoUrl.trim())}
              >
                {busy ? <Spinner size={14} color="var(--color-on-primary)" /> : "Add video"}
              </button>
            </div>
          )}

          {tab === "zip" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <p style={{ fontSize: "12.5px", color: "var(--color-on-surface-variant)", margin: 0 }}>
                Upload a .zip — every supported file inside is added and indexed as its own source, in parallel.
              </p>
              <input ref={zipInputRef} type="file" accept=".zip" style={{ display: "none" }}
                onChange={(e) => { const f = e.target.files?.[0]; if (f) guarded(() => onUploadZip(f)); }} />
              <button onClick={() => zipInputRef.current?.click()} disabled={busy} style={dropZoneStyle}>
                <span className="material-symbols-outlined" style={{ fontSize: "26px", color: "var(--color-primary)" }}>folder_zip</span>
                Click to choose a .zip archive
              </button>
            </div>
          )}

          {tab === "text" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <input
                value={textTitle}
                onChange={(e) => setTextTitle(e.target.value)}
                placeholder="Title (optional)"
                className="input-glow"
                style={inputStyle}
              />
              <div>
                <textarea
                  value={textContent}
                  onChange={(e) => setTextContent(e.target.value)}
                  placeholder="Paste or type text…"
                  className="input-glow"
                  rows={5}
                  style={{ ...inputStyle, resize: "vertical", fontFamily: "var(--font-inter)" }}
                />
                <div style={{ display: "flex", justifyContent: "flex-end", marginTop: "4px" }}>
                  <span style={{ fontSize: "11px", fontFamily: "var(--font-geist)", color: textContent.length > 50000 ? "var(--color-error)" : "var(--color-on-surface-variant)" }}>
                    {textContent.length.toLocaleString()} / 50,000 chars
                  </span>
                </div>
              </div>
              <button
                onClick={() => textContent.trim() && textContent.length <= 50000 && guarded(() => onAddText(textTitle.trim(), textContent.trim()))}
                disabled={busy || !textContent.trim() || textContent.length > 50000}
                style={primaryBtnStyle(busy || !textContent.trim() || textContent.length > 50000)}
              >
                {busy ? <Spinner size={14} color="var(--color-on-primary)" /> : "Add text"}
              </button>
            </div>
          )}

          {error && <p style={{ marginTop: "10px", fontSize: "12px", color: "var(--color-error)" }}>{error}</p>}
        </div>
      </motion.div>
    </div>
  );
}

// ── Generate Learning Tool Modal ─────────────────────────────────────────
function GenerateLearningToolModal({
  onClose,
  onGenerate,
}: {
  onClose: () => void;
  onGenerate: (toolType: ToolType, title: string) => Promise<void>;
}) {
  const [selectedType, setSelectedType] = useState<ToolType>("summary");
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const tools: Array<{ type: ToolType; title: string; desc: string; icon: string; color: string }> = [
    { type: "summary", title: "Summary", desc: "A structured markdown summary of your sources", icon: "description", color: "#22C55E" },
    { type: "key_takeaways", title: "Key Takeaways", desc: "Bullet-point insights you can copy and review", icon: "format_list_bulleted", color: "#3B82F6" },
    { type: "flashcards", title: "Flashcards", desc: "Flip cards for active recall study", icon: "style", color: "#F59E0B" },
    { type: "quiz", title: "Quiz", desc: "Multiple choice quiz with explanations", icon: "quiz", color: "#EC4899" },
    { type: "mind_map", title: "Mind Map", desc: "Visual concept map of the material", icon: "account_tree", color: "#8B5CF6" },
    { type: "ai_report", title: "AI Report", desc: "Long-form report with sections", icon: "article", color: "#06B6D4" },
  ];

  async function handleGenerate() {
    setBusy(true);
    setError(null);
    try {
      await onGenerate(selectedType, title);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Generation failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed", inset: 0, background: "rgba(0,0,0,0.7)", zIndex: 110,
        display: "flex", alignItems: "center", justifyContent: "center", padding: "20px",
      }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95, y: 10 }}
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "100%", maxWidth: "520px", background: "var(--color-surface-container-low)",
          border: "1px solid var(--color-outline-variant)", borderRadius: "var(--radius-xl)",
          padding: "24px", boxShadow: "var(--shadow-lg)", overflow: "hidden",
        }}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "6px" }}>
          <h3 style={{ margin: 0, fontFamily: "var(--font-geist)", fontSize: "17px", fontWeight: 700 }}>
            Generate learning tool
          </h3>
          <button onClick={onClose} style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer" }}>
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>
        <p style={{ margin: "0 0 18px 0", fontSize: "12.5px", color: "var(--color-on-surface-variant)", lineHeight: 1.5 }}>
          Uses all ready sources in this workspace. Generation runs in the background.
        </p>

        <p style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", margin: "0 0 8px 0" }}>
          Type
        </p>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "10px", marginBottom: "18px" }}>
          {tools.map((t) => {
            const isSelected = selectedType === t.type;
            return (
              <button
                key={t.type}
                type="button"
                onClick={() => setSelectedType(t.type)}
                style={{
                  padding: "12px 14px",
                  borderRadius: "var(--radius-md)",
                  border: `1.5px solid ${isSelected ? t.color : "var(--color-outline-variant)"}`,
                  background: isSelected ? `${t.color}15` : "var(--color-surface-container-lowest)",
                  textAlign: "left",
                  cursor: "pointer",
                  transition: "all var(--transition-fast)",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px" }}>
                  <span className="material-symbols-outlined" style={{ fontSize: "18px", color: isSelected ? t.color : "var(--color-on-surface-variant)" }}>
                    {t.icon}
                  </span>
                  <span style={{ fontFamily: "var(--font-geist)", fontSize: "13px", fontWeight: 700, color: isSelected ? t.color : "var(--color-on-surface)" }}>
                    {t.title}
                  </span>
                </div>
                <p style={{ margin: 0, fontSize: "11px", color: "var(--color-on-surface-variant)", lineHeight: 1.35 }}>
                  {t.desc}
                </p>
              </button>
            );
          })}
        </div>

        <div style={{ marginBottom: "20px" }}>
          <p style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", margin: "0 0 6px 0" }}>
            Title (optional)
          </p>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Custom title"
            className="input-glow"
            style={inputStyle}
          />
        </div>

        {error && <p style={{ margin: "0 0 12px 0", fontSize: "12px", color: "var(--color-error)" }}>{error}</p>}

        <div style={{ display: "flex", justifyContent: "flex-end", gap: "10px" }}>
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: "8px 16px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)",
              background: "var(--color-surface-container)", color: "var(--color-on-surface)", fontSize: "13px", fontWeight: 600, cursor: "pointer",
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleGenerate}
            disabled={busy}
            style={{
              padding: "8px 20px", borderRadius: "var(--radius-md)", border: "none",
              background: "var(--color-primary)", color: "var(--color-on-primary)", fontSize: "13px", fontWeight: 600, cursor: busy ? "not-allowed" : "pointer", opacity: busy ? 0.7 : 1, display: "flex", alignItems: "center", gap: "6px",
            }}
          >
            {busy ? <Spinner size={14} color="var(--color-on-primary)" /> : "Generate"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

// ── Learning Node Viewer Modal ───────────────────────────────────────────
function MindMapTreeNode({ label, children }: { label: string; children?: any[] }) {
  const [open, setOpen] = useState(true);
  const hasChildren = children && children.length > 0;
  return (
    <div style={{ marginLeft: "14px", marginTop: "6px" }}>
      <div
        onClick={() => hasChildren && setOpen(!open)}
        style={{
          display: "inline-flex", alignItems: "center", gap: "6px", padding: "4px 10px",
          borderRadius: "var(--radius-md)", background: "var(--color-surface-container-high)",
          border: "1px solid var(--color-outline-variant)", cursor: hasChildren ? "pointer" : "default",
          fontSize: "12.5px", fontFamily: "var(--font-geist)", fontWeight: 600,
        }}
      >
        {hasChildren && (
          <span className="material-symbols-outlined" style={{ fontSize: "14px", color: "var(--color-primary)" }}>
            {open ? "expand_more" : "chevron_right"}
          </span>
        )}
        <span>{label}</span>
      </div>
      {open && hasChildren && (
        <div style={{ borderLeft: "1.5px dashed var(--color-accent-border)", marginLeft: "10px", paddingLeft: "6px" }}>
          {children.map((child: any, idx: number) => (
            <MindMapTreeNode key={idx} label={child.label} children={child.children} />
          ))}
        </div>
      )}
    </div>
  );
}

function LearningNodeViewerModal({
  node,
  onClose,
}: {
  node: LearningNode;
  onClose: () => void;
}) {
  const content = node.content || {};

  // 1. Flashcards state
  const [cardIndex, setCardIndex] = useState(0);
  const [isFlipped, setIsFlipped] = useState(false);
  const cards: Array<{ front: string; back: string }> = content.cards || [];

  // 2. Quiz state
  const [quizIndex, setQuizIndex] = useState(0);
  const [selectedOption, setSelectedOption] = useState<number | null>(null);
  const [quizScore, setQuizScore] = useState(0);
  const [quizFinished, setQuizFinished] = useState(false);
  const quizItems: Array<{ question: string; options: string[]; correct: number; explanation: string }> = content.quiz || [];

  function handleOptionSelect(idx: number) {
    if (selectedOption !== null) return;
    setSelectedOption(idx);
    if (idx === quizItems[quizIndex]?.correct) {
      setQuizScore((s) => s + 1);
    }
  }

  function handleNextQuestion() {
    if (quizIndex + 1 < quizItems.length) {
      setQuizIndex((i) => i + 1);
      setSelectedOption(null);
    } else {
      setQuizFinished(true);
    }
  }

  function handleResetQuiz() {
    setQuizIndex(0);
    setSelectedOption(null);
    setQuizScore(0);
    setQuizFinished(false);
  }

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed", inset: 0, background: "rgba(0,0,0,0.75)", zIndex: 110,
        display: "flex", alignItems: "center", justifyContent: "center", padding: "20px",
      }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95, y: 10 }}
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "100%", maxWidth: "660px", background: "var(--color-surface-container-low)",
          border: "1px solid var(--color-outline-variant)", borderRadius: "var(--radius-xl)",
          padding: "24px", boxShadow: "var(--shadow-lg)", maxHeight: "85vh", display: "flex", flexDirection: "column",
        }}
      >
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "16px" }}>
          <div>
            <span style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-tertiary)" }}>
              ✨ {node.tool_type.replace("_", " ")}
            </span>
            <h3 style={{ margin: "4px 0 0 0", fontFamily: "var(--font-geist)", fontSize: "18px", fontWeight: 700 }}>
              {node.title}
            </h3>
          </div>
          <button onClick={onClose} style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer" }}>
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>

        <div style={{ flex: 1, overflowY: "auto", background: "var(--color-surface-container-lowest)", padding: "18px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)" }}>
          {node.status === "generating" ? (
            <div style={{ padding: "40px", textAlign: "center", color: "var(--color-on-surface-variant)" }}>
              <Spinner size={24} color="var(--color-primary)" />
              <p style={{ margin: "12px 0 0 0", fontSize: "13px" }}>Generating {node.title} in background…</p>
            </div>
          ) : node.status === "failed" ? (
            <div style={{ padding: "20px", color: "var(--color-error)", fontSize: "13px" }}>
              ⚠️ Generation failed: {content.error || "Something went wrong."}
            </div>
          ) : node.tool_type === "flashcards" ? (
            cards.length === 0 ? (
              <p style={{ color: "var(--color-on-surface-variant)", fontSize: "13px" }}>No flashcards found.</p>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: "16px" }}>
                <div style={{ fontSize: "12px", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)", fontWeight: 600 }}>
                  Card {cardIndex + 1} of {cards.length} • Click card to flip 🔄
                </div>

                <motion.div
                  onClick={() => setIsFlipped(!isFlipped)}
                  animate={{ rotateY: isFlipped ? 180 : 0 }}
                  transition={{ duration: 0.3 }}
                  style={{
                    width: "100%", minHeight: "180px", borderRadius: "var(--radius-lg)",
                    background: isFlipped ? "var(--color-surface-container-high)" : "var(--color-surface-container)",
                    border: `1.5px solid ${isFlipped ? "var(--color-tertiary)" : "var(--color-accent-border)"}`,
                    padding: "24px", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center",
                    textAlign: "center", cursor: "pointer", boxShadow: "var(--shadow-md)",
                  }}
                >
                  <div style={{ transform: isFlipped ? "rotateY(180deg)" : "none", width: "100%" }}>
                    <span style={{ fontSize: "10.5px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: isFlipped ? "var(--color-tertiary)" : "var(--color-primary)", display: "block", marginBottom: "8px" }}>
                      {isFlipped ? "Answer" : "Question"}
                    </span>
                    <p style={{ fontSize: "15px", fontFamily: "var(--font-inter)", fontWeight: 600, margin: 0, lineHeight: 1.5, color: "var(--color-on-surface)" }}>
                      {isFlipped ? cards[cardIndex]?.back : cards[cardIndex]?.front}
                    </p>
                  </div>
                </motion.div>

                <div style={{ display: "flex", gap: "12px", width: "100%", justifyContent: "space-between" }}>
                  <button
                    onClick={() => { setCardIndex((i) => (i > 0 ? i - 1 : cards.length - 1)); setIsFlipped(false); }}
                    style={{ padding: "8px 16px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)", background: "var(--color-surface-container)", color: "var(--color-on-surface)", cursor: "pointer", fontSize: "12.5px" }}
                  >
                    ← Previous
                  </button>
                  <button
                    onClick={() => { setCardIndex((i) => (i + 1) % cards.length); setIsFlipped(false); }}
                    style={{ padding: "8px 16px", borderRadius: "var(--radius-md)", border: "none", background: "var(--color-primary)", color: "var(--color-on-primary)", cursor: "pointer", fontSize: "12.5px", fontWeight: 600 }}
                  >
                    Next →
                  </button>
                </div>
              </div>
            )
          ) : node.tool_type === "quiz" ? (
            quizItems.length === 0 ? (
              <p style={{ color: "var(--color-on-surface-variant)", fontSize: "13px" }}>No quiz questions found.</p>
            ) : quizFinished ? (
              <div style={{ textAlign: "center", padding: "20px" }}>
                <div style={{ width: "56px", height: "56px", borderRadius: "50%", background: "var(--color-tertiary-container)", color: "var(--color-on-tertiary-container)", display: "grid", placeItems: "center", margin: "0 auto 12px", fontSize: "28px" }}>
                  🏆
                </div>
                <h4 style={{ fontSize: "18px", margin: "0 0 6px 0", fontFamily: "var(--font-geist)" }}>Quiz Complete!</h4>
                <p style={{ fontSize: "14px", color: "var(--color-on-surface-variant)", margin: "0 0 16px 0" }}>
                  You scored <strong style={{ color: "var(--color-tertiary)" }}>{quizScore}</strong> out of <strong>{quizItems.length}</strong> ({Math.round((quizScore / quizItems.length) * 100)}%)
                </p>
                <button
                  onClick={handleResetQuiz}
                  style={{ padding: "8px 20px", borderRadius: "var(--radius-md)", background: "var(--color-primary)", color: "var(--color-on-primary)", border: "none", fontWeight: 600, cursor: "pointer" }}
                >
                  Restart Quiz 🔄
                </button>
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
                <div style={{ display: "flex", justifyContent: "space-between", fontSize: "12px", color: "var(--color-on-surface-variant)", fontWeight: 600, fontFamily: "var(--font-geist)" }}>
                  <span>Question {quizIndex + 1} of {quizItems.length}</span>
                  <span>Score: {quizScore}</span>
                </div>

                <h4 style={{ margin: 0, fontSize: "14.5px", fontFamily: "var(--font-inter)", lineHeight: 1.5, color: "var(--color-on-surface)" }}>
                  {quizItems[quizIndex]?.question}
                </h4>

                <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                  {quizItems[quizIndex]?.options.map((opt, optIdx) => {
                    const isSelected = selectedOption === optIdx;
                    const isCorrect = optIdx === quizItems[quizIndex]?.correct;
                    const showFeedback = selectedOption !== null;

                    let bg = "var(--color-surface-container-high)";
                    let border = "var(--color-outline-variant)";
                    let color = "var(--color-on-surface)";

                    if (showFeedback) {
                      if (isCorrect) {
                        bg = "rgba(34, 197, 94, 0.15)";
                        border = "rgba(34, 197, 94, 0.4)";
                        color = "#86EFAC";
                      } else if (isSelected) {
                        bg = "rgba(239, 68, 68, 0.15)";
                        border = "rgba(239, 68, 68, 0.4)";
                        color = "#FCA5A5";
                      }
                    }

                    return (
                      <button
                        key={optIdx}
                        onClick={() => handleOptionSelect(optIdx)}
                        disabled={showFeedback}
                        style={{
                          padding: "10px 14px", borderRadius: "var(--radius-md)", border: `1px solid ${border}`,
                          background: bg, color, textAlign: "left", fontSize: "13px", fontFamily: "var(--font-inter)",
                          cursor: showFeedback ? "default" : "pointer", transition: "all var(--transition-fast)",
                        }}
                      >
                        {opt}
                      </button>
                    );
                  })}
                </div>

                {selectedOption !== null && (
                  <div style={{ padding: "12px 14px", borderRadius: "var(--radius-md)", background: "var(--color-surface-container)", border: "1px solid var(--color-accent-border)", marginTop: "4px" }}>
                    <p style={{ margin: "0 0 8px 0", fontSize: "12.5px", color: "var(--color-on-surface)", lineHeight: 1.4 }}>
                      💡 <strong>Explanation:</strong> {quizItems[quizIndex]?.explanation}
                    </p>
                    <button
                      onClick={handleNextQuestion}
                      style={{ padding: "6px 14px", borderRadius: "var(--radius-sm)", background: "var(--color-primary)", color: "var(--color-on-primary)", border: "none", fontSize: "12px", fontWeight: 600, cursor: "pointer", float: "right" }}
                    >
                      {quizIndex + 1 < quizItems.length ? "Next Question →" : "See Results 🏆"}
                    </button>
                  </div>
                )}
              </div>
            )
          ) : node.tool_type === "mind_map" ? (
            content.mind_map ? (
              <div style={{ padding: "8px" }}>
                <MindMapTreeNode label={content.mind_map.label} children={content.mind_map.children} />
              </div>
            ) : (
              <p style={{ color: "var(--color-on-surface-variant)", fontSize: "13px" }}>No mind map tree available.</p>
            )
          ) : (
            <div className="answer-prose" style={{ fontSize: "13.5px", lineHeight: 1.6 }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {content.summary || content.takeaways || content.report || JSON.stringify(content, null, 2)}
              </ReactMarkdown>
            </div>
          )}
        </div>

        <div style={{ marginTop: "16px", display: "flex", justifyContent: "flex-end" }}>
          <button
            onClick={onClose}
            style={{
              padding: "8px 20px", borderRadius: "var(--radius-md)", background: "var(--color-primary)",
              color: "var(--color-on-primary)", border: "none", fontFamily: "var(--font-geist)", fontWeight: 600, fontSize: "13px", cursor: "pointer",
            }}
          >
            Done
          </button>
        </div>
      </motion.div>
    </div>
  );
}

const inputStyle: React.CSSProperties = {
  padding: "10px 12px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)",
  background: "var(--color-surface-container-lowest)", color: "var(--color-on-surface)", fontSize: "13px", outline: "none", width: "100%",
};

const dropZoneStyle: React.CSSProperties = {
  display: "flex", flexDirection: "column", alignItems: "center", gap: "8px", padding: "28px 16px",
  borderRadius: "var(--radius-md)", border: "1.5px dashed var(--color-outline-variant)",
  background: "var(--color-surface-container-lowest)", color: "var(--color-on-surface-variant)",
  fontFamily: "var(--font-geist)", fontSize: "12.5px", fontWeight: 600, cursor: "pointer",
};
function primaryBtnStyle(disabled: boolean): React.CSSProperties {
  return {
    padding: "10px", borderRadius: "var(--radius-md)", border: "none", background: "var(--color-primary)",
    color: "var(--color-on-primary)", fontFamily: "var(--font-geist)", fontWeight: 600, fontSize: "13px",
    cursor: disabled ? "not-allowed" : "pointer", opacity: disabled ? 0.5 : 1, display: "flex", alignItems: "center", justifyContent: "center",
  };
}

export default function ChatPage() {
  const queryClient = useQueryClient();

  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [showAddModal, setShowAddModal] = useState(false);
  const [showLearnModal, setShowLearnModal] = useState(false);
  const [selectedNode, setSelectedNode] = useState<LearningNode | null>(null);
  const [editingTitleId, setEditingTitleId] = useState<string | null>(null);
  const [editingTitleValue, setEditingTitleValue] = useState("");
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);

  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);
  const [loadingChunk, setLoadingChunk] = useState(false);
  const [activeCitations, setActiveCitations] = useState<Message["citations"]>([]);
  const [activeCitationIndex, setActiveCitationIndex] = useState(0);
  const [sourcePanelTab, setSourcePanelTab] = useState<"overview" | "retrieved">("retrieved");

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Source Summary Modal State
  const [selectedDocSummary, setSelectedDocSummary] = useState<DocumentItem | null>(null);

  // ── Conversations (left sidebar) ─────────────────────────────────────
  const { data: conversationsData } = useQuery<{ items: Conversation[] }>({
    queryKey: ["conversations"],
    queryFn: () => apiListConversations(),
  });
  const conversations = conversationsData?.items ?? [];

  // ── Documents for open conversation (right sidebar) ───────────────
  const { data: documentsData } = useQuery<{ items: DocumentItem[] }>({
    queryKey: ["documents", conversationId],
    queryFn: () => apiListDocuments(conversationId as string),
    enabled: !!conversationId,
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? [];
      const stillBusy = items.some((d) => d.status !== "INDEXED" && d.status !== "FAILED");
      return stillBusy ? 2500 : false;
    },
  });
  const documents = documentsData?.items ?? [];

  // ── Learning Nodes for open conversation ───────────────────────────
  const { data: learningNodesData } = useQuery<{ items: LearningNode[] }>({
    queryKey: ["learning-nodes", conversationId],
    queryFn: () => apiListLearningNodes(conversationId as string),
    enabled: !!conversationId,
    refetchInterval: (query) => {
      const items = query.state.data?.items ?? [];
      const stillGenerating = items.some((n) => n.status === "generating");
      return stillGenerating ? 2000 : false;
    },
  });
  const learningNodes = learningNodesData?.items ?? [];

  const openConversation = useCallback(async (id: string) => {
    setConversationId(id);
    if (typeof window !== "undefined") {
      localStorage.setItem("last_active_conv_id", id);
      const url = new URL(window.location.href);
      url.searchParams.set("cid", id);
      window.history.replaceState(null, "", url.toString());
    }
    setMessages([]);
    setSelectedChunkId(null);
    setLoadingHistory(true);
    try {
      const res = await apiGetConversationMessages(id);
      if (res?.items) setMessages(res.items as Message[]);
    } catch (err) {
      console.error("Failed to load conversation history", err);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  const ensureConversation = useCallback(async (): Promise<string> => {
    if (conversationId) return conversationId;
    const conv = await apiCreateConversation();
    setConversationId(conv.id);
    if (typeof window !== "undefined") {
      localStorage.setItem("last_active_conv_id", conv.id);
      const url = new URL(window.location.href);
      url.searchParams.set("cid", conv.id);
      window.history.replaceState(null, "", url.toString());
    }
    queryClient.invalidateQueries({ queryKey: ["conversations"] });
    return conv.id;
  }, [conversationId, queryClient]);

  // Sync active conversation from URL ?cid=... or localStorage on initial mount
  useEffect(() => {
    if (typeof window === "undefined") return;
    const params = new URLSearchParams(window.location.search);
    const cidFromUrl = params.get("cid");
    const storedCid = localStorage.getItem("last_active_conv_id");
    const activeId = cidFromUrl || storedCid;

    if (activeId && activeId !== conversationId) {
      openConversation(activeId);
    }
  }, [openConversation]);

  const startNewConversation = useCallback(() => {
    setConversationId(null);
    if (typeof window !== "undefined") {
      localStorage.removeItem("last_active_conv_id");
      const url = new URL(window.location.href);
      url.searchParams.delete("cid");
      window.history.replaceState(null, "", url.toString());
    }
    setMessages([]);
    setSelectedChunkId(null);
  }, []);

  const handleDeleteConversation = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    try {
      await apiDeleteConversation(id);
      if (id === conversationId) startNewConversation();
      queryClient.invalidateQueries({ queryKey: ["conversations"] });
    } catch (err) {
      console.error("Failed to delete conversation", err);
    }
  };

  const commitTitleEdit = async (id: string) => {
    const title = editingTitleValue.trim();
    setEditingTitleId(null);
    if (!title) return;
    try {
      await apiUpdateConversationTitle(id, title);
      queryClient.invalidateQueries({ queryKey: ["conversations"] });
    } catch (err) {
      console.error("Failed to rename conversation", err);
    }
  };

  // ── Add Document modal handlers ──────────────────────────────────────
  const handleUploadFile = async (file: File) => {
    const id = await ensureConversation();
    await apiUploadDocument(id, file);
    queryClient.invalidateQueries({ queryKey: ["documents", id] });
  };
  const handleUploadZip = handleUploadFile;
  const handleAddVideoUrl = async (url: string) => {
    const id = await ensureConversation();
    await apiAddSource(id, "video_url", { url });
    queryClient.invalidateQueries({ queryKey: ["documents", id] });
  };
  const handleAddWebUrl = async (url: string) => {
    const id = await ensureConversation();
    await apiAddSource(id, "url", { url });
    queryClient.invalidateQueries({ queryKey: ["documents", id] });
  };
  const handleAddText = async (title: string, content: string) => {
    const id = await ensureConversation();
    await apiAddSource(id, "text", { content, title: title || undefined });
    queryClient.invalidateQueries({ queryKey: ["documents", id] });
  };
  const handleDeleteDocument = async (docId: string) => {
    if (!conversationId) return;
    try {
      await apiDeleteDocument(conversationId, docId);
      queryClient.invalidateQueries({ queryKey: ["documents", conversationId] });
    } catch (err) {
      console.error("Failed to delete document", err);
    }
  };

  const handleCreateLearningNode = async (toolType: ToolType, title: string) => {
    const id = await ensureConversation();
    await apiCreateLearningNode(id, toolType, title);
    queryClient.invalidateQueries({ queryKey: ["learning-nodes", id] });
  };

  const handleDeleteNode = async (nodeId: string) => {
    if (!conversationId) return;
    try {
      await apiDeleteLearningNode(conversationId, nodeId);
      queryClient.invalidateQueries({ queryKey: ["learning-nodes", conversationId] });
    } catch (err) {
      console.error("Failed to delete learning node", err);
    }
  };

  // Auto-scroll messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isStreaming]);

  // Fetch single chunk detail when citation opened
  useEffect(() => {
    if (!selectedChunkId || selectedChunkId.startsWith("web-")) {
      setSelectedChunk(null);
      return;
    }
    let mounted = true;
    setLoadingChunk(true);
    (async () => {
      try {
        const chunk = await apiGetChunk(selectedChunkId);
        if (mounted) setSelectedChunk(chunk);
      } catch (err) {
        console.error("Failed to fetch chunk", err);
      } finally {
        if (mounted) setLoadingChunk(false);
      }
    })();
    return () => { mounted = false; };
  }, [selectedChunkId]);

  const openCitation = useCallback((citations: Message["citations"], index: number) => {
    if (!citations || !citations[index]) return;
    setActiveCitations(citations);
    setActiveCitationIndex(index);
    setSelectedChunkId(citations[index].chunk_id);
    setSourcePanelTab("retrieved");
  }, []);

  const stepCitation = useCallback((dir: 1 | -1) => {
    if (!activeCitations || activeCitations.length === 0) return;
    const next = (activeCitationIndex + dir + activeCitations.length) % activeCitations.length;
    setActiveCitationIndex(next);
    setSelectedChunkId(activeCitations[next].chunk_id);
  }, [activeCitations, activeCitationIndex]);

  // ── Send query via SSE streaming ─────────────────────────────────────
  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isStreaming) return;

    const userText = input.trim();
    setInput("");

    let currentConvId: string;
    try {
      currentConvId = await ensureConversation();
      if (messages.length === 0) {
        const titleSummary = userText.length > 40 ? userText.slice(0, 40) + "…" : userText;
        apiUpdateConversationTitle(currentConvId, titleSummary).then(() =>
          queryClient.invalidateQueries({ queryKey: ["conversations"] })
        );
      }
    } catch (err) {
      console.error("Failed to create conversation", err);
      return;
    }

    const userMsg: Message = { id: "user-" + Date.now(), role: "user", content: userText };
    const assistantMsgId = "asst-" + Date.now();
    const initialAssistantMsg: Message = { id: assistantMsgId, role: "assistant", content: "" };
    setMessages((prev) => [...prev, userMsg, initialAssistantMsg]);
    setIsStreaming(true);

    try {
      const token = getToken();
      const response = await fetch(`${BASE}/conversations/${currentConvId}/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ content: userText }),
      });

      if (!response.ok || !response.body) throw new Error("Failed to start message stream");

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let assistantContent = "";
      let citationsBuffer: Message["citations"] = [];
      let confidenceBuffer = "normal";
      let streamBuffer = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        streamBuffer += decoder.decode(value, { stream: true });
        const lines = streamBuffer.split("\n");
        streamBuffer = lines.pop() || "";

        let streamEnded = false;
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed.startsWith("data: ")) continue;
          const raw = trimmed.replace("data: ", "").trim();

          if (raw === "[DONE]" || raw.includes("[DONE]")) {
            streamEnded = true;
            break;
          } else if (raw.startsWith("[RESULT]")) {
            try {
              const resObj = JSON.parse(raw.replace("[RESULT]", "").trim());
              if (resObj.citations && resObj.citations.length > 0) {
                citationsBuffer = resObj.citations;
              }
              if (resObj.confidence) {
                confidenceBuffer = resObj.confidence;
              }
            } catch (err) {
              console.error("Failed to parse SSE RESULT JSON:", err);
            }
          } else if (raw.startsWith("[ERROR:")) {
            assistantContent += `\n\n*Error: ${raw}*`;
          } else if (raw.startsWith("{") && raw.endsWith("}")) {
            try {
              const parsed = JSON.parse(raw);
              if (parsed.text) {
                assistantContent += parsed.text;
              }
            } catch {
              assistantContent += raw.replace(/\[DONE\]/g, "");
            }
          } else {
            const cleanText = raw.replace(/\[DONE\]/g, "");
            assistantContent += cleanText;
          }

          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === assistantMsgId
                ? { ...msg, content: assistantContent, citations: citationsBuffer, confidence: confidenceBuffer }
                : msg
            )
          );
        }
        if (streamEnded) break;
      }

      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantMsgId
            ? { ...msg, content: assistantContent, citations: citationsBuffer, confidence: confidenceBuffer }
            : msg
        )
      );
    } catch {
      setMessages((prev) => prev.map((msg) => (msg.id === assistantMsgId ? { ...msg, content: "Sorry, an error occurred while streaming the response." } : msg)));
    } finally {
      setIsStreaming(false);
    }
  };

  const hasIndexedDoc = documents.some((d) => d.status === "INDEXED");
  const hasPendingDoc = documents.length > 0 && !hasIndexedDoc;

  return (
    <div style={{ display: "flex", height: "100vh", background: "var(--color-background)", color: "var(--color-on-surface)", overflow: "hidden" }}>
      {/* ── LEFT SIDEBAR: Conversations ── */}
      <aside
        style={{
          width: isSidebarCollapsed ? "68px" : "260px",
          flexShrink: 0,
          background: "var(--color-surface-dim)",
          borderRight: "1px solid var(--color-outline-variant)",
          padding: isSidebarCollapsed ? "16px 8px" : "16px",
          display: "flex",
          flexDirection: "column",
          gap: "12px",
          overflowY: "auto",
          transition: "width 0.22s cubic-bezier(0.2, 0, 0, 1), padding 0.22s cubic-bezier(0.2, 0, 0, 1)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: isSidebarCollapsed ? "center" : "space-between" }}>
          {!isSidebarCollapsed && (
            <button
              type="button"
              onClick={startNewConversation}
              style={{
                flex: 1, display: "flex", alignItems: "center", justifyContent: "center", gap: "6px",
                padding: "9px 12px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-accent-border)",
                background: "var(--color-accent-light)", color: "var(--color-primary-fixed)",
                fontFamily: "var(--font-geist)", fontSize: "13px", fontWeight: 600, cursor: "pointer",
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>add</span>
              New chat
            </button>
          )}

          <button
            type="button"
            onClick={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
            title={isSidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
            style={{
              background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer",
              display: "flex", padding: "6px", borderRadius: "var(--radius-md)", marginLeft: isSidebarCollapsed ? 0 : "8px",
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "20px" }}>
              {isSidebarCollapsed ? "dock_to_left" : "menu_open"}
            </span>
          </button>
        </div>

        {isSidebarCollapsed && (
          <button
            type="button"
            onClick={startNewConversation}
            title="New Chat"
            style={{
              width: "100%", display: "flex", alignItems: "center", justifyContent: "center",
              padding: "10px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-accent-border)",
              background: "var(--color-accent-light)", color: "var(--color-primary-fixed)", cursor: "pointer",
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>add</span>
          </button>
        )}

        {!isSidebarCollapsed && (
          <p style={{ fontSize: "10.5px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", margin: "4px 0 0" }}>
            Conversations
          </p>
        )}

        <div style={{ flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: "4px" }}>
          {conversations.length === 0 ? (
            !isSidebarCollapsed && (
              <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "24px 10px", lineHeight: 1.6 }}>
                No conversations yet.
              </p>
            )
          ) : (
            conversations.map((conv) => {
              const isSelected = conv.id === conversationId;
              const isEditing = editingTitleId === conv.id;

              if (isSidebarCollapsed) {
                return (
                  <button
                    key={conv.id}
                    type="button"
                    onClick={() => openConversation(conv.id)}
                    title={conv.title || "Chat session"}
                    style={{
                      width: "100%", height: "42px", borderRadius: "var(--radius-md)",
                      border: `1.5px solid ${isSelected ? "var(--color-accent-border)" : "transparent"}`,
                      background: isSelected ? "var(--color-accent-light)" : "transparent",
                      color: isSelected ? "var(--color-primary)" : "var(--color-on-surface-variant)",
                      display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer",
                    }}
                  >
                    <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>
                      chat_bubble
                    </span>
                  </button>
                );
              }

              return (
                <div
                  key={conv.id}
                  onClick={() => { if (!isEditing && conv.id !== conversationId) openConversation(conv.id); }}
                  style={{
                    width: "100%", padding: "9px 10px", borderRadius: "var(--radius-md)",
                    border: `1px solid ${isSelected ? "var(--color-accent-border)" : "transparent"}`,
                    background: isSelected ? "var(--color-accent-light)" : "transparent",
                    cursor: "pointer", display: "flex", alignItems: "center", justifyContent: "space-between", gap: "6px",
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: "8px", overflow: "hidden", flex: 1 }}>
                    <span className="material-symbols-outlined" style={{ fontSize: "16px", color: isSelected ? "var(--color-primary)" : "var(--color-on-surface-variant)", flexShrink: 0 }}>
                      chat_bubble
                    </span>
                    {isEditing ? (
                      <input
                        autoFocus
                        value={editingTitleValue}
                        onChange={(e) => setEditingTitleValue(e.target.value)}
                        onClick={(e) => e.stopPropagation()}
                        onBlur={() => commitTitleEdit(conv.id)}
                        onKeyDown={(e) => { if (e.key === "Enter") commitTitleEdit(conv.id); if (e.key === "Escape") setEditingTitleId(null); }}
                        style={{ flex: 1, background: "var(--color-surface-container-lowest)", border: "1px solid var(--color-accent-border)", borderRadius: "4px", color: "var(--color-on-surface)", fontSize: "13px", padding: "2px 6px", outline: "none" }}
                      />
                    ) : (
                      <span style={{ fontSize: "13px", fontFamily: "var(--font-inter)", fontWeight: isSelected ? 600 : 400, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {conv.title || "Chat session"}
                      </span>
                    )}
                  </div>

                  {!isEditing && (
                    <div style={{ display: "flex", alignItems: "center", gap: "2px", flexShrink: 0 }}>
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); setEditingTitleId(conv.id); setEditingTitleValue(conv.title || ""); }}
                        title="Rename"
                        style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", padding: "2px" }}
                      >
                        <span className="material-symbols-outlined" style={{ fontSize: "15px" }}>edit</span>
                      </button>
                      <button
                        type="button"
                        onClick={(e) => handleDeleteConversation(e, conv.id)}
                        title="Delete"
                        style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", padding: "2px" }}
                      >
                        <span className="material-symbols-outlined" style={{ fontSize: "15px" }}>delete</span>
                      </button>
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </aside>

      {/* ── CENTER: Chat ── */}
      <main style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", background: "var(--color-background)" }}>
        <div style={{ flex: 1, padding: "28px clamp(16px, 5vw, 64px)", overflowY: "auto", display: "flex", flexDirection: "column", gap: "22px" }}>
          {loadingHistory ? (
            <div style={{ margin: "auto", display: "flex", flexDirection: "column", alignItems: "center", gap: "10px" }}>
              <Spinner size={22} color="var(--color-primary)" />
              <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>Loading conversation…</span>
            </div>
          ) : messages.length === 0 ? (
            <div style={{ margin: "auto", textAlign: "center", maxWidth: "440px", color: "var(--color-on-surface-variant)" }}>
              <div style={{ width: "56px", height: "56px", borderRadius: "var(--radius-lg)", background: "var(--color-primary-container)", display: "flex", alignItems: "center", justifyContent: "center", margin: "0 auto 16px" }}>
                <span className="material-symbols-outlined" style={{ fontSize: "26px", color: "var(--color-on-primary-container)" }}>auto_stories</span>
              </div>
              <h3 style={{ margin: "0 0 8px 0", fontFamily: "var(--font-geist)", fontSize: "17px", color: "var(--color-on-surface)" }}>
                {documents.length === 0 ? "Start by adding a source" : "Ask about your sources"}
              </h3>
              <p style={{ fontSize: "13.5px", lineHeight: 1.6 }}>
                {documents.length === 0
                  ? "Use \u201cAdd\u201d on the right — a PDF, a YouTube link, a ZIP of files, or pasted text. Answers ground strictly in your sources."
                  : hasPendingDoc && !hasIndexedDoc
                    ? "Your source is indexing — ask now or wait a moment for grounded citations."
                    : "Ask a question — answers are grounded in your uploaded sources."}
              </p>
              {documents.find((d) => d.status === "INDEXED" && d.ai_overview) && (
                <div style={{ marginTop: "16px", padding: "12px 16px", borderRadius: "var(--radius-md)", background: "var(--color-surface-container-high)", border: "1px solid var(--color-accent-border)", textAlign: "left" }}>
                  <span style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-tertiary)" }}>
                    ✨ Source Overview
                  </span>
                  <p style={{ margin: "6px 0 0 0", fontSize: "13px", lineHeight: 1.5, color: "var(--color-on-surface)" }}>
                    {documents.find((d) => d.status === "INDEXED" && d.ai_overview)?.ai_overview}
                  </p>
                </div>
              )}
            </div>
          ) : (
            messages.map((m) => (
              <div key={m.id} className="fade-up" style={{ display: "flex", flexDirection: "column", alignItems: m.role === "user" ? "flex-end" : "flex-start" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "6px" }}>
                  <span style={{ fontFamily: "var(--font-geist)", fontSize: "10.5px", fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", padding: "0 4px" }}>
                    {m.role === "user" ? "You" : "Assistant"}
                  </span>
                  {m.confidence === "web_search" && (
                    <span style={{ fontSize: "10px", padding: "2px 6px", borderRadius: "var(--radius-full)", background: "rgba(236, 72, 153, 0.15)", color: "#EC4899", border: "1px solid rgba(236, 72, 153, 0.3)", fontWeight: 700 }}>
                      🌐 Live Web Search
                    </span>
                  )}
                </div>
                <div style={{ maxWidth: "88%", padding: m.role === "user" ? "12px 16px" : "18px 20px", borderRadius: "var(--radius-lg)", background: m.role === "user" ? "var(--color-surface-container-high)" : "var(--color-surface-container-low)", border: "1px solid var(--color-outline-variant)" }}>
                  {m.role === "assistant" ? (
                    <div className="answer-prose">
                      <ReactMarkdown
                        remarkPlugins={[remarkGfm]}
                        components={{
                          p: ({ children }) => <p style={{ margin: "0 0 14px 0", lineHeight: "1.75", fontSize: "14.5px", color: "var(--color-on-surface)" }}>{children}</p>,
                          h1: ({ children }) => <h1 style={{ marginTop: "24px", marginBottom: "12px", fontSize: "19px", fontWeight: 700, color: "var(--color-on-surface)", fontFamily: "var(--font-geist)" }}>{children}</h1>,
                          h2: ({ children }) => <h2 style={{ marginTop: "20px", marginBottom: "10px", fontSize: "17px", fontWeight: 700, color: "var(--color-on-surface)", fontFamily: "var(--font-geist)" }}>{children}</h2>,
                          h3: ({ children }) => <h3 style={{ marginTop: "16px", marginBottom: "8px", fontSize: "15px", fontWeight: 700, color: "var(--color-on-surface)", fontFamily: "var(--font-geist)" }}>{children}</h3>,
                          ul: ({ children }) => <ul style={{ margin: "10px 0 16px 20px", padding: 0, display: "flex", flexDirection: "column", gap: "8px" }}>{children}</ul>,
                          ol: ({ children }) => <ol style={{ margin: "10px 0 16px 20px", padding: 0, display: "flex", flexDirection: "column", gap: "8px" }}>{children}</ol>,
                          li: ({ children }) => <li style={{ lineHeight: "1.65", color: "var(--color-on-surface)", fontSize: "14px" }}>{children}</li>,
                          strong: ({ children }) => <strong style={{ color: "var(--color-primary-fixed)", fontWeight: 600 }}>{children}</strong>,
                          code: ({ children }) => <code style={{ background: "var(--color-surface-container-high)", padding: "2px 6px", borderRadius: "4px", fontSize: "13px", fontFamily: "var(--font-mono)" }}>{children}</code>,
                          a: ({ href, children }) => <a href={href} target="_blank" rel="noopener noreferrer" style={{ color: "var(--color-primary)", textDecoration: "underline" }}>{children}</a>,
                        }}
                      >
                        {m.content || (isStreaming ? "…" : "")}
                      </ReactMarkdown>
                    </div>
                  ) : (
                    <div style={{ fontSize: "14px", lineHeight: 1.6, fontFamily: "var(--font-inter)" }}>
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content}</ReactMarkdown>
                    </div>
                  )}

                  {m.citations && m.citations.length > 0 && (
                    <div style={{ marginTop: "16px", paddingTop: "14px", borderTop: "1px solid var(--color-outline-variant)" }}>
                      <p style={{ fontSize: "10.5px", fontFamily: "var(--font-geist)", fontWeight: 700, color: "var(--color-on-surface-variant)", letterSpacing: "0.06em", textTransform: "uppercase", margin: "0 0 9px 0" }}>
                        Sources ({m.citations.length})
                      </p>
                      <div style={{ display: "flex", gap: "8px", flexWrap: "wrap" }}>
                        {m.citations.map((c, i) => {
                          const isActive = selectedChunkId === c.chunk_id && activeCitations === m.citations;
                          const isWeb = c.title?.startsWith("🌐") || c.document_id?.startsWith("http");

                          if (isWeb && c.document_id?.startsWith("http")) {
                            return (
                              <a
                                key={c.chunk_id + "-" + i}
                                href={c.document_id}
                                target="_blank"
                                rel="noopener noreferrer"
                                style={{
                                  display: "inline-flex", alignItems: "center", gap: "6px", padding: "6px 11px",
                                  background: "rgba(236, 72, 153, 0.12)", border: "1px solid rgba(236, 72, 153, 0.3)",
                                  borderRadius: "var(--radius-md)", color: "#EC4899", textDecoration: "none",
                                  fontSize: "11.5px", fontFamily: "var(--font-geist)", fontWeight: 600,
                                }}
                              >
                                <span>🌐 {c.title?.replace("🌐 ", "")} ↗</span>
                              </a>
                            );
                          }

                          const colorScheme = getSourceColor(undefined);

                          return (
                            <button
                              key={c.chunk_id + "-" + i}
                              type="button"
                              onClick={() => openCitation(m.citations, i)}
                              style={{
                                display: "flex", alignItems: "center", gap: "8px", padding: "7px 11px",
                                background: isActive ? "var(--color-accent-light)" : "var(--color-surface-container)",
                                border: `1.5px solid ${isActive ? "var(--color-accent-border)" : colorScheme.border}`,
                                borderRadius: "var(--radius-md)", cursor: "pointer", color: "var(--color-on-surface)",
                                textAlign: "left", maxWidth: "220px",
                              }}
                            >
                              <span className="citation-tab" style={{ width: "18px", height: "16px", fontSize: "9.5px", flexShrink: 0, background: colorScheme.color, color: "#fff" }}>{i + 1}</span>
                              <div style={{ overflow: "hidden" }}>
                                <p style={{ margin: 0, fontSize: "11.5px", fontFamily: "var(--font-geist)", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                  {c.title ? c.title.replace(/^\d+[\.\-_]\s*/, "").replace(/\.(vtt|srt|mp4|pdf|docx|txt)$/i, "") : `Source ${i + 1}`}
                                </p>
                                {c.start_timestamp != null && (
                                  <span style={{ fontSize: "10px", fontFamily: "var(--font-mono)", color: "var(--color-tertiary)", fontWeight: 600 }}>
                                    {formatTime(c.start_timestamp)}
                                  </span>
                                )}
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* ── Suggested Questions ── */}
        {messages.length === 0 && documents.some((d) => d.status === "INDEXED") && (
          <div style={{ padding: "0 clamp(16px, 5vw, 64px) 10px", display: "flex", gap: "8px", flexWrap: "wrap", justifyContent: "center" }}>
            {(() => {
              const indexedDocs = documents.filter((d) => d.status === "INDEXED");
              const aiQuestions = indexedDocs.flatMap((d) => d.ai_questions || []);
              if (aiQuestions.length > 0) {
                return aiQuestions.slice(0, 4);
              }
              return indexedDocs.flatMap((d) => {
                const title = cleanFilename(d.original_filename);
                return [
                  `What are the key concepts in ${title}?`,
                  `Summarize the main takeaways from ${title}`,
                ];
              }).slice(0, 4);
            })().map((q, idx) => (
              <button
                key={idx}
                type="button"
                onClick={() => setInput(q)}
                style={{
                  padding: "6px 14px", borderRadius: "var(--radius-full)", background: "var(--color-surface-container-high)",
                  border: "1px solid var(--color-accent-border)", color: "var(--color-on-surface)", fontSize: "12px",
                  fontFamily: "var(--font-geist)", cursor: "pointer", transition: "all var(--transition-fast)",
                }}
              >
                💡 {q}
              </button>
            ))}
          </div>
        )}

        <form onSubmit={handleSend} style={{ padding: "16px clamp(16px, 5vw, 64px) 20px", borderTop: "1px solid var(--color-outline-variant)", display: "flex", gap: "10px", background: "var(--color-surface-dim)" }}>
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask a question about your sources…"
            disabled={isStreaming}
            className="input-glow"
            style={{ flex: 1, padding: "12px 16px", borderRadius: "var(--radius-md)", background: "var(--color-surface-container-lowest)", border: "1px solid var(--color-outline-variant)", color: "var(--color-on-surface)", fontFamily: "var(--font-inter)", fontSize: "14px", outline: "none" }}
          />
          <button
            type="submit"
            disabled={!input.trim() || isStreaming}
            style={{ padding: "0 22px", borderRadius: "var(--radius-md)", background: "var(--color-primary)", color: "var(--color-on-primary)", border: "none", fontFamily: "var(--font-geist)", fontWeight: 600, fontSize: "13.5px", cursor: "pointer", opacity: !input.trim() || isStreaming ? 0.5 : 1 }}
          >
            {isStreaming ? <Spinner size={16} color="var(--color-on-primary)" /> : "Send"}
          </button>
        </form>
      </main>

      {/* ── RIGHT SIDEBAR: Sources & Learning Tools ── */}
      <aside style={{ width: "300px", flexShrink: 0, background: "var(--color-surface-dim)", borderLeft: "1px solid var(--color-outline-variant)", display: "flex", flexDirection: "column" }}>
        {/* Sources Header */}
        <div style={{ padding: "16px", borderBottom: "1px solid var(--color-outline-variant)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <h3 style={{ margin: 0, fontFamily: "var(--font-geist)", fontSize: "13px", fontWeight: 700, letterSpacing: "0.03em", textTransform: "uppercase", color: "var(--color-on-surface-variant)" }}>
            Sources {documents.length > 0 && `(${documents.length})`}
          </h3>
          <button
            type="button"
            onClick={() => setShowAddModal(true)}
            style={{ display: "flex", alignItems: "center", gap: "4px", padding: "6px 10px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-accent-border)", background: "var(--color-accent-light)", color: "var(--color-primary-fixed)", fontFamily: "var(--font-geist)", fontSize: "11.5px", fontWeight: 600, cursor: "pointer" }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "14px" }}>add</span>
            Add
          </button>
        </div>

        {/* Sources List */}
        <div style={{ flex: 1, overflowY: "auto", padding: "12px", display: "flex", flexDirection: "column", gap: "6px" }}>
          {!conversationId ? (
            <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "24px 10px", lineHeight: 1.6 }}>
              Start a chat or add a source to begin a new one.
            </p>
          ) : documents.length === 0 ? (
            <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "24px 10px", lineHeight: 1.6 }}>
              No sources added to this conversation yet.
            </p>
          ) : (
            documents.map((d) => {
              const scheme = getSourceColor(d.source_type);
              return (
                <div key={d.id} onClick={() => setSelectedDocSummary(d)} style={{ cursor: "pointer", display: "flex", alignItems: "flex-start", gap: "8px", padding: "9px 10px", borderRadius: "var(--radius-md)", border: `1px solid ${scheme.border}`, background: scheme.bg }}>
                  <span style={{ marginTop: "1px", color: scheme.color, flexShrink: 0 }}>
                    <SourceTypeIcon kind={d.source_type} size={16} />
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p style={{ margin: "0 0 4px 0", fontSize: "12.5px", fontFamily: "var(--font-inter)", fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={d.original_filename}>
                      {d.original_filename}
                    </p>
                    <StatusPill status={d.status} />
                  </div>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteDocument(d.id);
                    }}
                    title="Remove source"
                    style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", flexShrink: 0, padding: "2px" }}
                  >
                    <span className="material-symbols-outlined" style={{ fontSize: "15px" }}>close</span>
                  </button>
                </div>
              );
            })
          )}
        </div>

        {/* ── Study Tools Section ── */}
        <div style={{ padding: "14px 16px 10px", borderTop: "1px solid var(--color-outline-variant)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <h3 style={{ margin: 0, fontFamily: "var(--font-geist)", fontSize: "12px", fontWeight: 700, letterSpacing: "0.03em", textTransform: "uppercase", color: "var(--color-on-surface-variant)" }}>
            Study Tools {learningNodes.length > 0 && `(${learningNodes.length})`}
          </h3>
          <button
            type="button"
            onClick={() => setShowLearnModal(true)}
            disabled={!hasIndexedDoc}
            style={{
              display: "flex", alignItems: "center", gap: "4px", padding: "5px 9px", borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-accent-border)", background: "var(--color-accent-light)", color: "var(--color-primary-fixed)",
              fontFamily: "var(--font-geist)", fontSize: "11px", fontWeight: 600, cursor: hasIndexedDoc ? "pointer" : "not-allowed", opacity: hasIndexedDoc ? 1 : 0.5,
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "14px" }}>school</span>
            Learn
          </button>
        </div>

        <div style={{ padding: "0 12px 12px", display: "flex", flexDirection: "column", gap: "6px", maxHeight: "180px", overflowY: "auto" }}>
          {learningNodes.length === 0 ? (
            <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "11.5px", padding: "12px 6px", margin: 0 }}>
              {hasIndexedDoc ? "Click \u201cLearn\u201d to generate flashcards, quizzes & summaries." : "Add a source to unlock study tools."}
            </p>
          ) : (
            learningNodes.map((n) => (
              <div
                key={n.id}
                onClick={() => setSelectedNode(n)}
                style={{
                  display: "flex", alignItems: "center", justifyContent: "space-between", gap: "8px",
                  padding: "8px 10px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)",
                  background: "var(--color-surface-container-low)", cursor: "pointer",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: "8px", overflow: "hidden" }}>
                  <span className="material-symbols-outlined" style={{ fontSize: "16px", color: "var(--color-tertiary)", flexShrink: 0 }}>
                    {n.tool_type === "flashcards" ? "style" : n.tool_type === "quiz" ? "quiz" : n.tool_type === "mind_map" ? "account_tree" : "description"}
                  </span>
                  <span style={{ fontSize: "12px", fontFamily: "var(--font-inter)", fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {n.title}
                  </span>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: "4px", flexShrink: 0 }}>
                  {n.status === "generating" && <Spinner size={10} color="var(--color-primary)" />}
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeleteNode(n.id); }}
                    style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", padding: "2px" }}
                  >
                    <span className="material-symbols-outlined" style={{ fontSize: "14px" }}>close</span>
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </aside>

      {/* ── Add Document modal ── */}
      <AnimatePresence>
        {showAddModal && (
          <AddDocumentModal
            onClose={() => setShowAddModal(false)}
            onUploadFile={handleUploadFile}
            onUploadZip={handleUploadZip}
            onAddVideoUrl={handleAddVideoUrl}
            onAddWebUrl={handleAddWebUrl}
            onAddText={handleAddText}
          />
        )}
      </AnimatePresence>

      {/* ── Generate Learning Tool Modal ── */}
      <AnimatePresence>
        {showLearnModal && (
          <GenerateLearningToolModal
            onClose={() => setShowLearnModal(false)}
            onGenerate={handleCreateLearningNode}
          />
        )}
      </AnimatePresence>

      {/* ── Learning Node Viewer Modal ── */}
      <AnimatePresence>
        {selectedNode && (
          <LearningNodeViewerModal
            node={selectedNode}
            onClose={() => setSelectedNode(null)}
          />
        )}
      </AnimatePresence>

      {/* ── Citation / source detail panel ── */}
      <AnimatePresence>
        {selectedChunkId && (
          <motion.div
            initial={{ x: 380, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: 380, opacity: 0 }}
            transition={{ type: "spring", damping: 28, stiffness: 260 }}
            style={{ position: "fixed", right: 0, top: 0, width: "380px", height: "100vh", background: "var(--color-surface-dim)", borderLeft: "1px solid var(--color-outline-variant)", zIndex: 90, display: "flex", flexDirection: "column" }}
          >
            <div style={{ padding: "16px 18px 12px", borderBottom: "1px solid var(--color-outline-variant)", display: "flex", flexDirection: "column", gap: "10px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <h3 style={{ fontFamily: "var(--font-geist)", fontSize: "13px", fontWeight: 700, letterSpacing: "0.03em", textTransform: "uppercase", margin: 0, color: "var(--color-on-surface-variant)" }}>
                  Source material
                </h3>
                <button onClick={() => setSelectedChunkId(null)} aria-label="Close" style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", padding: "2px" }}>
                  <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>close</span>
                </button>
              </div>

              {activeCitations && activeCitations.length > 1 && (
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                  <button type="button" onClick={() => stepCitation(-1)} style={{ background: "none", border: "none", color: "var(--color-on-surface)", cursor: "pointer", display: "flex" }} aria-label="Previous">
                    <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>chevron_left</span>
                  </button>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "11.5px", color: "var(--color-on-surface-variant)" }}>
                    Source {activeCitationIndex + 1} of {activeCitations.length}
                  </span>
                  <button type="button" onClick={() => stepCitation(1)} style={{ background: "none", border: "none", color: "var(--color-on-surface)", cursor: "pointer", display: "flex" }} aria-label="Next">
                    <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>chevron_right</span>
                  </button>
                </div>
              )}

              <div style={{ display: "flex", background: "var(--color-surface-container)", border: "1px solid var(--color-outline-variant)", borderRadius: "var(--radius-md)", padding: "3px", gap: "2px" }}>
                {(["overview", "retrieved"] as const).map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    onClick={() => setSourcePanelTab(tab)}
                    style={{ flex: 1, padding: "6px 8px", border: "none", borderRadius: "6px", cursor: "pointer", fontFamily: "var(--font-geist)", fontSize: "12px", fontWeight: 600, background: sourcePanelTab === tab ? "var(--color-surface-container-highest)" : "transparent", color: sourcePanelTab === tab ? "var(--color-on-surface)" : "var(--color-on-surface-variant)" }}
                  >
                    {tab === "overview" ? "Overview" : "Retrieved content"}
                  </button>
                ))}
              </div>
            </div>

            <div style={{ flex: 1, overflowY: "auto", padding: "18px" }}>
              {loadingChunk ? (
                <div style={{ margin: "40px auto", display: "flex", justifyContent: "center" }}>
                  <Spinner size={22} color="var(--color-primary)" />
                </div>
              ) : selectedChunk ? (
                <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
                  {(() => {
                    const ytId = extractYouTubeId(selectedChunk);
                    const startSec = parseTimestampInSeconds(selectedChunk);
                    const isVideo = selectedChunk.source_type === "video" || !!ytId;

                    let displayTitle = selectedChunk.document_name;
                    if (!displayTitle || displayTitle === "bundle") {
                      displayTitle = ytId ? `YouTube Video (${ytId})` : selectedChunk.source_url || "Document source";
                    }

                    return (
                      <>
                        {ytId && (
                          <div style={{ borderRadius: "var(--radius-md)", overflow: "hidden", border: "1px solid var(--color-outline-variant)", background: "#000", marginBottom: "6px", position: "relative" }}>
                            <iframe
                              width="100%"
                              height="200"
                              src={`https://www.youtube.com/embed/${ytId}?start=${startSec}`}
                              title={displayTitle}
                              frameBorder="0"
                              allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                              allowFullScreen
                            />
                          </div>
                        )}

                        <div
                          style={{
                            padding: "14px", background: "var(--color-surface-container-low)", borderRadius: "var(--radius-md)",
                            border: "1px solid var(--color-outline-variant)", display: "flex", gap: "10px", alignItems: "flex-start",
                          }}
                        >
                          <div
                            style={{
                              width: "32px", height: "32px", borderRadius: "var(--radius-sm)",
                              background: isVideo ? "rgba(255, 0, 0, 0.15)" : "var(--color-accent-light)",
                              display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0,
                            }}
                          >
                            <span className="material-symbols-outlined" style={{ fontSize: "18px", color: isVideo ? "#ff4e4e" : "var(--color-primary)" }}>
                              {isVideo ? "smart_display" : "description"}
                            </span>
                          </div>
                          <div style={{ minWidth: 0, flex: 1 }}>
                            <h4 style={{ fontFamily: "var(--font-geist)", fontSize: "13.5px", fontWeight: 600, margin: "0 0 4px 0", color: "var(--color-on-surface)", lineHeight: 1.4, wordBreak: "break-word" }}>
                              {displayTitle}
                            </h4>

                            {selectedChunk.source_url && (
                              <p style={{ margin: "0 0 6px 0", fontSize: "11.5px", fontFamily: "var(--font-mono)", color: "var(--color-on-surface-variant)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                {selectedChunk.source_url}
                              </p>
                            )}

                            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: "8px", marginTop: "4px" }}>
                              {startSec > 0 || selectedChunk.start_timestamp != null ? (
                                <span style={{ fontSize: "11px", fontFamily: "var(--font-mono)", fontWeight: 600, color: "var(--color-tertiary)" }}>
                                  Timeline segment: ⏱ {formatTime(startSec || selectedChunk.start_timestamp || 0)}
                                  {selectedChunk.end_timestamp != null && ` – ${formatTime(selectedChunk.end_timestamp)}`}
                                </span>
                              ) : selectedChunk.page_number != null ? (
                                <span style={{ fontSize: "11px", fontFamily: "var(--font-mono)", color: "var(--color-tertiary)" }}>
                                  Page {selectedChunk.page_number}
                                </span>
                              ) : null}

                              {ytId && (
                                <a
                                  href={`https://www.youtube.com/watch?v=${ytId}&t=${startSec}s`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 600, color: "var(--color-primary)", textDecoration: "none", display: "inline-flex", alignItems: "center", gap: "3px" }}
                                >
                                  Open in YouTube ↗
                                </a>
                              )}
                            </div>
                          </div>
                        </div>

                        {selectedChunk.content && (
                          <div style={{ marginTop: "12px", padding: "14px", borderRadius: "var(--radius-md)", background: "var(--color-surface-container-lowest)", border: "1px solid var(--color-outline-variant)" }}>
                            <p style={{ margin: "0 0 8px 0", fontSize: "10.5px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-tertiary)" }}>
                              Retrieved Excerpt
                            </p>
                            <div className="answer-prose" style={{ fontSize: "13px", lineHeight: 1.6, color: "var(--color-on-surface)" }}>
                              <ReactMarkdown remarkPlugins={[remarkGfm]}>{selectedChunk.content}</ReactMarkdown>
                            </div>
                          </div>
                        )}
                      </>
                    );
                  })()}
                </div>
              ) : null}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* ── NotebookLM 1:1 Source Guide Modal ── */}
      <AnimatePresence>
        {selectedDocSummary && (
          <div
            onClick={() => setSelectedDocSummary(null)}
            style={{
              position: "fixed", inset: 0, background: "rgba(0,0,0,0.7)", zIndex: 110,
              display: "flex", alignItems: "center", justifyContent: "center", padding: "20px",
            }}
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              onClick={(e) => e.stopPropagation()}
              style={{
                width: "100%", maxWidth: "620px", background: "var(--color-surface-container-low)",
                border: "1px solid var(--color-outline-variant)", borderRadius: "var(--radius-xl)",
                padding: "24px", boxShadow: "var(--shadow-lg)",
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "16px" }}>
                <div>
                  <span style={{ fontSize: "11px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-tertiary)" }}>
                    ✨ Source guide
                  </span>
                  <h3 style={{ margin: "4px 0 0 0", fontFamily: "var(--font-geist)", fontSize: "17px", fontWeight: 700, color: "var(--color-on-surface)" }}>
                    {selectedDocSummary.original_filename}
                  </h3>
                </div>
                <button onClick={() => setSelectedDocSummary(null)} style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer" }}>
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>

              <div style={{ background: "var(--color-surface-container-lowest)", padding: "18px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)", maxHeight: "360px", overflowY: "auto" }}>
                <p style={{ fontSize: "13.5px", lineHeight: 1.7, margin: 0, color: "var(--color-on-surface)", whiteSpace: "pre-wrap" }}>
                  {selectedDocSummary.ai_summary || selectedDocSummary.summary || "Generating AI Source Guide summary..."}
                </p>
              </div>

              <div style={{ marginTop: "18px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <StatusPill status={selectedDocSummary.status} />
                <button
                  type="button"
                  onClick={() => setSelectedDocSummary(null)}
                  style={{
                    padding: "8px 20px", borderRadius: "var(--radius-md)", background: "var(--color-primary)",
                    color: "var(--color-on-primary)", border: "none", fontFamily: "var(--font-geist)", fontWeight: 600, fontSize: "13px", cursor: "pointer",
                  }}
                >
                  Done
                </button>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}

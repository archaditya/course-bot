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
  getToken,
  type Conversation,
  type ChunkDetail,
  type DocumentItem,
  type DocumentStatus,
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

// ── Add Document modal: 4 tabs — File, YouTube URL, ZIP, Text ─────────────
function AddDocumentModal({
  onClose,
  onUploadFile,
  onUploadZip,
  onAddVideoUrl,
  onAddText,
}: {
  onClose: () => void;
  onUploadFile: (file: File) => Promise<void>;
  onUploadZip: (file: File) => Promise<void>;
  onAddVideoUrl: (url: string) => Promise<void>;
  onAddText: (title: string, content: string) => Promise<void>;
}) {
  const [tab, setTab] = useState<"file" | "video" | "zip" | "text">("file");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [videoUrl, setVideoUrl] = useState("");
  const [textTitle, setTextTitle] = useState("");
  const [textContent, setTextContent] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const zipInputRef = useRef<HTMLInputElement>(null);

  const tabs = [
    { key: "file" as const, label: "PDF / Doc", icon: "description" },
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
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.6)",
        zIndex: 100,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "20px",
      }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        transition={{ duration: 0.18 }}
        onClick={(e) => e.stopPropagation()}
        style={{
          width: "100%",
          maxWidth: "480px",
          background: "var(--color-surface-container-low)",
          border: "1px solid var(--color-outline-variant)",
          borderRadius: "var(--radius-xl)",
          boxShadow: "var(--shadow-lg)",
          overflow: "hidden",
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
                flex: 1,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: "4px",
                padding: "10px 4px",
                borderRadius: "var(--radius-md)",
                border: `1px solid ${tab === t.key ? "var(--color-accent-border)" : "var(--color-outline-variant)"}`,
                background: tab === t.key ? "var(--color-accent-light)" : "transparent",
                color: tab === t.key ? "var(--color-primary-fixed)" : "var(--color-on-surface-variant)",
                cursor: "pointer",
                fontFamily: "var(--font-geist)",
                fontSize: "10.5px",
                fontWeight: 600,
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
              <button
                onClick={() => fileInputRef.current?.click()}
                disabled={busy}
                style={dropZoneStyle}
              >
                <span className="material-symbols-outlined" style={{ fontSize: "26px", color: "var(--color-primary)" }}>upload_file</span>
                Click to choose a file
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
              <textarea
                value={textContent}
                onChange={(e) => setTextContent(e.target.value)}
                placeholder="Paste or type text…"
                className="input-glow"
                rows={5}
                style={{ ...inputStyle, resize: "vertical", fontFamily: "var(--font-inter)" }}
              />
              <button
                onClick={() => textContent.trim() && guarded(() => onAddText(textTitle.trim(), textContent.trim()))}
                disabled={busy || !textContent.trim()}
                style={primaryBtnStyle(busy || !textContent.trim())}
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

const dropZoneStyle: React.CSSProperties = {
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  gap: "8px",
  padding: "28px 16px",
  borderRadius: "var(--radius-md)",
  border: "1.5px dashed var(--color-outline-variant)",
  background: "var(--color-surface-container-lowest)",
  color: "var(--color-on-surface-variant)",
  fontFamily: "var(--font-geist)",
  fontSize: "12.5px",
  fontWeight: 600,
  cursor: "pointer",
};
const inputStyle: React.CSSProperties = {
  padding: "10px 12px",
  borderRadius: "var(--radius-md)",
  border: "1px solid var(--color-outline-variant)",
  background: "var(--color-surface-container-lowest)",
  color: "var(--color-on-surface)",
  fontSize: "13px",
  outline: "none",
  width: "100%",
};
function primaryBtnStyle(disabled: boolean): React.CSSProperties {
  return {
    padding: "10px",
    borderRadius: "var(--radius-md)",
    border: "none",
    background: "var(--color-primary)",
    color: "var(--color-on-primary)",
    fontFamily: "var(--font-geist)",
    fontWeight: 600,
    fontSize: "13px",
    cursor: disabled ? "not-allowed" : "pointer",
    opacity: disabled ? 0.5 : 1,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
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
  const [editingTitleId, setEditingTitleId] = useState<string | null>(null);
  const [editingTitleValue, setEditingTitleValue] = useState("");

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

  // ── Documents for the open conversation (right sidebar) — polls while
  //    anything is still indexing so status pills update live ──────────
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

  const ensureConversation = useCallback(async (): Promise<string> => {
    if (conversationId) return conversationId;
    const conv = await apiCreateConversation();
    setConversationId(conv.id);
    queryClient.invalidateQueries({ queryKey: ["conversations"] });
    return conv.id;
  }, [conversationId, queryClient]);

  const openConversation = useCallback(async (id: string) => {
    setConversationId(id);
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

  const startNewConversation = useCallback(() => {
    setConversationId(null);
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
  const handleUploadZip = handleUploadFile; // same endpoint, backend auto-detects .zip
  const handleAddVideoUrl = async (url: string) => {
    const id = await ensureConversation();
    await apiAddSource(id, "video_url", { url });
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

  // Auto-scroll messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isStreaming]);

  // Fetch single chunk detail when a citation is opened
  useEffect(() => {
    if (!selectedChunkId) {
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

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        const chunkText = decoder.decode(value, { stream: true });
        const lines = chunkText.split("\n");

        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const raw = line.replace("data: ", "").trim();

          if (raw === "[DONE]") {
            break;
          } else if (raw.startsWith("[RESULT]")) {
            try {
              const resObj = JSON.parse(raw.replace("[RESULT]", "").trim());
              if (resObj.citations && resObj.citations.length > 0) {
                citationsBuffer = resObj.citations;
                setMessages((prev) =>
                  prev.map((msg) => (msg.id === assistantMsgId ? { ...msg, citations: resObj.citations } : msg))
                );
              }
            } catch { /* ignore parse error */ }
          } else if (raw.startsWith("[ERROR:")) {
            assistantContent += `\n\n*Error: ${raw}*`;
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantMsgId ? { ...msg, content: assistantContent } : msg))
            );
          } else {
            assistantContent += raw;
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantMsgId ? { ...msg, content: assistantContent, citations: citationsBuffer } : msg))
            );
          }
        }
      }

      setMessages((prev) => prev.map((msg) => (msg.id === assistantMsgId ? { ...msg, content: assistantContent, citations: citationsBuffer } : msg)));
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
      <aside style={{ width: "260px", flexShrink: 0, background: "var(--color-surface-dim)", borderRight: "1px solid var(--color-outline-variant)", padding: "16px", display: "flex", flexDirection: "column", gap: "12px", overflowY: "auto" }}>
        <button
          type="button"
          onClick={startNewConversation}
          style={{
            width: "100%", display: "flex", alignItems: "center", justifyContent: "center", gap: "6px",
            padding: "9px 12px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-accent-border)",
            background: "var(--color-accent-light)", color: "var(--color-primary-fixed)",
            fontFamily: "var(--font-geist)", fontSize: "13px", fontWeight: 600, cursor: "pointer",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>add</span>
          New chat
        </button>

        <p style={{ fontSize: "10.5px", fontFamily: "var(--font-geist)", fontWeight: 700, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", margin: "4px 0 0" }}>
          Conversations
        </p>

        <div style={{ flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: "4px" }}>
          {conversations.length === 0 ? (
            <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "24px 10px", lineHeight: 1.6 }}>
              No conversations yet.
            </p>
          ) : (
            conversations.map((conv) => {
              const isSelected = conv.id === conversationId;
              const isEditing = editingTitleId === conv.id;
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
                  ? "Use \u201cAdd\u201d on the right — a PDF, a YouTube link, a ZIP of files, or pasted text. This chat only ever grounds itself in what you add here."
                  : hasPendingDoc && !hasIndexedDoc
                    ? "Your source is still indexing — you can ask now, or wait a moment for grounded citations."
                    : "Ask a question — answers are grounded in the sources you've added to this conversation."}
              </p>
            </div>
          ) : (
            messages.map((m) => (
              <div key={m.id} className="fade-up" style={{ display: "flex", flexDirection: "column", alignItems: m.role === "user" ? "flex-end" : "flex-start" }}>
                <span style={{ fontFamily: "var(--font-geist)", fontSize: "10.5px", fontWeight: 600, letterSpacing: "0.06em", textTransform: "uppercase", color: "var(--color-on-surface-variant)", marginBottom: "6px", padding: "0 4px" }}>
                  {m.role === "user" ? "You" : "Assistant"}
                </span>
                <div style={{ maxWidth: "88%", padding: m.role === "user" ? "12px 16px" : "18px 20px", borderRadius: "var(--radius-lg)", background: m.role === "user" ? "var(--color-surface-container-high)" : "var(--color-surface-container-low)", border: "1px solid var(--color-outline-variant)" }}>
                  {m.role === "assistant" ? (
                    <div className="answer-prose">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content || (isStreaming ? "…" : "")}</ReactMarkdown>
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
                          return (
                            <button
                              key={c.chunk_id + "-" + i}
                              type="button"
                              onClick={() => openCitation(m.citations, i)}
                              style={{
                                display: "flex", alignItems: "center", gap: "8px", padding: "7px 11px",
                                background: isActive ? "var(--color-accent-light)" : "var(--color-surface-container)",
                                border: `1px solid ${isActive ? "var(--color-accent-border)" : "var(--color-outline-variant)"}`,
                                borderRadius: "var(--radius-md)", cursor: "pointer", color: "var(--color-on-surface)",
                                textAlign: "left", maxWidth: "220px",
                              }}
                            >
                              <span className="citation-tab" style={{ width: "18px", height: "16px", fontSize: "9.5px", flexShrink: 0 }}>{i + 1}</span>
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

        {/* ── Dynamic NotebookLM Suggested Questions from Indexed Documents ── */}
        {messages.length === 0 && documents.some((d) => d.status === "INDEXED") && (
          <div style={{ padding: "0 clamp(16px, 5vw, 64px) 10px", display: "flex", gap: "8px", flexWrap: "wrap", justifyContent: "center" }}>
            {documents
              .filter((d) => d.status === "INDEXED")
              .flatMap((d) => {
                const title = cleanFilename(d.original_filename);
                return [
                  `What are the key concepts in ${title}?`,
                  `Summarize the main takeaways from ${title}`,
                ];
              })
              .slice(0, 3)
              .map((q, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() => setInput(q)}
                  style={{
                    padding: "6px 14px",
                    borderRadius: "var(--radius-full)",
                    background: "var(--color-surface-container-high)",
                    border: "1px solid var(--color-accent-border)",
                    color: "var(--color-on-surface)",
                    fontSize: "12px",
                    fontFamily: "var(--font-geist)",
                    cursor: "pointer",
                    transition: "all var(--transition-fast)",
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

      {/* ── RIGHT SIDEBAR: Sources for this conversation ── */}
      <aside style={{ width: "300px", flexShrink: 0, background: "var(--color-surface-dim)", borderLeft: "1px solid var(--color-outline-variant)", display: "flex", flexDirection: "column" }}>
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
            documents.map((d) => (
              <div key={d.id} onClick={() => setSelectedDocSummary(d)} style={{ cursor: "pointer", display: "flex", alignItems: "flex-start", gap: "8px", padding: "9px 10px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)", background: "var(--color-surface-container-low)" }}>
                <span style={{ marginTop: "1px", color: "var(--color-primary)", flexShrink: 0 }}>
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
                  onClick={() => handleDeleteDocument(d.id)}
                  title="Remove source"
                  style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer", display: "flex", flexShrink: 0, padding: "2px" }}
                >
                  <span className="material-symbols-outlined" style={{ fontSize: "15px" }}>close</span>
                </button>
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
            onAddText={handleAddText}
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
                  {/* YouTube Video Player Embed / Source Header */}
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
                        {/* YouTube Video Embed Player with Exact Timestamp Seek */}
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

                        {/* Source Metadata Card */}
                        <div
                          style={{
                            padding: "14px",
                            background: "var(--color-surface-container-low)",
                            borderRadius: "var(--radius-md)",
                            border: "1px solid var(--color-outline-variant)",
                            display: "flex",
                            gap: "10px",
                            alignItems: "flex-start",
                          }}
                        >
                          <div
                            style={{
                              width: "32px",
                              height: "32px",
                              borderRadius: "var(--radius-sm)",
                              background: isVideo ? "rgba(255, 0, 0, 0.15)" : "var(--color-accent-light)",
                              display: "flex",
                              alignItems: "center",
                              justifyContent: "center",
                              flexShrink: 0,
                            }}
                          >
                            <span className="material-symbols-outlined" style={{ fontSize: "18px", color: isVideo ? "#ff4e4e" : "var(--color-primary)" }}>
                              {isVideo ? "smart_display" : "description"}
                            </span>
                          </div>
                          <div style={{ minWidth: 0, flex: 1 }}>
                            <h4
                              style={{
                                fontFamily: "var(--font-geist)",
                                fontSize: "13.5px",
                                fontWeight: 600,
                                margin: "0 0 4px 0",
                                color: "var(--color-on-surface)",
                                lineHeight: 1.4,
                                wordBreak: "break-word",
                              }}
                            >
                              {displayTitle}
                            </h4>

                            {selectedChunk.source_url && (
                              <p style={{ margin: "0 0 6px 0", fontSize: "11.5px", fontFamily: "var(--font-mono)", color: "var(--color-on-surface-variant)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                                {selectedChunk.source_url}
                              </p>
                            )}

                            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: "8px", marginTop: "4px" }}>
                              {startSec > 0 || selectedChunk.start_timestamp != null ? (
                                <span
                                  style={{
                                    fontSize: "11px",
                                    fontFamily: "var(--font-mono)",
                                    fontWeight: 600,
                                    color: "var(--color-tertiary)",
                                  }}
                                >
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
                                  style={{
                                    fontSize: "11px",
                                    fontFamily: "var(--font-geist)",
                                    fontWeight: 600,
                                    color: "var(--color-primary)",
                                    textDecoration: "none",
                                    display: "inline-flex",
                                    alignItems: "center",
                                    gap: "3px",
                                  }}
                                >
                                  Open in YouTube ↗
                                </a>
                              )}
                            </div>
                          </div>
                        </div>
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
              position: "fixed",
              inset: 0,
              background: "rgba(0,0,0,0.7)",
              zIndex: 110,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              padding: "20px",
            }}
          >
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              onClick={(e) => e.stopPropagation()}
              style={{
                width: "100%",
                maxWidth: "620px",
                background: "var(--color-surface-container-low)",
                border: "1px solid var(--color-outline-variant)",
                borderRadius: "var(--radius-xl)",
                padding: "24px",
                boxShadow: "var(--shadow-lg)",
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
                <button
                  onClick={() => setSelectedDocSummary(null)}
                  style={{ background: "none", border: "none", color: "var(--color-on-surface-variant)", cursor: "pointer" }}
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>

              <div style={{ background: "var(--color-surface-container-lowest)", padding: "18px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)", maxHeight: "360px", overflowY: "auto" }}>
                <p style={{ fontSize: "13.5px", lineHeight: 1.7, margin: 0, color: "var(--color-on-surface)", whiteSpace: "pre-wrap" }}>
                  {selectedDocSummary.summary || "Generating AI Source Guide summary..."}
                </p>
              </div>

              <div style={{ marginTop: "18px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <StatusPill status={selectedDocSummary.status} />
                <button
                  type="button"
                  onClick={() => setSelectedDocSummary(null)}
                  style={{
                    padding: "8px 20px",
                    borderRadius: "var(--radius-md)",
                    background: "var(--color-primary)",
                    color: "var(--color-on-primary)",
                    border: "none",
                    fontFamily: "var(--font-geist)",
                    fontWeight: 600,
                    fontSize: "13px",
                    cursor: "pointer",
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

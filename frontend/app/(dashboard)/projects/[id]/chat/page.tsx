"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  BASE,
  apiGetProject,
  apiGetProjectCourses,
  apiListConversations,
  apiCreateConversation,
  apiGetConversationMessages,
  apiGetChunk,
  getToken,
  apiDeleteConversation,
  apiUpdateConversationTitle,
  type Project,
  type Course,
  type Conversation,
  type ChunkDetail,
} from "@/lib/api";
import { Spinner } from "@/design-system";

function parseTimestampInSeconds(chunk: ChunkDetail): number {
  if (typeof chunk.start_timestamp === "number" && chunk.start_timestamp > 0) {
    return chunk.start_timestamp;
  }
  // Fallback: parse [MM:SS] or [HH:MM:SS] timestamp from transcript header
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

function formatTime(secs?: number): string {
  if (secs == null) return "";
  const m = Math.floor(secs / 60);
  const s = Math.floor(secs % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
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

export default function ChatPage() {
  const params = useParams();
  const router = useRouter();
  const queryClient = useQueryClient();
  const projectId = params.id as string;

  const [activeTab, setActiveTab] = useState<"chats" | "sources">("chats");
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);
  const [loadingChunk, setLoadingChunk] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [activeCitations, setActiveCitations] = useState<Message["citations"]>([]);
  const [activeCitationIndex, setActiveCitationIndex] = useState(0);
  const [sourcePanelTab, setSourcePanelTab] = useState<"overview" | "retrieved">("retrieved");

  const openCitation = useCallback((citations: Message["citations"], index: number) => {
    if (!citations || !citations[index]) return;
    setActiveCitations(citations);
    setActiveCitationIndex(index);
    setSelectedChunkId(citations[index].chunk_id);
    setSourcePanelTab("retrieved");
  }, []);

  const stepCitation = useCallback(
    (dir: 1 | -1) => {
      if (!activeCitations || activeCitations.length === 0) return;
      const next = (activeCitationIndex + dir + activeCitations.length) % activeCitations.length;
      setActiveCitationIndex(next);
      setSelectedChunkId(activeCitations[next].chunk_id);
    },
    [activeCitations, activeCitationIndex]
  );

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Fetch project details
  const { data: project } = useQuery<Project>({
    queryKey: ["project", projectId],
    queryFn: () => apiGetProject(projectId),
  });

  // Fetch indexed sources / courses
  const { data: coursesData } = useQuery<{ items: Course[] }>({
    queryKey: ["project-courses", projectId],
    queryFn: () => apiGetProjectCourses(projectId),
  });

  // Fetch past conversations list
  const { data: conversationsData } = useQuery<{ items: Conversation[] }>({
    queryKey: ["conversations", projectId],
    queryFn: () => apiListConversations(projectId),
  });

  const conversations = conversationsData?.items ?? [];

  // Switch to a past conversation
  const openConversation = useCallback(async (id: string) => {
    setConversationId(id);
    setMessages([]);
    setLoadingHistory(true);
    try {
      const res = await apiGetConversationMessages(id);
      if (res?.items) {
        setMessages(res.items as Message[]);
      }
    } catch (err) {
      console.error("Failed to load conversation history", err);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  // Start new conversation
  const startNewConversation = useCallback(() => {
    setConversationId(null);
    setMessages([]);
  }, []);

  const handleDeleteConversation = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    try {
      await apiDeleteConversation(id);
      if (id === conversationId) {
        setConversationId(null);
        setMessages([]);
      }
      queryClient.invalidateQueries({ queryKey: ["conversations", projectId] });
    } catch (err) {
      console.error("Failed to delete conversation", err);
    }
  };



  // Auto-scroll messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isStreaming]);

  // Fetch single chunk detail when clicked
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
    return () => {
      mounted = false;
    };
  }, [selectedChunkId]);

  // Send query via SSE Streaming
  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isStreaming) return;

    const userText = input.trim();
    setInput("");

    let currentConvId = conversationId;

    // Create conversation if starting fresh
    if (!currentConvId) {
      try {
        const titleSummary = userText.length > 35 ? userText.slice(0, 35) + "…" : userText;
        const newConv = await apiCreateConversation(projectId);
        currentConvId = newConv.id;
        setConversationId(newConv.id);
        await apiUpdateConversationTitle(newConv.id, titleSummary);
        queryClient.invalidateQueries({ queryKey: ["conversations", projectId] });
      } catch (err) {
        console.error("Failed to create conversation", err);
        return;
      }
    }

    const userMsg: Message = {
      id: "user-" + Date.now(),
      role: "user",
      content: userText,
    };

    const assistantMsgId = "asst-" + Date.now();
    const initialAssistantMsg: Message = {
      id: assistantMsgId,
      role: "assistant",
      content: "",
    };

    setMessages((prev) => [...prev, userMsg, initialAssistantMsg]);
    setIsStreaming(true);

    try {
      const token = getToken();
      const response = await fetch(
        `${BASE}/conversations/${currentConvId}/messages`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ content: userText }),
        }
      );

      if (!response.ok || !response.body) {
        throw new Error("Failed to start message stream");
      }

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
              if (resObj.citations) {
                citationsBuffer = resObj.citations;
              }
            } catch {
              // Parse error
            }
          } else if (raw.startsWith("[ERROR:")) {
            assistantContent += `\n\n*Error: ${raw}*`;
          } else {
            assistantContent += raw;
            setMessages((prev) =>
              prev.map((msg) =>
                msg.id === assistantMsgId
                  ? { ...msg, content: assistantContent }
                  : msg
              )
            );
          }
        }
      }

      // Final update with citations
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantMsgId
            ? {
              ...msg,
              content: assistantContent,
              citations: citationsBuffer,
            }
            : msg
        )
      );
      queryClient.invalidateQueries({ queryKey: ["conversations", projectId] });
    } catch (err) {
      console.error("Stream error", err);
      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantMsgId
            ? {
              ...msg,
              content:
                "Sorry, an error occurred while streaming the response.",
            }
            : msg
        )
      );
    } finally {
      setIsStreaming(false);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        height: "calc(100vh - 64px)",
        background: "var(--color-background)",
        color: "var(--color-on-surface)",
        overflow: "hidden",
      }}
    >
      {/* ── LEFT SIDEBAR: Two Tabs (Chats & Sources) ── */}
      <aside
        style={{
          width: "272px",
          flexShrink: 0,
          background: "var(--color-surface-dim)",
          borderRight: "1px solid var(--color-outline-variant)",
          padding: "16px",
          display: "flex",
          flexDirection: "column",
          gap: "14px",
          overflowY: "auto",
        }}
      >
        <Link
          href={`/projects/${projectId}`}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "6px",
            fontSize: "12px",
            fontFamily: "var(--font-geist)",
            fontWeight: 500,
            color: "var(--color-on-surface-variant)",
            textDecoration: "none",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>
            arrow_back
          </span>
          {project?.name || "Back to project"}
        </Link>

        {/* New Chat Button */}
        <button
          type="button"
          onClick={startNewConversation}
          style={{
            width: "100%",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "6px",
            padding: "9px 12px",
            borderRadius: "var(--radius-md)",
            border: "1px solid var(--color-accent-border)",
            background: "var(--color-accent-light)",
            color: "var(--color-primary-fixed)",
            fontFamily: "var(--font-geist)",
            fontSize: "13px",
            fontWeight: 600,
            cursor: "pointer",
            transition: "all var(--transition-fast)",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>
            add
          </span>
          New chat
        </button>

        {/* Tab Switcher Header */}
        <div
          style={{
            display: "flex",
            background: "var(--color-surface-container)",
            border: "1px solid var(--color-outline-variant)",
            borderRadius: "var(--radius-md)",
            padding: "3px",
            gap: "2px",
          }}
        >
          <button
            type="button"
            onClick={() => setActiveTab("chats")}
            style={{
              flex: 1,
              padding: "6px 8px",
              border: "none",
              borderRadius: "6px",
              cursor: "pointer",
              fontFamily: "var(--font-geist)",
              fontSize: "12px",
              fontWeight: 600,
              background:
                activeTab === "chats"
                  ? "var(--color-surface-container-highest)"
                  : "transparent",
              color: activeTab === "chats" ? "var(--color-on-surface)" : "var(--color-on-surface-variant)",
              transition: "all 0.2s ease",
            }}
          >
            Chats · {conversations.length}
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("sources")}
            style={{
              flex: 1,
              padding: "6px 8px",
              border: "none",
              borderRadius: "6px",
              cursor: "pointer",
              fontFamily: "var(--font-geist)",
              fontSize: "12px",
              fontWeight: 600,
              background:
                activeTab === "sources"
                  ? "var(--color-surface-container-highest)"
                  : "transparent",
              color: activeTab === "sources" ? "var(--color-on-surface)" : "var(--color-on-surface-variant)",
              transition: "all 0.2s ease",
            }}
          >
            Sources · {coursesData?.items?.length || 0}
          </button>
        </div>

        {/* Tab Body */}
        <div style={{ flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: "6px" }}>
          {activeTab === "chats" ? (
            conversations.length === 0 ? (
              <p
                style={{
                  textAlign: "center",
                  color: "var(--color-on-surface-variant)",
                  fontSize: "12px",
                  padding: "24px 10px",
                  lineHeight: 1.6,
                }}
              >
                No past chats yet.
                <br />
                Start one with &ldquo;New chat&rdquo; above.
              </p>
            ) : (
              conversations.map((conv) => {
                const isSelected = conv.id === conversationId;
                return (
                  <div
                    key={conv.id}
                    onClick={() => {
                      if (conv.id !== conversationId) openConversation(conv.id);
                    }}
                    style={{
                      width: "100%",
                      padding: "9px 10px",
                      borderRadius: "var(--radius-md)",
                      border: `1px solid ${isSelected ? "var(--color-accent-border)" : "transparent"}`,
                      background: isSelected
                        ? "var(--color-accent-light)"
                        : "transparent",
                      color: "var(--color-on-surface)",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: "8px",
                      transition: "background var(--transition-fast)",
                    }}
                  >
                    <div style={{ display: "flex", alignItems: "center", gap: "8px", overflow: "hidden" }}>
                      <span
                        className="material-symbols-outlined"
                        style={{ fontSize: "16px", color: isSelected ? "var(--color-primary)" : "var(--color-on-surface-variant)", flexShrink: 0 }}
                      >
                        chat_bubble
                      </span>
                      <span
                        style={{
                          fontSize: "13px",
                          fontFamily: "var(--font-inter)",
                          fontWeight: isSelected ? 600 : 400,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {conv.title || "Chat session"}
                      </span>
                    </div>

                    <button
                      type="button"
                      onClick={(e) => handleDeleteConversation(e, conv.id)}
                      title="Delete conversation"
                      style={{
                        background: "none",
                        border: "none",
                        color: "var(--color-on-surface-variant)",
                        cursor: "pointer",
                        display: "flex",
                        alignItems: "center",
                        flexShrink: 0,
                      }}
                    >
                      <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>
                        delete
                      </span>
                    </button>
                  </div>
                );
              })
            )
          ) : coursesData?.items?.length === 0 ? (
            <p
              style={{
                textAlign: "center",
                color: "var(--color-on-surface-variant)",
                fontSize: "12px",
                padding: "24px 10px",
              }}
            >
              No indexed sources yet.
            </p>
          ) : (
            coursesData?.items?.map((c) => (
              <div
                key={c.id}
                style={{
                  padding: "9px 10px",
                  border: "1px solid var(--color-outline-variant)",
                  borderRadius: "var(--radius-md)",
                  fontSize: "13px",
                  fontFamily: "var(--font-inter)",
                  display: "flex",
                  alignItems: "center",
                  gap: "8px",
                }}
              >
                <span
                  className="material-symbols-outlined"
                  style={{ fontSize: "16px", color: "var(--color-primary)" }}
                >
                  auto_stories
                </span>
                <span
                  style={{
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {c.title}
                </span>
              </div>
            ))
          )}
        </div>
      </aside>

      {/* ── CENTER: Chat Container ── */}
      <main
        style={{
          flex: 1,
          minWidth: 0,
          display: "flex",
          flexDirection: "column",
          background: "var(--color-background)",
        }}
      >
        {/* Messages Feed */}
        <div
          style={{
            flex: 1,
            padding: "28px clamp(16px, 5vw, 64px)",
            overflowY: "auto",
            display: "flex",
            flexDirection: "column",
            gap: "22px",
          }}
        >
          {loadingHistory ? (
            <div
              style={{
                margin: "auto",
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: "10px",
              }}
            >
              <Spinner size={22} color="var(--color-primary)" />
              <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>
                Loading conversation history…
              </span>
            </div>
          ) : messages.length === 0 ? (
            <div
              style={{
                margin: "auto",
                textAlign: "center",
                maxWidth: "420px",
                color: "var(--color-on-surface-variant)",
              }}
            >
              <div
                style={{
                  width: "56px",
                  height: "56px",
                  borderRadius: "var(--radius-lg)",
                  background: "var(--color-primary-container)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  margin: "0 auto 16px",
                }}
              >
                <span
                  className="material-symbols-outlined"
                  style={{ fontSize: "26px", color: "var(--color-on-primary-container)" }}
                >
                  auto_stories
                </span>
              </div>
              <h3 style={{ margin: "0 0 8px 0", fontFamily: "var(--font-geist)", fontSize: "17px", color: "var(--color-on-surface)" }}>
                Ask your sources anything
              </h3>
              <p style={{ fontSize: "13.5px", lineHeight: 1.6 }}>
                Every answer here is grounded in the material you&rsquo;ve indexed — with
                citation tabs you can click to jump straight to the passage it came from.
              </p>
            </div>
          ) : (
            messages.map((m) => (
              <div
                key={m.id}
                className="fade-up"
                style={{
                  display: "flex",
                  flexDirection: "column",
                  alignItems: m.role === "user" ? "flex-end" : "flex-start",
                }}
              >
                <span
                  style={{
                    fontFamily: "var(--font-geist)",
                    fontSize: "10.5px",
                    fontWeight: 600,
                    letterSpacing: "0.06em",
                    textTransform: "uppercase",
                    color: "var(--color-on-surface-variant)",
                    marginBottom: "6px",
                    padding: "0 4px",
                  }}
                >
                  {m.role === "user" ? "You" : "Assistant"}
                </span>
                <div
                  style={{
                    maxWidth: "88%",
                    padding: m.role === "user" ? "12px 16px" : "18px 20px",
                    borderRadius: "var(--radius-lg)",
                    background:
                      m.role === "user"
                        ? "var(--color-surface-container-high)"
                        : "var(--color-surface-container-low)",
                    border: "1px solid var(--color-outline-variant)",
                  }}
                >
                  {/* Message Content */}
                  {m.role === "assistant" ? (
                    <div className="answer-prose">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                        {m.content || (isStreaming ? "…" : "")}
                      </ReactMarkdown>
                    </div>
                  ) : (
                    <div style={{ fontSize: "14px", lineHeight: 1.6, fontFamily: "var(--font-inter)" }}>
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content}</ReactMarkdown>
                    </div>
                  )}

                  {/* Grounded Source strip below assistant message */}
                  {m.citations && m.citations.length > 0 && (
                    <div
                      style={{
                        marginTop: "16px",
                        paddingTop: "14px",
                        borderTop: "1px solid var(--color-outline-variant)",
                      }}
                    >
                      <p
                        style={{
                          fontSize: "10.5px",
                          fontFamily: "var(--font-geist)",
                          fontWeight: 700,
                          color: "var(--color-on-surface-variant)",
                          letterSpacing: "0.06em",
                          textTransform: "uppercase",
                          margin: "0 0 9px 0",
                        }}
                      >
                        Sources ({m.citations.length})
                      </p>
                      <div
                        style={{
                          display: "flex",
                          gap: "8px",
                          flexWrap: "wrap",
                        }}
                      >
                        {m.citations.map((c, i) => {
                          const isActive =
                            selectedChunkId === c.chunk_id && activeCitations === m.citations;
                          return (
                            <button
                              key={c.chunk_id + "-" + i}
                              type="button"
                              onClick={() => openCitation(m.citations, i)}
                              style={{
                                display: "flex",
                                alignItems: "center",
                                gap: "8px",
                                padding: "7px 11px",
                                background: isActive
                                  ? "var(--color-accent-light)"
                                  : "var(--color-surface-container)",
                                border: `1px solid ${isActive ? "var(--color-accent-border)" : "var(--color-outline-variant)"}`,
                                borderRadius: "var(--radius-md)",
                                cursor: "pointer",
                                color: "var(--color-on-surface)",
                                textAlign: "left",
                                transition: "all var(--transition-fast)",
                                maxWidth: "220px",
                              }}
                            >
                              <span className="citation-tab" style={{ width: "18px", height: "16px", fontSize: "9.5px", flexShrink: 0 }}>
                                {i + 1}
                              </span>
                              <div style={{ overflow: "hidden" }}>
                                <p
                                  style={{
                                    margin: 0,
                                    fontSize: "11.5px",
                                    fontFamily: "var(--font-geist)",
                                    fontWeight: 600,
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                    whiteSpace: "nowrap",
                                  }}
                                >
                                  {c.title || `Source ${i + 1}`}
                                </p>
                                {c.start_timestamp != null && (
                                  <span
                                    style={{
                                      fontSize: "10px",
                                      fontFamily: "var(--font-mono)",
                                      color: "var(--color-tertiary)",
                                      fontWeight: 600,
                                    }}
                                  >
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

        {/* Input Bar */}
        <form
          onSubmit={handleSend}
          style={{
            padding: "16px clamp(16px, 5vw, 64px) 20px",
            borderTop: "1px solid var(--color-outline-variant)",
            display: "flex",
            gap: "10px",
            background: "var(--color-surface-dim)",
          }}
        >
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask a question about your indexed materials…"
            disabled={isStreaming}
            className="input-glow"
            style={{
              flex: 1,
              padding: "12px 16px",
              borderRadius: "var(--radius-md)",
              background: "var(--color-surface-container-lowest)",
              border: "1px solid var(--color-outline-variant)",
              color: "var(--color-on-surface)",
              fontFamily: "var(--font-inter)",
              fontSize: "14px",
              outline: "none",
            }}
          />
          <button
            type="submit"
            disabled={!input.trim() || isStreaming}
            style={{
              padding: "0 22px",
              borderRadius: "var(--radius-md)",
              background: "var(--color-primary)",
              color: "var(--color-on-primary)",
              border: "none",
              fontFamily: "var(--font-geist)",
              fontWeight: 600,
              fontSize: "13.5px",
              cursor: "pointer",
              opacity: !input.trim() || isStreaming ? 0.5 : 1,
              transition: "opacity var(--transition-fast)",
            }}
          >
            {isStreaming ? <Spinner size={16} color="var(--color-on-primary)" /> : "Send"}
          </button>
        </form>
      </main>

      {/* ── RIGHT PANEL: Source detail — Overview / Retrieved content tabs,
           with prev/next navigation across the citations on the answer
           that's currently open (mirrors "Source X of Y"). ── */}
      <AnimatePresence>
        {selectedChunkId && (
          <motion.aside
            initial={{ width: 0, opacity: 0 }}
            animate={{ width: 380, opacity: 1 }}
            exit={{ width: 0, opacity: 0 }}
            transition={{ type: "spring", damping: 28, stiffness: 260 }}
            style={{
              flexShrink: 0,
              height: "100%",
              background: "var(--color-surface-dim)",
              borderLeft: "1px solid var(--color-outline-variant)",
              overflow: "hidden",
              display: "flex",
              flexDirection: "column",
            }}
          >
            <div style={{ width: "380px", height: "100%", display: "flex", flexDirection: "column" }}>
              {/* Panel header */}
              <div
                style={{
                  padding: "16px 18px 12px",
                  borderBottom: "1px solid var(--color-outline-variant)",
                  display: "flex",
                  flexDirection: "column",
                  gap: "10px",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <h3
                    style={{
                      fontFamily: "var(--font-geist)",
                      fontSize: "13px",
                      fontWeight: 700,
                      letterSpacing: "0.03em",
                      textTransform: "uppercase",
                      margin: 0,
                      color: "var(--color-on-surface-variant)",
                    }}
                  >
                    Source material
                  </h3>
                  <button
                    onClick={() => setSelectedChunkId(null)}
                    aria-label="Close source panel"
                    style={{
                      background: "none",
                      border: "none",
                      color: "var(--color-on-surface-variant)",
                      cursor: "pointer",
                      display: "flex",
                      padding: "2px",
                    }}
                  >
                    <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>
                      close
                    </span>
                  </button>
                </div>

                {/* Source X of Y navigator */}
                {activeCitations && activeCitations.length > 1 && (
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <button
                      type="button"
                      onClick={() => stepCitation(-1)}
                      style={{ background: "none", border: "none", color: "var(--color-on-surface)", cursor: "pointer", display: "flex" }}
                      aria-label="Previous source"
                    >
                      <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>chevron_left</span>
                    </button>
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "11.5px", color: "var(--color-on-surface-variant)" }}>
                      Source {activeCitationIndex + 1} of {activeCitations.length}
                    </span>
                    <button
                      type="button"
                      onClick={() => stepCitation(1)}
                      style={{ background: "none", border: "none", color: "var(--color-on-surface)", cursor: "pointer", display: "flex" }}
                      aria-label="Next source"
                    >
                      <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>chevron_right</span>
                    </button>
                  </div>
                )}

                {/* Overview / Retrieved content tabs */}
                <div
                  style={{
                    display: "flex",
                    background: "var(--color-surface-container)",
                    border: "1px solid var(--color-outline-variant)",
                    borderRadius: "var(--radius-md)",
                    padding: "3px",
                    gap: "2px",
                  }}
                >
                  {(["overview", "retrieved"] as const).map((tab) => (
                    <button
                      key={tab}
                      type="button"
                      onClick={() => setSourcePanelTab(tab)}
                      style={{
                        flex: 1,
                        padding: "6px 8px",
                        border: "none",
                        borderRadius: "6px",
                        cursor: "pointer",
                        fontFamily: "var(--font-geist)",
                        fontSize: "12px",
                        fontWeight: 600,
                        background: sourcePanelTab === tab ? "var(--color-surface-container-highest)" : "transparent",
                        color: sourcePanelTab === tab ? "var(--color-on-surface)" : "var(--color-on-surface-variant)",
                      }}
                    >
                      {tab === "overview" ? "Overview" : "Retrieved content"}
                    </button>
                  ))}
                </div>
              </div>

              {/* Panel body */}
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
                            <div style={{ borderRadius: "var(--radius-md)", overflow: "hidden", border: "1px solid var(--color-outline-variant)", background: "#000", marginBottom: "6px" }}>
                              <iframe
                                width="100%"
                                height="200"
                                src={`https://www.youtube.com/embed/${ytId}?start=${startSec}&autoplay=1`}
                                title={displayTitle}
                                frameBorder="0"
                                allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
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

                    {sourcePanelTab === "overview" ? (
                      <div>
                        <p
                          style={{
                            fontSize: "10.5px",
                            fontFamily: "var(--font-geist)",
                            fontWeight: 700,
                            textTransform: "uppercase",
                            letterSpacing: "0.05em",
                            color: "var(--color-on-surface-variant)",
                            margin: "0 0 8px 0",
                          }}
                        >
                          About this source
                        </p>
                        <p style={{ fontSize: "13px", lineHeight: 1.7, color: "var(--color-on-surface-variant)" }}>
                          This is the video/document the assistant drew on for its answer. Switch to
                          &ldquo;Retrieved transcript&rdquo; to view the exact transcript segment that was used, or
                          use the arrows above to step through every source cited in this answer.
                        </p>
                      </div>
                    ) : (
                      <div>
                        <p
                          style={{
                            fontSize: "10.5px",
                            fontFamily: "var(--font-geist)",
                            fontWeight: 700,
                            textTransform: "uppercase",
                            letterSpacing: "0.05em",
                            color: "var(--color-on-surface-variant)",
                            margin: "0 0 8px 0",
                          }}
                        >
                          Retrieved transcript
                        </p>
                        <p
                          className="answer-prose"
                          style={{
                            fontSize: "13.5px",
                            background: "var(--color-surface-container-low)",
                            padding: "14px",
                            borderRadius: "var(--radius-md)",
                            border: "1px solid var(--color-outline-variant)",
                            whiteSpace: "pre-wrap",
                            lineHeight: 1.7,
                          }}
                        >
                          {selectedChunk.content}
                        </p>
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            </div>
          </motion.aside>
        )}
      </AnimatePresence>
    </div>
  );
}

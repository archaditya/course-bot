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
        background: "#080e1a",
        color: "var(--color-on-surface)",
        overflow: "hidden",
      }}
    >
      {/* ── LEFT SIDEBAR: Two Tabs (Chats & Sources) ── */}
      <aside
        style={{
          width: "280px",
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
            color: "var(--color-on-surface-variant)",
            textDecoration: "none",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>
            arrow_back
          </span>
          Back to project
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
            borderRadius: "8px",
            border: "1px solid var(--color-primary)",
            background: "rgba(140, 136, 255, 0.1)",
            color: "var(--color-primary)",
            fontSize: "13px",
            fontWeight: 600,
            cursor: "pointer",
          }}
        >
          <span className="material-symbols-outlined" style={{ fontSize: "16px" }}>
            add
          </span>
          New Chat
        </button>

        {/* Tab Switcher Header */}
        <div
          style={{
            display: "flex",
            background: "rgba(255, 255, 255, 0.05)",
            borderRadius: "8px",
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
              fontSize: "12px",
              fontWeight: 600,
              background:
                activeTab === "chats"
                  ? "var(--color-primary)"
                  : "transparent",
              color: activeTab === "chats" ? "#fff" : "var(--color-on-surface-variant)",
              transition: "all 0.2s ease",
            }}
          >
            Chats ({conversations.length})
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
              fontSize: "12px",
              fontWeight: 600,
              background:
                activeTab === "sources"
                  ? "var(--color-primary)"
                  : "transparent",
              color: activeTab === "sources" ? "#fff" : "var(--color-on-surface-variant)",
              transition: "all 0.2s ease",
            }}
          >
            Sources ({coursesData?.items?.length || 0})
          </button>
        </div>

        {/* Tab Body */}
        <div style={{ flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: "8px" }}>
          {activeTab === "chats" ? (
            conversations.length === 0 ? (
              <p
                style={{
                  textAlign: "center",
                  color: "var(--color-on-surface-variant)",
                  fontSize: "12px",
                  padding: "20px 10px",
                }}
              >
                No past chats found. Click &quot;New Chat&quot; above.
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
                      padding: "10px 12px",
                      borderRadius: "8px",
                      border: `1px solid ${isSelected ? "var(--color-primary)" : "rgba(255, 255, 255, 0.08)"
                        }`,
                      background: isSelected
                        ? "rgba(140, 136, 255, 0.15)"
                        : "rgba(30, 40, 60, 0.3)",
                      color: "var(--color-on-surface)",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: "8px",
                    }}
                  >
                    <div style={{ display: "flex", alignItems: "center", gap: "8px", overflow: "hidden" }}>
                      <span
                        className="material-symbols-outlined"
                        style={{ fontSize: "16px", color: "var(--color-primary)", flexShrink: 0 }}
                      >
                        chat
                      </span>
                      <span
                        style={{
                          fontSize: "13px",
                          fontWeight: 600,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {conv.title || "Chat Session"}
                      </span>
                    </div>

                    <button
                      type="button"
                      onClick={(e) => handleDeleteConversation(e, conv.id)}
                      title="Delete Conversation"
                      style={{
                        background: "none",
                        border: "none",
                        color: "var(--color-on-surface-variant)",
                        cursor: "pointer",
                        padding: "2px",
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
          ) : (
            coursesData?.items?.length === 0 ? (
              <p
                style={{
                  textAlign: "center",
                  color: "var(--color-on-surface-variant)",
                  fontSize: "12px",
                  padding: "20px 10px",
                }}
              >
                No indexed sources found.
              </p>
            ) : (
              coursesData?.items?.map((c) => (
                <div
                  key={c.id}
                  style={{
                    padding: "10px 12px",
                    background: "rgba(30, 40, 60, 0.4)",
                    border: "1px solid rgba(255, 255, 255, 0.05)",
                    borderRadius: "8px",
                    fontSize: "13px",
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
            )
          )}
        </div>
      </aside>

      {/* ── CENTER: Chat Container ── */}
      <main
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          background: "#080e1a",
        }}
      >
        {/* Messages Feed */}
        <div
          style={{
            flex: 1,
            padding: "24px",
            overflowY: "auto",
            display: "flex",
            flexDirection: "column",
            gap: "20px",
          }}
        >
          {loadingHistory ? (
            <div
              style={{
                margin: "auto",
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: "8px",
              }}
            >
              <Spinner size={24} color="var(--color-primary)" />
              <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)" }}>
                Loading conversation history...
              </span>
            </div>
          ) : messages.length === 0 ? (
            <div
              style={{
                margin: "auto",
                textAlign: "center",
                maxWidth: "400px",
                color: "var(--color-on-surface-variant)",
              }}
            >
              <span
                className="material-symbols-outlined"
                style={{
                  fontSize: "48px",
                  color: "var(--color-primary)",
                  marginBottom: "12px",
                }}
              >
                chat_spark
              </span>
              <h3 style={{ margin: "0 0 8px 0" }}>archadiLM Assistant</h3>
              <p style={{ fontSize: "13px", lineHeight: 1.5 }}>
                Ask questions across all indexed source material with grounded
                citations.
              </p>
            </div>
          ) : (
            messages.map((m) => (
              <div
                key={m.id}
                style={{
                  display: "flex",
                  flexDirection: "column",
                  alignItems:
                    m.role === "user" ? "flex-end" : "flex-start",
                }}
              >
                <div
                  style={{
                    maxWidth: "85%",
                    padding: "16px 20px",
                    borderRadius: "16px",
                    background:
                      m.role === "user"
                        ? "var(--color-primary-container)"
                        : "rgba(22, 32, 54, 0.7)",
                    border:
                      m.role === "user"
                        ? "none"
                        : "1px solid rgba(255, 255, 255, 0.08)",
                  }}
                >
                  {/* Message Content */}
                  <div style={{ fontSize: "14px", lineHeight: 1.7 }}>
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {m.content}
                    </ReactMarkdown>
                  </div>

                  {/* Grounded Source Boxes below assistant message */}
                  {m.citations && m.citations.length > 0 && (
                    <div
                      style={{
                        marginTop: "16px",
                        paddingTop: "14px",
                        borderTop: "1px solid rgba(155, 155, 255, 0.14)",
                      }}
                    >
                      <p
                        style={{
                          fontSize: "11px",
                          fontWeight: 700,
                          color: "var(--color-secondary)",
                          letterSpacing: "0.05em",
                          textTransform: "uppercase",
                          margin: "0 0 8px 0",
                        }}
                      >
                        Cited Sources ({m.citations.length})
                      </p>
                      <div
                        style={{
                          display: "flex",
                          gap: "8px",
                          flexWrap: "wrap",
                        }}
                      >
                        {m.citations.map((c, i) => {
                          const formatTime = (secs?: number): string => {
                            if (secs == null) return "";
                            const m = Math.floor(secs / 60);
                            const s = secs % 60;
                            return `${m}:${String(s).padStart(2, "0")}`;
                          };

                          return (
                            <button
                              key={c.chunk_id + "-" + i}
                              type="button"
                              onClick={() => setSelectedChunkId(c.chunk_id)}
                              style={{
                                display: "flex",
                                alignItems: "center",
                                gap: "8px",
                                padding: "8px 12px",
                                background: "rgba(10, 18, 38, 0.6)",
                                border:
                                  "1px solid rgba(155, 155, 255, 0.2)",
                                borderRadius: "8px",
                                cursor: "pointer",
                                color: "var(--color-on-surface)",
                                textAlign: "left",
                                transition: "all 0.2s ease",
                              }}
                            >
                              <span
                                className="material-symbols-outlined"
                                style={{
                                  fontSize: "16px",
                                  color: "var(--color-primary)",
                                }}
                              >
                                description
                              </span>
                              <div>
                                <p
                                  style={{
                                    margin: 0,
                                    fontSize: "12px",
                                    fontWeight: 600,
                                    maxWidth: "180px",
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                    whiteSpace: "nowrap",
                                  }}
                                >
                                  {c.title || `Source [${i + 1}]`}
                                </p>
                                {c.start_timestamp != null && (
                                  <span
                                    style={{
                                      fontSize: "10px",
                                      color: "var(--color-secondary)",
                                      fontWeight: 600,
                                    }}
                                  >
                                    ⏱ {formatTime(c.start_timestamp)}
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
            padding: "16px 24px",
            borderTop: "1px solid var(--color-outline-variant)",
            display: "flex",
            gap: "12px",
            background: "var(--color-surface-dim)",
          }}
        >
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask a question about your indexed materials..."
            disabled={isStreaming}
            style={{
              flex: 1,
              padding: "12px 16px",
              borderRadius: "10px",
              background: "rgba(20, 28, 48, 0.8)",
              border: "1px solid var(--color-outline-variant)",
              color: "#fff",
              fontSize: "14px",
              outline: "none",
            }}
          />
          <button
            type="submit"
            disabled={!input.trim() || isStreaming}
            style={{
              padding: "0 20px",
              borderRadius: "10px",
              background: "var(--color-primary)",
              color: "var(--color-on-primary)",
              border: "none",
              fontWeight: 600,
              cursor: "pointer",
              opacity: !input.trim() || isStreaming ? 0.5 : 1,
            }}
          >
            {isStreaming ? (
              <Spinner size={16} color="var(--color-on-primary)" />
            ) : (
              "Send"
            )}
          </button>
        </form>
      </main>

      {/* ── RIGHT SLIDE-OVER: User-Friendly Source Material Detail ── */}
      <AnimatePresence>
        {selectedChunkId && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setSelectedChunkId(null)}
              style={{
                position: "fixed",
                inset: 0,
                background: "rgba(0, 0, 0, 0.4)",
                zIndex: 90,
              }}
            />

            <motion.aside
              initial={{ x: "100%" }}
              animate={{ x: 0 }}
              exit={{ x: "100%" }}
              transition={{ type: "spring", damping: 25, stiffness: 250 }}
              style={{
                position: "fixed",
                top: 0,
                right: 0,
                height: "100vh",
                width: "400px",
                maxWidth: "90vw",
                background: "var(--color-surface-dim)",
                borderLeft: "1px solid var(--color-outline-variant)",
                padding: "24px",
                display: "flex",
                flexDirection: "column",
                gap: "16px",
                zIndex: 91,
                boxShadow: "-8px 0 30px rgba(0,0,0,0.35)",
                overflowY: "auto",
              }}
            >
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <h3
                  style={{
                    fontFamily: "var(--font-geist)",
                    fontSize: "16px",
                    fontWeight: 600,
                    margin: 0,
                    color: "var(--color-on-surface)",
                  }}
                >
                  Source Material
                </h3>
                <button
                  onClick={() => setSelectedChunkId(null)}
                  style={{
                    background: "none",
                    border: "none",
                    color: "var(--color-on-surface-variant)",
                    cursor: "pointer",
                    fontSize: "18px",
                  }}
                >
                  ✕
                </button>
              </div>

              {loadingChunk ? (
                <div style={{ margin: "auto" }}>
                  <Spinner size={24} color="var(--color-primary)" />
                </div>
              ) : selectedChunk ? (
                <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
                  {/* Source Metadata Card */}
                  <div
                    style={{
                      padding: "14px",
                      background: "rgba(35, 45, 70, 0.5)",
                      borderRadius: "10px",
                      border: "1px solid rgba(155, 155, 255, 0.15)",
                    }}
                  >
                    <h4
                      style={{
                        fontFamily: "var(--font-geist)",
                        fontSize: "14px",
                        fontWeight: 600,
                        margin: "0 0 6px 0",
                        color: "var(--color-primary)",
                        lineHeight: 1.4,
                      }}
                    >
                      {selectedChunk.document_name || selectedChunk.title || "Document Source"}
                    </h4>
                    {selectedChunk.start_timestamp != null && (
                      <span
                        style={{
                          fontSize: "11px",
                          fontWeight: 600,
                          color: "var(--color-secondary)",
                          display: "inline-flex",
                          alignItems: "center",
                          gap: "4px",
                        }}
                      >
                        ⏱ Timestamp: {Math.floor(selectedChunk.start_timestamp / 60)}:{String(selectedChunk.start_timestamp % 60).padStart(2, "0")}
                        {selectedChunk.end_timestamp != null &&
                          ` - ${Math.floor(selectedChunk.end_timestamp / 60)}:${String(selectedChunk.end_timestamp % 60).padStart(2, "0")}`}
                      </span>
                    )}
                  </div>

                  {/* Excerpt Body */}
                  <div>
                    <p
                      style={{
                        fontSize: "11px",
                        fontWeight: 700,
                        textTransform: "uppercase",
                        letterSpacing: "0.05em",
                        color: "var(--color-on-surface-variant)",
                        margin: "0 0 8px 0",
                      }}
                    >
                      Extracted Excerpt
                    </p>
                    <p
                      style={{
                        fontSize: "13px",
                        lineHeight: 1.65,
                        background: "rgba(18, 26, 46, 0.7)",
                        padding: "14px",
                        borderRadius: "10px",
                        border: "1px solid rgba(255, 255, 255, 0.08)",
                        color: "var(--color-on-surface)",
                        whiteSpace: "pre-wrap",
                      }}
                    >
                      {selectedChunk.content}
                    </p>
                  </div>
                </div>
              ) : null}
            </motion.aside>
          </>
        )}
      </AnimatePresence>
    </div>
  );
}

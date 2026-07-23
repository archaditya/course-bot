"use client";

import React, { useState, useEffect, useRef, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { motion, AnimatePresence } from "framer-motion";
import {
  apiCreateConversation,
  apiGetChunk,
  getToken,
  type Collection,
  type ChunkDetail,
  apiListCollections,
  apiListConversations,
} from "@/lib/api";
import { Spinner, CitationMarker } from "@/design-system";

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

export default function ProjectChatPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();

  const [conversationId, setConversationId] = useState<string | null>(null);
  const [selectedCollectionId, setSelectedCollectionId] = useState<
    string | null
  >(null);
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);
  const [loadingChunk, setLoadingChunk] = useState(false);

  const [messages, setMessages] = useState<Message[]>([]);
  const [inputQuery, setInputQuery] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  const [activeSidebarTab, setActiveSidebarTab] = useState<'conversations' | 'sources'>('conversations');
  const { data: conversationsData } = useQuery({
    queryKey: ['conversations', projectId],
    queryFn: () => apiListConversations(projectId),
  });

  const conversations = conversationsData?.items ?? [];

  // Default tab fallback logic: if conversations exist, default to 'conversations', else 'sources'
  useEffect(() => {
    if (conversations?.length) {
      setActiveSidebarTab('conversations');
    } else {
      setActiveSidebarTab('sources');
    }
  }, [conversationsData]);

  // Poll courses list every 3s if any course is still processing
  const { data: coursesData } = useQuery({
    queryKey: ["courses", projectId],
    queryFn: () => apiListCollections(projectId),
    refetchInterval: (query) => {
      const hasProcessing = query.state.data?.items?.some(
        (c) => !["INDEXED", "CREATED", "FAILED"].includes(c.status),
      );
      return hasProcessing ? 3000 : false;
    },
  });

  const indexedCollections =
    coursesData?.items.filter(
      (collection) => collection.status === "INDEXED",
    ) ?? [];

  useEffect(() => {
    if (coursesData?.items?.length && !selectedCollectionId) {
      const indexed = coursesData.items.find((c) => c.status === "INDEXED");
      setSelectedCollectionId(indexed?.id ?? coursesData.items[0].id);
    }
  }, [coursesData, selectedCollectionId]);

  useEffect(() => {
    if (!selectedChunkId) {
      setSelectedChunk(null);
      return;
    }
    setLoadingChunk(true);
    apiGetChunk(selectedChunkId)
      .then(setSelectedChunk)
      .catch(() => setSelectedChunk(null))
      .finally(() => setLoadingChunk(false));
  }, [selectedChunkId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, streamingContent]);

  const handleSendMessage = useCallback(async () => {
    const text = inputQuery.trim();
    if (!text || isStreaming) return;

    let convId = conversationId;
    if (!convId) {
      const conv = await apiCreateConversation(projectId);
      convId = conv.id;
      setConversationId(conv.id);
    }

    const targetCollectionId = selectedCollectionId;

    if (!targetCollectionId) {
      setMessages((previous) => [
        ...previous,
        {
          id: `${Date.now()}-no-source`,
          role: 'assistant',
          content: 'Please select an indexed source before asking a question.',
        },
      ]);
      return;
    }

    const userMsg: Message = {
      id: Date.now().toString(),
      role: "user",
      content: text,
    };
    setMessages((prev) => [...prev, userMsg]);
    setInputQuery("");
    setIsStreaming(true);
    setStreamingContent("");

    const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
    const token = getToken();

    try {
      const res = await fetch(`${BASE}/conversations/${convId}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ content: text, course_id: targetCollectionId }),
      });

      if (!res.ok || !res.body) throw new Error(`Server error: ${res.status}`);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let fullText = "";
      let resultData: Message | null = null;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        const chunkText = decoder.decode(value);
        for (const line of chunkText.split("\n")) {
          if (!line.startsWith("data: ")) continue;
          const payload = line.slice(6);
          if (payload === "[DONE]") break;
          if (payload.startsWith("[RESULT]")) {
            try {
              resultData = JSON.parse(payload.slice(8).trim());
            } catch { }
            continue;
          }
          fullText += payload;
          setStreamingContent(fullText);
        }
      }

      const citationsList = resultData?.citations || (resultData as any)?.Citations;

      setMessages((prev) => [
        ...prev,
        {
          id: resultData?.id ?? Date.now().toString() + "-a",
          role: "assistant",
          content: fullText,
          citations: citationsList,
        },
      ]);
      setStreamingContent("");
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        {
          id: Date.now().toString() + "-err",
          role: "assistant",
          content: `Error: ${err instanceof Error ? err.message : "Query failed"}`,
        },
      ]);
    } finally {
      setIsStreaming(false);
    }
  }, [
    inputQuery,
    isStreaming,
    conversationId,
    projectId,
    selectedCollectionId,
    coursesData,
  ]);

  return (
    <div
      style={{
        display: "flex",
        height: "calc(100vh - 64px)",
        margin: "-32px -40px",
        overflow: "hidden",
        background: "var(--color-background)",
        position: "relative",
      }}
    >
      <aside style={{ width: "280px", flexShrink: 0, borderRight: "1px solid rgba(155, 155, 255, 0.18)", display: "flex", flexDirection: "column", background: "rgba(10, 18, 38, 0.55)" }}>
        <div style={{ padding: "16px", borderBottom: "1px solid rgba(155, 155, 255, 0.14)" }}>
          <button type="button" onClick={() => router.push(`/projects/${projectId}`)} style={{ border: 0, background: "transparent", color: "var(--color-primary)", cursor: "pointer", display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", fontWeight: 600, marginBottom: "14px" }}>
            <span className="material-symbols-outlined" style={{ fontSize: "17px" }}>arrow_back</span>
            Back to project
          </button>
          {/* TABS HEADER */}
          <div style={{ display: "flex", background: "rgba(155, 155, 255, 0.08)", borderRadius: "8px", padding: "3px", gap: "2px" }}>
            <button
              type="button"
              onClick={() => setActiveSidebarTab('conversations')}
              style={{
                flex: 1, padding: "6px 8px", border: "none", borderRadius: "6px", cursor: "pointer", fontSize: "12px", fontWeight: 600,
                background: activeSidebarTab === 'conversations' ? "var(--color-primary)" : "transparent",
                color: activeSidebarTab === 'conversations' ? "#fff" : "var(--color-on-surface-variant)",
                transition: "all 0.2s ease",
              }}
            >
              Chats ({conversations?.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveSidebarTab('sources')}
              style={{
                flex: 1, padding: "6px 8px", border: "none", borderRadius: "6px", cursor: "pointer", fontSize: "12px", fontWeight: 600,
                background: activeSidebarTab === 'sources' ? "var(--color-primary)" : "transparent",
                color: activeSidebarTab === 'sources' ? "#fff" : "var(--color-on-surface-variant)",
                transition: "all 0.2s ease",
              }}
            >
              Sources ({indexedCollections.length})
            </button>
          </div>
        </div>
        {/* TAB CONTENT BODY */}
        <div style={{ flex: 1, overflowY: "auto", padding: "12px", display: "flex", flexDirection: "column", gap: "8px" }}>
          {activeSidebarTab === 'conversations' ? (
            conversations.length === 0 ? (
              <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "20px 10px" }}>
                No past chats found.
              </p>
            ) : (
              conversations.map((conv) => {
                const isSelected = conv.id === conversationId;
                return (
                  <button
                    key={conv.id}
                    type="button"
                    onClick={() => setConversationId(conv.id)}
                    style={{
                      width: "100%", padding: "10px 12px", borderRadius: "8px", border: `1px solid ${isSelected ? "var(--color-primary)" : "rgba(155, 155, 255, 0.14)"}`,
                      background: isSelected ? "rgba(140, 136, 255, 0.15)" : "transparent", color: "var(--color-on-surface)", textAlign: "left", cursor: "pointer"
                    }}
                  >
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                      <span className="material-symbols-outlined" style={{ fontSize: "16px", color: "var(--color-primary)" }}>chat</span>
                      <span style={{ fontSize: "13px", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {conv.title || "Chat Session"}
                      </span>
                    </div>
                  </button>
                );
              })
            )
          ) : (
            indexedCollections.length === 0 ? (
              <p style={{ textAlign: "center", color: "var(--color-on-surface-variant)", fontSize: "12px", padding: "20px 10px" }}>
                No indexed sources found.
              </p>
            ) : (
              indexedCollections.map((collection) => {
                const isSelected = collection.id === selectedCollectionId;
                return (
                  <button
                    key={collection.id}
                    type="button"
                    onClick={() => setSelectedCollectionId(collection.id)}
                    style={{
                      width: "100%", padding: "10px 12px", borderRadius: "8px", border: `1px solid ${isSelected ? "var(--color-primary)" : "rgba(155, 155, 255, 0.14)"}`,
                      background: isSelected ? "rgba(140, 136, 255, 0.15)" : "transparent", color: "var(--color-on-surface)", textAlign: "left", cursor: "pointer"
                    }}
                  >
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                      <span className="material-symbols-outlined" style={{ fontSize: "16px", color: "var(--color-primary)" }}>description</span>
                      <span style={{ fontSize: "13px", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {collection.title}
                      </span>
                    </div>
                  </button>
                );
              })
            )
          )}
        </div>
      </aside>

      {/* ── MAIN CHAT PANE ────────────────────────────────────────── */}
      <section
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          position: "relative",
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: "16px 24px",
            borderBottom: "1px solid rgba(70,69,84,0.3)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <div>
            <h2
              style={{
                fontFamily: "var(--font-geist)",
                fontSize: "18px",
                fontWeight: 700,
                color: "var(--color-on-surface)",
                margin: 0,
              }}
            >
              archadiLM Assistant
            </h2>
            <p
              style={{
                fontFamily: "var(--font-inter)",
                fontSize: "12px",
                color: "var(--color-on-surface-variant)",
                margin: 0,
              }}
            >
              Ask questions across all indexed source material with grounded
              citations.
            </p>
          </div>
        </div>

        {/* Message Stream */}
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            padding: "24px",
            display: "flex",
            flexDirection: "column",
            gap: "20px",
            paddingBottom: "120px",
          }}
        >
          {messages.length === 0 && !isStreaming && (
            <div
              style={{ margin: "auto", textAlign: "center", maxWidth: "400px" }}
            >
              <span
                className="material-symbols-outlined"
                style={{
                  color: "var(--color-primary)",
                  fontSize: "48px",
                  marginBottom: "12px",
                }}
              >
                psychology
              </span>
              <h3
                style={{
                  fontFamily: "var(--font-geist)",
                  fontSize: "18px",
                  fontWeight: 600,
                  marginBottom: "8px",
                }}
              >
                Ask anything about your material
              </h3>
              <p
                style={{
                  fontFamily: "var(--font-inter)",
                  fontSize: "13px",
                  color: "var(--color-on-surface-variant)",
                }}
              >
                Index PDFs, web links, or text notes on the left pane and get
                vector-grounded responses with exact citations.
              </p>
            </div>
          )}

          {messages.map((m) => (
            <div
              key={m.id}
              style={{
                display: "flex",
                justifyContent: m.role === "user" ? "flex-end" : "flex-start",
              }}
            >
              {m.role === "user" ? (
                <div
                  style={{
                    maxWidth: "80%",
                    padding: "12px 16px",
                    borderRadius: "16px",
                    background: "var(--color-surface-container-highest)",
                    color: "var(--color-on-surface)",
                    fontSize: "14px",
                  }}
                >
                  {m.content}
                </div>
              ) : (
                <div
                  className="glass-panel"
                  style={{
                    maxWidth: "90%",
                    padding: "20px",
                    borderRadius: "16px",
                  }}
                >
                  {/* Markdown Response Content */}
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
                                border: "1px solid rgba(155, 155, 255, 0.2)",
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
              )}
            </div>
          ))}

          {isStreaming && (
            <div
              className="glass-panel"
              style={{ padding: "20px", borderRadius: "16px" }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "8px",
                  marginBottom: "8px",
                }}
              >
                <Spinner size={14} color="var(--color-primary)" />
                <span
                  style={{
                    fontSize: "12px",
                    color: "var(--color-primary)",
                    fontFamily: "var(--font-geist)",
                  }}
                >
                  Generating response...
                </span>
              </div>
              <div style={{ fontSize: "14px", lineHeight: 1.7 }}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {streamingContent}
                </ReactMarkdown>
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        {/* Input Bar */}
        <div
          style={{
            position: "absolute",
            bottom: 0,
            left: 0,
            right: 0,
            padding: "20px 24px",
            background:
              "linear-gradient(to top, var(--color-background) 70%, transparent)",
          }}
        >
          <div
            className="glass-panel"
            style={{
              maxWidth: "900px",
              margin: "0 auto",
              borderRadius: "16px",
              padding: "8px 12px",
              display: "flex",
              alignItems: "center",
              gap: "12px",
            }}
          >
            <input
              type="text"
              value={inputQuery}
              onChange={(e) => setInputQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSendMessage()}
              placeholder="Ask a question about your indexed materials..."
              disabled={isStreaming || indexedCollections.length === 0}
              style={{
                flex: 1,
                background: "transparent",
                border: "none",
                outline: "none",
                fontFamily: "var(--font-inter)",
                fontSize: "14px",
                color: "var(--color-on-surface)",
              }}
            />
            <button
              onClick={handleSendMessage}
              disabled={!inputQuery.trim() || isStreaming}
              style={{
                padding: "10px 20px",
                background: "var(--color-primary)",
                color: "var(--color-on-primary)",
                border: "none",
                borderRadius: "10px",
                fontFamily: "var(--font-geist)",
                fontSize: "13px",
                fontWeight: 600,
                cursor:
                  !inputQuery.trim() || isStreaming ? "not-allowed" : "pointer",
                opacity: !inputQuery.trim() || isStreaming ? 0.5 : 1,
              }}
            >
              Ask
            </button>
          </div>
        </div>
      </section>

      {/* ── RIGHT PANE: Chunk Detail ──────────────────────────────── */}
      <AnimatePresence>
        {selectedChunkId && (
          <motion.aside
            initial={{ x: 350, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: 350, opacity: 0 }}
            transition={{ duration: 0.3 }}
            style={{
              width: "320px",
              minWidth: "320px",
              background: "var(--color-surface-dim)",
              borderLeft: "1px solid var(--color-outline-variant)",
              padding: "24px",
              display: "flex",
              flexDirection: "column",
              gap: "16px",
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
                  margin: 0,
                }}
              >
                Source Chunk
              </h3>
              <button
                onClick={() => setSelectedChunkId(null)}
                style={{
                  background: "none",
                  border: "none",
                  color: "var(--color-on-surface-variant)",
                  cursor: "pointer",
                }}
              >
                ✕
              </button>
            </div>
            {loadingChunk ? (
              <Spinner size={24} color="var(--color-primary)" />
            ) : selectedChunk ? (
              <div>
                <h4
                  style={{
                    fontFamily: "var(--font-geist)",
                    fontSize: "14px",
                    marginBottom: "8px",
                    color: "var(--color-primary)",
                  }}
                >
                  {selectedChunk.title || "Source Citation"}
                </h4>
                <p
                  style={{
                    fontSize: "13px",
                    lineHeight: 1.6,
                    background: "rgba(45,52,73,0.3)",
                    padding: "12px",
                    borderRadius: "8px",
                    color: "var(--color-on-surface-variant)",
                  }}
                >
                  {selectedChunk.content}
                </p>
              </div>
            ) : null}
          </motion.aside>
        )}
      </AnimatePresence>
    </div>
  );
}

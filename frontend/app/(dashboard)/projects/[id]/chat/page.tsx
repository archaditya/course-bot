'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import {
  apiListCourses,
  apiCreateConversation,
  apiGetChunk,
  getToken,
  type Course,
  type ChunkDetail,
} from '@/lib/api';
import { Button, Spinner, Badge, CitationMarker } from '@/design-system';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  citations?: Array<{ chunk_id: string; document_id: string; start_timestamp?: number; title?: string }>;
}

export default function ProjectChatPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const router = useRouter();

  const [conversationId, setConversationId] = useState<string | null>(null);
  const [selectedCourseId, setSelectedCourseId] = useState<string | null>(null);
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);
  const [loadingChunk, setLoadingChunk] = useState(false);

  const [messages, setMessages] = useState<Message[]>([]);
  const [inputQuery, setInputQuery] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);

  // Load Indexed Courses
  const { data: coursesData } = useQuery({
    queryKey: ['courses', projectId],
    queryFn: () => apiListCourses(projectId),
  });

  useEffect(() => {
    if (coursesData?.items?.length) {
      const indexed = coursesData.items.find((c) => c.status === 'INDEXED');
      setSelectedCourseId(indexed?.id ?? coursesData.items[0].id);
    }
  }, [coursesData]);

  // Load Chunk detail when citation clicked
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
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
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

    const targetCourseId = selectedCourseId || coursesData?.items?.[0]?.id || '';

    const userMsg: Message = { id: Date.now().toString(), role: 'user', content: text };
    setMessages((prev) => [...prev, userMsg]);
    setInputQuery('');
    setIsStreaming(true);
    setStreamingContent('');

    const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
    const token = getToken();

    try {
      const res = await fetch(`${BASE}/conversations/${convId}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ content: text, course_id: targetCourseId }),
      });

      if (!res.ok || !res.body) throw new Error(`Server error: ${res.status}`);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let fullText = '';
      let resultData: Message | null = null;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const chunkText = decoder.decode(value);
        const lines = chunkText.split('\n');
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const payload = line.slice(6);
          if (payload === '[DONE]') break;
          if (payload.startsWith('[RESULT]')) {
            try {
              resultData = JSON.parse(payload.slice(8).trim());
            } catch {}
            continue;
          }
          fullText += payload;
          setStreamingContent(fullText);
        }
      }

      const assistantMsg: Message = {
        id: resultData?.id ?? Date.now().toString() + '-a',
        role: 'assistant',
        content: fullText,
        citations: resultData?.citations,
      };
      setMessages((prev) => [...prev, assistantMsg]);
      setStreamingContent('');
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        { id: Date.now().toString() + '-err', role: 'assistant', content: `Error: ${err instanceof Error ? err.message : 'Failed'}` },
      ]);
    } finally {
      setIsStreaming(false);
    }
  }, [inputQuery, isStreaming, conversationId, projectId, selectedCourseId, coursesData]);

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 65px)', margin: '-var(--space-8)' }}>
      {/* ── LEFT SIDEBAR ───────────────────────────────────────────────────── */}
      <aside style={{ width: '260px', background: 'var(--color-surface)', borderRight: '1px solid var(--color-border-subtle)', padding: 'var(--space-4)', display: 'flex', flexDirection: 'column' }}>
        <button
          onClick={() => router.push(`/projects/${projectId}`)}
          style={{ background: 'none', border: 'none', color: 'var(--color-ink-muted)', cursor: 'pointer', fontSize: 'var(--text-xs)', marginBottom: 'var(--space-3)' }}
        >
          ← Choice Page
        </button>

        <h3 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-sm)', textTransform: 'uppercase', color: 'var(--color-ink-muted)', marginBottom: 'var(--space-3)' }}>
          Indexed Sources
        </h3>

        <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
          {coursesData?.items.map((c: Course) => (
            <button
              key={c.id}
              onClick={() => setSelectedCourseId(c.id)}
              style={{
                padding: 'var(--space-2) var(--space-3)',
                borderRadius: 'var(--radius-md)',
                background: selectedCourseId === c.id ? 'var(--color-accent-light)' : 'transparent',
                border: 'none',
                textAlign: 'left',
                cursor: 'pointer',
                fontSize: 'var(--text-sm)',
              }}
            >
              📄 {c.title}
            </button>
          ))}
        </div>
      </aside>

      {/* ── MAIN CHAT PANEL ────────────────────────────────────────────────── */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%' }}>
        <div style={{ flex: 1, overflowY: 'auto', padding: 'var(--space-6)', display: 'flex', flexDirection: 'column', gap: 'var(--space-5)' }}>
          {messages.length === 0 && !isStreaming && (
            <div style={{ margin: 'auto', textAlign: 'center' }}>
              <p style={{ fontSize: '3rem', marginBottom: 'var(--space-2)' }}>💬</p>
              <h3 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)' }}>Ask anything about this project</h3>
            </div>
          )}

          {messages.map((m) => (
            <div key={m.id} style={{ display: 'flex', justifyContent: m.role === 'user' ? 'flex-end' : 'flex-start' }}>
              <div style={{ maxWidth: '80%', background: m.role === 'user' ? 'var(--color-accent)' : 'var(--color-surface)', color: m.role === 'user' ? '#fff' : 'var(--color-ink)', padding: 'var(--space-4)', borderRadius: 'var(--radius-xl)' }}>
                {m.role === 'user' ? (
                  <p style={{ margin: 0 }}>{m.content}</p>
                ) : (
                  <>
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content}</ReactMarkdown>
                    {m.citations && (
                      <div style={{ display: 'flex', gap: 'var(--space-2)', marginTop: 'var(--space-3)' }}>
                        {m.citations.map((c, i) => (
                          <CitationMarker key={c.chunk_id} index={i} chunkId={c.chunk_id} title={c.title} startTimestamp={c.start_timestamp} onJumpTo={() => setSelectedChunkId(c.chunk_id)} />
                        ))}
                      </div>
                    )}
                  </>
                )}
              </div>
            </div>
          ))}

          {isStreaming && (
            <div style={{ background: 'var(--color-surface)', padding: 'var(--space-4)', borderRadius: 'var(--radius-xl)' }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{streamingContent}</ReactMarkdown>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        {/* Prompt Input Box */}
        <div style={{ padding: 'var(--space-4) var(--space-6)', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-surface)' }}>
          <div style={{ display: 'flex', gap: 'var(--space-3)', maxWidth: '900px', margin: '0 auto' }}>
            <textarea
              value={inputQuery}
              onChange={(e) => setInputQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && (e.preventDefault(), handleSendMessage())}
              placeholder="Type a Query here..."
              rows={1}
              style={{ flex: 1, padding: '12px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-lg)', background: 'var(--color-paper)', outline: 'none' }}
            />
            <Button onClick={handleSendMessage} disabled={!inputQuery.trim() || isStreaming}>Send</Button>
          </div>
        </div>
      </main>

      {/* ── RIGHT SIDEBAR (Citation Detail) ────────────────────────────────── */}
      <aside style={{ width: '340px', background: 'var(--color-surface)', borderLeft: '1px solid var(--color-border-subtle)', transform: selectedChunkId ? 'translateX(0)' : 'translateX(100%)', transition: 'transform var(--transition-slow)', position: 'fixed', top: 65, right: 0, bottom: 0, padding: 'var(--space-5)', zIndex: 50 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
          <h4 style={{ fontFamily: 'var(--font-display)', margin: 0 }}>Source Info</h4>
          <button onClick={() => setSelectedChunkId(null)} style={{ background: 'none', border: 'none', cursor: 'pointer' }}>✕</button>
        </div>
        {loadingChunk ? (
          <Spinner />
        ) : selectedChunk ? (
          <div>
            <h4 style={{ fontFamily: 'var(--font-display)', marginBottom: 'var(--space-2)' }}>{selectedChunk.title}</h4>
            {selectedChunk.start_timestamp != null && <Badge variant="accent">⏱ {selectedChunk.start_timestamp}s</Badge>}
            <p style={{ marginTop: 'var(--space-3)', fontSize: 'var(--text-sm)', lineHeight: 1.7, background: 'var(--color-paper)', padding: 'var(--space-3)', borderRadius: 'var(--radius-md)' }}>
              {selectedChunk.content}
            </p>
          </div>
        ) : null}
      </aside>
    </div>
  );
}

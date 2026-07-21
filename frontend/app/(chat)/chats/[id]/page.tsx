'use client';

import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { CitationMarker, Button, Spinner } from '@/design-system';
import { getToken } from '@/lib/api';
import { useState, useRef, useEffect, useCallback } from 'react';
import type { Metadata } from 'next';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  citations?: Array<{ chunk_id: string; document_id: string; start_timestamp?: number; title?: string }>;
  confidence?: string;
}

export default function ChatPage() {
  const { id: conversationId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const courseId = searchParams.get('course') ?? '';

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingContent]);

  const sendMessage = useCallback(async () => {
    const content = input.trim();
    if (!content || streaming) return;

    const userMsg: Message = { id: Date.now().toString(), role: 'user', content };
    setMessages((prev) => [...prev, userMsg]);
    setInput('');
    setStreaming(true);
    setStreamingContent('');

    const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
    const token = getToken();

    try {
      const res = await fetch(`${BASE}/conversations/${conversationId}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ content, course_id: courseId }),
      });

      if (!res.ok || !res.body) throw new Error(`Server error: ${res.status}`);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let fullContent = '';
      let resultData: Message | null = null;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const text = decoder.decode(value);
        const lines = text.split('\n');
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const data = line.slice(6);
          if (data === '[DONE]') break;
          if (data.startsWith('[RESULT]')) {
            try {
              resultData = JSON.parse(data.slice(8).trim());
            } catch {}
            continue;
          }
          if (data.startsWith('[ERROR:')) {
            fullContent += `\n\n⚠️ ${data}`;
            continue;
          }
          fullContent += data;
          setStreamingContent(fullContent);
        }
      }

      const assistantMsg: Message = {
        id: resultData?.message_id ?? Date.now().toString() + '-a',
        role: 'assistant',
        content: fullContent,
        citations: resultData?.citations,
        confidence: resultData?.confidence,
      };
      setMessages((prev) => [...prev, assistantMsg]);
      setStreamingContent('');
    } catch (err) {
      setMessages((prev) => [...prev, {
        id: Date.now().toString() + '-err',
        role: 'assistant',
        content: `Sorry, something went wrong: ${err instanceof Error ? err.message : 'Unknown error'}. Please try again.`,
      }]);
    } finally {
      setStreaming(false);
    }
  }, [input, streaming, conversationId, courseId]);

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: '100vh',
      background: 'var(--color-paper)',
    }}>
      {/* Header */}
      <header style={{
        padding: 'var(--space-4) var(--space-6)',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: 'var(--color-surface)',
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--space-4)',
      }}>
        <h1 style={{
          fontFamily: 'var(--font-display)',
          fontSize: 'var(--text-lg)',
          fontWeight: 600,
        }}>
          Chat
        </h1>
      </header>

      {/* Messages */}
      <div style={{
        flex: 1,
        overflowY: 'auto',
        padding: 'var(--space-6)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-6)',
        maxWidth: 'var(--content-max)',
        width: '100%',
        margin: '0 auto',
      }}>
        {messages.length === 0 && !streaming && (
          <div style={{
            textAlign: 'center',
            padding: 'var(--space-16)',
            color: 'var(--color-ink-muted)',
          }}>
            <p style={{ fontSize: 'var(--text-5xl)', marginBottom: 'var(--space-4)' }}>💬</p>
            <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-2xl)', marginBottom: 'var(--space-2)', color: 'var(--color-ink)' }}>
              Ask anything about this course
            </h2>
            <p>Every answer is grounded in the material, with citations you can click.</p>
          </div>
        )}

        {messages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} />
        ))}

        {streaming && streamingContent && (
          <div style={{
            background: 'var(--color-surface)',
            border: '1px solid var(--color-border-subtle)',
            borderRadius: 'var(--radius-xl)',
            padding: 'var(--space-5)',
            boxShadow: 'var(--shadow-sm)',
          }}>
            <MarkdownContent content={streamingContent} />
            <span style={{
              display: 'inline-block',
              width: '8px',
              height: '18px',
              background: 'var(--color-accent)',
              marginLeft: '2px',
              animation: 'blink 1s step-end infinite',
            }} />
            <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
          </div>
        )}

        {streaming && !streamingContent && (
          <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'center', color: 'var(--color-ink-muted)' }}>
            <Spinner size={16} /> Thinking…
          </div>
        )}

        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div style={{
        padding: 'var(--space-4) var(--space-6)',
        borderTop: '1px solid var(--color-border-subtle)',
        background: 'var(--color-surface)',
      }}>
        <div style={{
          maxWidth: 'var(--content-max)',
          margin: '0 auto',
          display: 'flex',
          gap: 'var(--space-3)',
          alignItems: 'flex-end',
        }}>
          <textarea
            ref={inputRef}
            id="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
              }
            }}
            placeholder="Ask a question about this course…"
            rows={1}
            style={{
              flex: 1,
              padding: '12px 16px',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-lg)',
              fontSize: 'var(--text-base)',
              fontFamily: 'var(--font-body)',
              background: 'var(--color-paper)',
              color: 'var(--color-ink)',
              resize: 'none',
              lineHeight: 1.6,
              outline: 'none',
            }}
          />
          <Button
            id="btn-send"
            onClick={sendMessage}
            disabled={!input.trim() || streaming}
            loading={streaming}
          >
            Ask
          </Button>
        </div>
        <p style={{
          maxWidth: 'var(--content-max)',
          margin: 'var(--space-2) auto 0',
          fontSize: 'var(--text-xs)',
          color: 'var(--color-ink-muted)',
          textAlign: 'center',
        }}>
          Press Enter to send · Shift+Enter for new line
        </p>
      </div>
    </div>
  );
}

function MessageBubble({ message }: { message: Message }) {
  const isUser = message.role === 'user';
  return (
    <div style={{ display: 'flex', justifyContent: isUser ? 'flex-end' : 'flex-start' }}>
      <div style={{
        maxWidth: '75%',
        background: isUser ? 'var(--color-accent)' : 'var(--color-surface)',
        color: isUser ? '#fff' : 'var(--color-ink)',
        border: isUser ? 'none' : '1px solid var(--color-border-subtle)',
        borderRadius: isUser ? 'var(--radius-xl) var(--radius-xl) var(--radius-sm) var(--radius-xl)' : 'var(--radius-xl) var(--radius-xl) var(--radius-xl) var(--radius-sm)',
        padding: 'var(--space-4) var(--space-5)',
        boxShadow: 'var(--shadow-sm)',
      }}>
        {isUser ? (
          <p style={{ margin: 0 }}>{message.content}</p>
        ) : (
          <>
            <MarkdownContent content={message.content} />
            {message.citations && message.citations.length > 0 && (
              <div style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: 'var(--space-2)',
                marginTop: 'var(--space-4)',
                paddingTop: 'var(--space-3)',
                borderTop: '1px solid var(--color-border-subtle)',
              }}>
                {message.citations.map((cit, i) => (
                  <CitationMarker
                    key={cit.chunk_id}
                    index={i}
                    chunkId={cit.chunk_id}
                    title={cit.title}
                    startTimestamp={cit.start_timestamp}
                  />
                ))}
              </div>
            )}
            {message.confidence === 'low_confidence' && (
              <p style={{
                marginTop: 'var(--space-3)',
                fontSize: 'var(--text-xs)',
                color: 'var(--color-warning)',
                fontStyle: 'italic',
              }}>
                ⚠ I'm not fully confident in this answer. Please verify with the original material.
              </p>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function MarkdownContent({ content }: { content: string }) {
  return (
    <div style={{ fontSize: 'var(--text-base)', lineHeight: 1.8 }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ className, children }) {
            return (
              <code style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '0.9em',
                background: 'var(--color-paper-subtle)',
                padding: '1px 5px',
                borderRadius: 'var(--radius-sm)',
              }}>
                {children}
              </code>
            );
          },
          pre({ children }) {
            return <pre style={{
              background: 'var(--color-ink)',
              color: '#E5E7EB',
              padding: 'var(--space-4)',
              borderRadius: 'var(--radius-md)',
              overflowX: 'auto',
              margin: 'var(--space-3) 0',
            }}>{children}</pre>;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

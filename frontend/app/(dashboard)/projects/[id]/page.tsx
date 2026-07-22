'use client';

import { useParams, useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { apiGetProject } from '@/lib/api';
import { Spinner } from '@/design-system';

export default function ProjectChoicePage() {
  const { id: projectId } = useParams<{ id: string }>();
  const router = useRouter();

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => apiGetProject(projectId),
  });

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 'var(--space-16)' }}>
        <Spinner size={32} />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: '900px', margin: '0 auto', textAlign: 'center' }}>
      <button
        onClick={() => router.push('/')}
        style={{
          background: 'none',
          border: 'none',
          color: 'var(--color-ink-muted)',
          fontSize: 'var(--text-sm)',
          cursor: 'pointer',
          marginBottom: 'var(--space-4)',
        }}
      >
        ← Back to Projects
      </button>

      <h1 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-3xl)', fontWeight: 700, marginBottom: 'var(--space-2)' }}>
        {project?.name || 'Project'}
      </h1>
      <p style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-lg)', marginBottom: 'var(--space-12)' }}>
        Select an action for this project
      </p>

      {/* ── TWO ACTION CARDS ONLY ────────────────────────────────────────────── */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 'var(--space-8)' }}>
        {/* Card 1: Indexing */}
        <button
          onClick={() => router.push(`/projects/${projectId}/indexing`)}
          style={{
            background: 'var(--color-surface)',
            border: '2px solid var(--color-border-subtle)',
            borderRadius: 'var(--radius-2xl)',
            padding: 'var(--space-10) var(--space-6)',
            cursor: 'pointer',
            textAlign: 'center',
            boxShadow: 'var(--shadow-md)',
            transition: 'all var(--transition-normal)',
          }}
        >
          <div style={{ fontSize: '4rem', marginBottom: 'var(--space-4)' }}>📁</div>
          <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-2xl)', fontWeight: 700, marginBottom: 'var(--space-3)' }}>
            Index Materials
          </h2>
          <p style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-base)', lineHeight: 1.6, margin: 0 }}>
            Upload PDFs, videos, URLs, subtitles, or ZIP files to process and index into this project.
          </p>
        </button>

        {/* Card 2: Chat UI */}
        <button
          onClick={() => router.push(`/projects/${projectId}/chat`)}
          style={{
            background: 'var(--color-surface)',
            border: '2px solid var(--color-accent-border)',
            borderRadius: 'var(--radius-2xl)',
            padding: 'var(--space-10) var(--space-6)',
            cursor: 'pointer',
            textAlign: 'center',
            boxShadow: 'var(--shadow-md)',
            transition: 'all var(--transition-normal)',
          }}
        >
          <div style={{ fontSize: '4rem', marginBottom: 'var(--space-4)' }}>💬</div>
          <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-2xl)', fontWeight: 700, marginBottom: 'var(--space-3)', color: 'var(--color-accent)' }}>
            Chat Assistant
          </h2>
          <p style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-base)', lineHeight: 1.6, margin: 0 }}>
            Ask questions with streaming answers, grounded citations, and NotebookLM source sidebars.
          </p>
        </button>
      </div>
    </div>
  );
}

'use client';

import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiListCourses, apiCreateCourse, apiUpload, apiAddSource, Course } from '@/lib/api';
import { Button, Badge, Spinner } from '@/design-system';

interface SourceOption {
  id: string;
  label: string;
  icon: string;
  desc: string;
  accept?: string;
  inputType: 'file' | 'url' | 'text';
}

const SOURCES: SourceOption[] = [
  { id: 'pdf', label: 'PDF Document', icon: '📄', desc: 'Upload textbooks, slides, or papers', accept: '.pdf', inputType: 'file' },
  { id: 'video_url', label: 'Video URL', icon: '🎬', desc: 'YouTube or online video URL', inputType: 'url' },
  { id: 'upload', label: 'Upload Subtitles', icon: '📁', desc: 'SRT or VTT subtitle transcripts', accept: '.srt,.vtt', inputType: 'file' },
  { id: 'url', label: 'Web URL', icon: '🌐', desc: 'Webpage or article link', inputType: 'url' },
  { id: 'text', label: 'Raw Text', icon: '📝', desc: 'Paste lecture notes or text', inputType: 'text' },
  { id: 'zip', label: 'ZIP Archive', icon: '📦', desc: 'Archive of SRT, VTT, PDF, TXT files', accept: '.zip', inputType: 'file' },
];

export default function IndexingPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [activeSource, setActiveSource] = useState<SourceOption | null>(null);

  // Poll courses list for live background status updates
  const { data: coursesData, isLoading } = useQuery({
    queryKey: ['courses', projectId],
    queryFn: () => apiListCourses(projectId),
    refetchInterval: (query) => {
      const hasProcessing = query.state.data?.items?.some(
        (c) => !['INDEXED', 'CREATED', 'FAILED'].includes(c.status)
      );
      return hasProcessing ? 3000 : false;
    },
  });

  return (
    <div style={{ maxWidth: '1000px', margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-6)' }}>
        <button
          onClick={() => router.push(`/projects/${projectId}`)}
          style={{ background: 'none', border: 'none', color: 'var(--color-ink-muted)', cursor: 'pointer', fontSize: 'var(--text-sm)' }}
        >
          ← Back to Choice Page
        </button>
        <Button size="sm" onClick={() => router.push(`/projects/${projectId}/chat`)}>
          Go to Chat Assistant →
        </Button>
      </div>

      <h1 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-3xl)', fontWeight: 700, marginBottom: 'var(--space-2)' }}>
        Index Materials
      </h1>
      <p style={{ color: 'var(--color-ink-secondary)', marginBottom: 'var(--space-8)' }}>
        Click a source card to upload and start indexing into this project.
      </p>

      {/* ── 6 SOURCE CARDS ─────────────────────────────────────────────────── */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 'var(--space-4)', marginBottom: 'var(--space-12)' }}>
        {SOURCES.map((s) => (
          <button
            key={s.id}
            onClick={() => setActiveSource(s)}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              padding: 'var(--space-6)',
              background: 'var(--color-surface)',
              border: '1px solid var(--color-border-subtle)',
              borderRadius: 'var(--radius-xl)',
              cursor: 'pointer',
              boxShadow: 'var(--shadow-sm)',
              transition: 'all var(--transition-fast)',
              textAlign: 'center',
            }}
          >
            <span style={{ fontSize: '3rem', marginBottom: 'var(--space-2)' }}>{s.icon}</span>
            <h3 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-lg)', fontWeight: 600, marginBottom: 'var(--space-1)' }}>
              {s.label}
            </h3>
            <p style={{ fontSize: 'var(--text-xs)', color: 'var(--color-ink-muted)', margin: 0 }}>{s.desc}</p>
          </button>
        ))}
      </div>

      {/* ── LIVE INDEXED MATERIALS LIST ────────────────────────────────────── */}
      <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)', marginBottom: 'var(--space-4)' }}>
        Indexed Materials ({coursesData?.items?.length ?? 0})
      </h2>

      {isLoading ? (
        <Spinner />
      ) : !coursesData?.items?.length ? (
        <p style={{ color: 'var(--color-ink-muted)' }}>No materials indexed yet. Select a source above to add your first file.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          {coursesData.items.map((c: Course) => (
            <div
              key={c.id}
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: 'var(--space-4)',
                background: 'var(--color-surface)',
                border: '1px solid var(--color-border-subtle)',
                borderRadius: 'var(--radius-lg)',
              }}
            >
              <div>
                <h4 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-base)', fontWeight: 600, margin: 0 }}>
                  {c.title}
                </h4>
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-ink-muted)' }}>
                  Added {new Date(c?.created_at).toLocaleTimeString()}
                </span>
              </div>
              <Badge variant={c.status === 'INDEXED' ? 'success' : c.status === 'FAILED' ? 'error' : 'warning'}>
                {c.status}
              </Badge>
            </div>
          ))}
        </div>
      )}

      {/* Input Modal */}
      {activeSource && (
        <SourceModal
          source={activeSource}
          projectId={projectId}
          onClose={() => setActiveSource(null)}
          onSuccess={() => {
            setActiveSource(null);
            queryClient.invalidateQueries({ queryKey: ['courses', projectId] });
          }}
        />
      )}
    </div>
  );
}

function SourceModal({ source, projectId, onClose, onSuccess }: { source: SourceOption; projectId: string; onClose: () => void; onSuccess: () => void }) {
  const [title, setTitle] = useState('');
  const [url, setUrl] = useState('');
  const [content, setContent] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async () => {
    setLoading(true);
    setError('');

    try {
      const courseTitle = title || file?.name || url || 'Untitled Source';
      const course = await apiCreateCourse(projectId, courseTitle);

      if (source.inputType === 'file' && file) {
        await apiUpload(course.id, projectId, file);
      } else if (source.inputType === 'url') {
        const sourceType = source.id === 'video_url' ? 'video_url' : 'url';
        await apiAddSource(course.id, sourceType as 'url' | 'video_url', { url, title: courseTitle });
      } else if (source.inputType === 'text') {
        await apiAddSource(course.id, 'text', { content, title: courseTitle });
      }

      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
      <div style={{ background: 'var(--color-surface)', borderRadius: 'var(--radius-xl)', padding: 'var(--space-6)', maxWidth: '480px', width: '90%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
          <h3 style={{ fontFamily: 'var(--font-display)', margin: 0 }}>
            {source.icon} Add {source.label}
          </h3>
          <button onClick={onClose} style={{ background: 'none', border: 'none', fontSize: '1.2rem', cursor: 'pointer' }}>✕</button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Source title (optional)"
            style={{ padding: '10px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-md)' }}
          />

          {source.inputType === 'file' && (
            <input
              type="file"
              accept={source.accept}
              onChange={(e) => e.target.files?.[0] && setFile(e.target.files[0])}
              style={{ padding: '10px', border: '1px dashed var(--color-border)', borderRadius: 'var(--radius-md)' }}
            />
          )}

          {source.inputType === 'url' && (
            <input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="Paste URL here…"
              style={{ padding: '10px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-md)' }}
            />
          )}

          {source.inputType === 'text' && (
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="Paste text content here…"
              rows={6}
              style={{ padding: '10px', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-md)' }}
            />
          )}

          {error && <p style={{ color: 'var(--color-error)', fontSize: 'var(--text-xs)', margin: 0 }}>⚠ {error}</p>}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-2)', marginTop: 'var(--space-2)' }}>
            <Button variant="secondary" onClick={onClose} disabled={loading}>Cancel</Button>
            <Button onClick={handleSubmit} loading={loading}>Index Source</Button>
          </div>
        </div>
      </div>
    </div>
  );
}

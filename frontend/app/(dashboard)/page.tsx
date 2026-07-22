'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { apiListProjects, apiCreateProject } from '@/lib/api';
import { Button, Spinner } from '@/design-system';
import { useState } from 'react';

export default function DashboardPage() {
  const queryClient = useQueryClient();
  const [projectName, setProjectName] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: () => apiListProjects(),
  });

  const { mutate: createProject, isPending } = useMutation({
    mutationFn: () => apiCreateProject(projectName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setProjectName('');
      setShowCreate(false);
    },
  });

  return (
    <div style={{ maxWidth: '1000px', margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-8)' }}>
        <div>
          <h1 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-3xl)', fontWeight: 700, margin: 0 }}>
            Projects
          </h1>
          <p style={{ color: 'var(--color-ink-secondary)', marginTop: 'var(--space-1)' }}>
            Select a project or create a new one to manage and chat with your materials.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          + New Project
        </Button>
      </div>

      {showCreate && (
        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-xl)',
          padding: 'var(--space-5)',
          marginBottom: 'var(--space-6)',
          display: 'flex',
          gap: 'var(--space-3)',
          alignItems: 'flex-end',
        }}>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 'var(--text-sm)', fontWeight: 500, display: 'block', marginBottom: 'var(--space-1)' }}>
              Project Name
            </label>
            <input
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              placeholder="e.g. Machine Learning Course, History Notes…"
              onKeyDown={(e) => e.key === 'Enter' && projectName && createProject()}
              style={{
                width: '100%',
                padding: '10px 14px',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                fontSize: 'var(--text-base)',
                background: 'var(--color-paper)',
              }}
            />
          </div>
          <Button onClick={() => createProject()} disabled={!projectName.trim()} loading={isPending}>
            Create
          </Button>
          <Button variant="ghost" onClick={() => setShowCreate(false)}>Cancel</Button>
        </div>
      )}

      {isLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 'var(--space-12)' }}>
          <Spinner size={32} />
        </div>
      ) : !data?.items?.length ? (
        <div style={{
          textAlign: 'center',
          padding: 'var(--space-16)',
          background: 'var(--color-surface)',
          borderRadius: 'var(--radius-2xl)',
          border: '2px dashed var(--color-border)',
        }}>
          <p style={{ fontSize: 'var(--text-4xl)', marginBottom: 'var(--space-4)' }}>📚</p>
          <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)', marginBottom: 'var(--space-2)' }}>
            No projects found
          </h2>
          <p style={{ color: 'var(--color-ink-secondary)', marginBottom: 'var(--space-5)' }}>
            Create your first project to start indexing materials.
          </p>
          <Button onClick={() => setShowCreate(true)}>Create First Project</Button>
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 'var(--space-5)' }}>
          {data.items.map((project) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              style={{
                background: 'var(--color-surface)',
                border: '1px solid var(--color-border-subtle)',
                borderRadius: 'var(--radius-xl)',
                padding: 'var(--space-6)',
                boxShadow: 'var(--shadow-sm)',
                transition: 'all var(--transition-fast)',
                display: 'block',
              }}
            >
              <div style={{ fontSize: '2rem', marginBottom: 'var(--space-2)' }}>📁</div>
              <h3 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                {project.name}
              </h3>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--color-ink-muted)', fontFamily: 'var(--font-mono)', margin: 0 }}>
                Created {new Date(project.created_at).toLocaleDateString()}
              </p>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

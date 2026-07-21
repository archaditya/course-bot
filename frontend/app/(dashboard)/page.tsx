'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { apiListProjects, apiCreateProject } from '@/lib/api';
import { Button, Badge, Spinner } from '@/design-system';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { Metadata } from 'next';

export default function DashboardPage() {
  const queryClient = useQueryClient();
  const [newProjectName, setNewProjectName] = useState('');
  const [creating, setCreating] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: () => apiListProjects(),
  });

  const { mutate: createProject, isPending } = useMutation({
    mutationFn: () => apiCreateProject(newProjectName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setNewProjectName('');
      setCreating(false);
    },
  });

  return (
    <div>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'flex-start',
        marginBottom: 'var(--space-8)',
      }}>
        <div>
          <h1 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-3xl)', fontWeight: 700, marginBottom: 'var(--space-1)' }}>
            Your projects
          </h1>
          <p style={{ color: 'var(--color-ink-secondary)' }}>Each project is a collection of courses you can chat with.</p>
        </div>
        <Button id="btn-new-project" onClick={() => setCreating(true)}>
          + New project
        </Button>
      </div>

      {creating && (
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
            <label htmlFor="project-name" style={{ fontSize: 'var(--text-sm)', fontWeight: 500, display: 'block', marginBottom: 'var(--space-1)' }}>
              Project name
            </label>
            <input
              id="project-name"
              value={newProjectName}
              onChange={(e) => setNewProjectName(e.target.value)}
              placeholder="e.g. Machine Learning Bootcamp"
              onKeyDown={(e) => e.key === 'Enter' && newProjectName && createProject()}
              style={{
                width: '100%',
                padding: '10px 14px',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                fontSize: 'var(--text-base)',
                fontFamily: 'var(--font-body)',
                background: 'var(--color-paper)',
                color: 'var(--color-ink)',
              }}
            />
          </div>
          <Button id="btn-create-project" onClick={() => createProject()} disabled={!newProjectName} loading={isPending}>
            Create
          </Button>
          <Button variant="ghost" onClick={() => setCreating(false)}>Cancel</Button>
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
            No projects yet
          </h2>
          <p style={{ color: 'var(--color-ink-secondary)', marginBottom: 'var(--space-5)' }}>
            Create your first project to start uploading course material.
          </p>
          <Button id="btn-first-project" onClick={() => setCreating(true)}>
            Create your first project
          </Button>
        </div>
      ) : (
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          gap: 'var(--space-4)',
        }}>
          {data.items.map((project) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              style={{
                background: 'var(--color-surface)',
                border: '1px solid var(--color-border-subtle)',
                borderRadius: 'var(--radius-xl)',
                padding: 'var(--space-5)',
                boxShadow: 'var(--shadow-sm)',
                transition: 'box-shadow var(--transition-fast), border-color var(--transition-fast)',
                display: 'block',
              }}
            >
              <h3 style={{
                fontFamily: 'var(--font-display)',
                fontSize: 'var(--text-lg)',
                fontWeight: 600,
                marginBottom: 'var(--space-2)',
              }}>
                {project.name}
              </h3>
              <p style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--color-ink-muted)',
                fontFamily: 'var(--font-mono)',
              }}>
                {new Date(project.created_at).toLocaleDateString()}
              </p>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

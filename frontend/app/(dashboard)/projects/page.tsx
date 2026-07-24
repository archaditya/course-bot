'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { apiListProjects, apiCreateProject } from '@/lib/api';
import { Spinner } from '@/design-system';
import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

const PROJECT_ICONS = ['auto_awesome', 'science', 'database', 'psychology', 'hub', 'biotech'];
const PROJECT_ICON_COLORS = [
  'var(--color-primary)',
  'var(--color-tertiary)',
  'var(--color-secondary)',
  'var(--color-primary-container)',
  'var(--color-tertiary)',
  'var(--color-secondary)',
];

const stagger = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.07 } },
};

const cardVariant = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' as const } },
};

export default function ProjectsPage() {
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
    <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', marginBottom: '40px' }}
      >
        <div>
          <h2
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '28px',
              fontWeight: 600,
              letterSpacing: '-0.01em',
              color: 'var(--color-on-surface)',
              margin: 0,
              marginBottom: '6px',
            }}
          >
            My Projects
          </h2>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '14px', color: 'var(--color-on-surface-variant)', margin: 0 }}>
            Manage your AI-enhanced research and documents.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '8px' }}>
          {[
            { icon: 'filter_list', label: 'Filter' },
            { icon: 'sort', label: 'Recent' },
          ].map(({ icon, label }) => (
            <button
              key={label}
              className="glass-card"
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '8px 14px',
                borderRadius: '8px',
                border: '1px solid var(--color-outline-variant)',
                background: 'rgba(19,27,46,0.5)',
                cursor: 'pointer',
                fontFamily: 'var(--font-geist)',
                fontSize: '12px',
                fontWeight: 500,
                color: 'var(--color-on-surface)',
                letterSpacing: '0.05em',
                transition: 'all 0.2s',
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>{icon}</span>
              {label}
            </button>
          ))}
        </div>
      </motion.div>

      {/* Create Project Modal */}
      <AnimatePresence>
        {showCreate && (
          <motion.div
            initial={{ opacity: 0, height: 0, marginBottom: 0 }}
            animate={{ opacity: 1, height: 'auto', marginBottom: '24px' }}
            exit={{ opacity: 0, height: 0, marginBottom: 0 }}
            transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
            style={{ overflow: 'hidden' }}
          >
            <div
              className="glass-card"
              style={{
                borderRadius: '16px',
                padding: '20px',
                border: '1px solid rgba(192,193,255,0.2)',
                display: 'flex',
                gap: '12px',
                alignItems: 'flex-end',
              }}
            >
              <div style={{ flex: 1 }}>
                <label
                  style={{
                    fontFamily: 'var(--font-geist)',
                    fontSize: '11px',
                    fontWeight: 500,
                    color: 'var(--color-on-surface-variant)',
                    letterSpacing: '0.05em',
                    textTransform: 'uppercase',
                    display: 'block',
                    marginBottom: '8px',
                  }}
                >
                  Project Name
                </label>
                <input
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  placeholder="e.g. Machine Learning Course, History Notes…"
                  onKeyDown={(e) => e.key === 'Enter' && projectName && createProject()}
                  autoFocus
                  className="input-glow"
                  style={{
                    width: '100%',
                    background: 'var(--color-surface-container-lowest)',
                    border: '1px solid var(--color-outline-variant)',
                    borderRadius: '10px',
                    padding: '12px 14px',
                    fontFamily: 'var(--font-inter)',
                    fontSize: '14px',
                    color: 'var(--color-on-surface)',
                    outline: 'none',
                  }}
                />
              </div>
              <button
                onClick={() => createProject()}
                disabled={!projectName.trim() || isPending}
                style={{
                  padding: '12px 20px',
                  background: 'var(--color-primary)',
                  color: 'var(--color-on-primary)',
                  border: 'none',
                  borderRadius: '10px',
                  fontFamily: 'var(--font-geist)',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: isPending || !projectName.trim() ? 'not-allowed' : 'pointer',
                  opacity: isPending || !projectName.trim() ? 0.6 : 1,
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                }}
              >
                {isPending ? <Spinner size={14} /> : null}
                Create
              </button>
              <button
                onClick={() => { setShowCreate(false); setProjectName(''); }}
                style={{
                  padding: '12px 16px',
                  background: 'transparent',
                  color: 'var(--color-on-surface-variant)',
                  border: '1px solid var(--color-outline-variant)',
                  borderRadius: '10px',
                  fontFamily: 'var(--font-geist)',
                  fontSize: '13px',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Project Grid */}
      {isLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '80px' }}>
          <Spinner size={32} />
        </div>
      ) : (
        <motion.div
          initial="hidden"
          animate="visible"
          variants={stagger}
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
            gap: '24px',
          }}
        >
          {/* New Project CTA Card */}
          <motion.button
            variants={cardVariant}
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={() => setShowCreate(true)}
            className="glass-card"
            style={{
              aspectRatio: '16/10',
              borderRadius: '16px',
              border: '2px dashed var(--color-outline-variant)',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '12px',
              cursor: 'pointer',
              background: 'rgba(19,27,46,0.3)',
              position: 'relative',
              overflow: 'hidden',
              transition: 'border-color 0.2s',
            }}
          >
            <div
              style={{
                width: '52px',
                height: '52px',
                borderRadius: '50%',
                background: 'var(--color-surface-container-highest)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition: 'background 0.2s',
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: '28px', color: 'var(--color-on-surface-variant)' }}>add</span>
            </div>
            <span
              style={{
                fontFamily: 'var(--font-geist)',
                fontSize: '16px',
                fontWeight: 600,
                color: 'var(--color-on-surface-variant)',
                transition: 'color 0.2s',
              }}
            >
              New Project
            </span>
          </motion.button>

          {/* Real Project Cards */}
          {data?.items?.map((project, idx) => (
            <motion.div key={project.id} variants={cardVariant}>
              <Link
                href={`/projects/${project.id}`}
                style={{ textDecoration: 'none', display: 'block' }}
              >
                <motion.div
                  whileHover={{ y: -4, boxShadow: '0 0 20px rgba(192,193,255,0.1)' }}
                  transition={{ duration: 0.2 }}
                  className="glass-card"
                  style={{
                    borderRadius: '16px',
                    padding: '20px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '16px',
                    position: 'relative',
                    overflow: 'hidden',
                    cursor: 'pointer',
                    border: '1px solid rgba(255,255,255,0.06)',
                    transition: 'all 0.2s',
                  }}
                >
                  {/* Card Header */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div
                      style={{
                        width: '40px',
                        height: '40px',
                        borderRadius: '10px',
                        background: 'var(--color-surface-container-highest)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      <span
                        className="material-symbols-outlined"
                        style={{ color: PROJECT_ICON_COLORS[idx % PROJECT_ICON_COLORS.length], fontSize: '20px' }}
                      >
                        {PROJECT_ICONS[idx % PROJECT_ICONS.length]}
                      </span>
                    </div>
                    <button
                      onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}
                      style={{
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        color: 'var(--color-on-surface-variant)',
                        padding: '4px',
                        borderRadius: '6px',
                        display: 'flex',
                        transition: 'color 0.2s',
                      }}
                    >
                      <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>more_vert</span>
                    </button>
                  </div>

                  {/* Card Content */}
                  <div style={{ flex: 1 }}>
                    <h3
                      style={{
                        fontFamily: 'var(--font-geist)',
                        fontSize: '15px',
                        fontWeight: 600,
                        color: 'var(--color-on-surface)',
                        margin: 0,
                        marginBottom: '6px',
                        overflow: 'hidden',
                        display: '-webkit-box',
                        WebkitLineClamp: 1,
                        WebkitBoxOrient: 'vertical',
                      }}
                    >
                      {project.name}
                    </h3>
                    <p
                      style={{
                        fontFamily: 'var(--font-inter)',
                        fontSize: '12px',
                        color: 'var(--color-on-surface-variant)',
                        margin: 0,
                        lineHeight: 1.5,
                        overflow: 'hidden',
                        display: '-webkit-box',
                        WebkitLineClamp: 2,
                        WebkitBoxOrient: 'vertical',
                      }}
                    >
                      Created {new Date(project.created_at).toLocaleDateString()}
                    </p>
                  </div>

                  {/* Card Footer */}
                  <div
                    style={{
                      paddingTop: '12px',
                      borderTop: '1px solid rgba(70,69,84,0.3)',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <span style={{ display: 'flex', alignItems: 'center', gap: '4px', fontFamily: 'var(--font-geist)', fontSize: '11px', color: 'var(--color-on-surface-variant)', letterSpacing: '0.05em' }}>
                      <span className="material-symbols-outlined" style={{ fontSize: '14px' }}>description</span>
                      0 Sources
                    </span>
                    <span style={{ fontFamily: 'var(--font-inter)', fontSize: '11px', color: 'var(--color-on-surface-variant)', fontStyle: 'italic' }}>
                      {new Date(project.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </motion.div>
              </Link>
            </motion.div>
          ))}

        </motion.div>
      )}

      {/* Genuine empty state — no projects yet */}
      {!isLoading && !data?.items?.length && (
        <motion.p
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.2 }}
          style={{
            marginTop: '24px',
            textAlign: 'center',
            fontFamily: 'var(--font-inter)',
            fontSize: 'var(--text-sm)',
            color: 'var(--color-on-surface-variant)',
          }}
        >
          You don&apos;t have any projects yet — create one above to start indexing sources and chatting with your material.
        </motion.p>
      )}
    </div>
  );
}

'use client';

import { useEffect, useMemo } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth-context';
import { motion } from 'framer-motion';
import { Spinner } from '@/design-system';

const RAIL_WIDTH = 88;

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated, isLoading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  // ── Auth guard ────────────────────────────────────────────────────────
  // Every route under this layout is private. Without this, anyone could
  // open /projects/<id>/chat directly and see the shell (with a null user)
  // instead of being sent to /login.
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace('/login');
    }
  }, [isLoading, isAuthenticated, router]);

  // The app only has three real destinations. Indexing & Chat are scoped to
  // whichever project the person is currently inside, so we read the
  // project id out of the URL rather than hardcoding a dead '#' link.
  const activeProjectId = useMemo(() => {
    const match = pathname.match(/^\/projects\/([^/]+)/);
    return match ? match[1] : null;
  }, [pathname]);

  const navItems = [
    { key: 'projects', icon: 'folder_special', label: 'Projects', href: '/projects', enabled: true, active: pathname === '/projects' },
    {
      key: 'indexing',
      icon: 'database',
      label: 'Indexing',
      href: activeProjectId ? `/projects/${activeProjectId}/indexing` : null,
      enabled: Boolean(activeProjectId),
      active: Boolean(activeProjectId) && pathname.endsWith('/indexing'),
    },
    {
      key: 'chat',
      icon: 'forum',
      label: 'Chat',
      href: activeProjectId ? `/projects/${activeProjectId}/chat` : null,
      enabled: Boolean(activeProjectId),
      active: Boolean(activeProjectId) && pathname.endsWith('/chat'),
    },
  ];

  const handleLogout = () => {
    logout();
    router.replace('/login');
  };

  if (isLoading || !isAuthenticated) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--color-background)' }}>
        <Spinner size={32} color="var(--color-primary)" />
      </div>
    );
  }

  return (
    <div style={{ minHeight: '100vh', background: 'var(--color-background)', display: 'flex' }}>
      {/* ── App Rail (3 destinations only) ───────────────────────────── */}
      <motion.aside
        initial={{ x: -RAIL_WIDTH, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
        className="glass-nav"
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          height: '100vh',
          width: `${RAIL_WIDTH}px`,
          zIndex: 50,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          padding: '16px 0',
          gap: '8px',
        }}
      >
        {/* Brand mark */}
        <Link href="/projects" style={{ textDecoration: 'none' }}>
          <div
            title="archadiLM"
            style={{
              width: '40px',
              height: '40px',
              borderRadius: '10px',
              background: 'var(--color-primary-container)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '20px',
            }}
          >
            <span className="material-symbols-outlined" style={{ color: 'var(--color-on-primary-container)', fontSize: '22px' }}>memory</span>
          </div>
        </Link>

        <nav style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '6px', width: '100%' }}>
          {navItems.map((item) => {
            const content = (
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: '4px',
                  padding: '10px 4px',
                  width: '72px',
                  borderRadius: '12px',
                  color: item.active ? 'var(--color-secondary)' : item.enabled ? 'var(--color-on-surface-variant)' : 'var(--color-ink-faint)',
                  background: item.active ? 'rgba(0,165,114,0.14)' : 'transparent',
                  cursor: item.enabled ? 'pointer' : 'not-allowed',
                  opacity: item.enabled ? 1 : 0.45,
                  transition: 'all 0.15s',
                }}
              >
                <span className="material-symbols-outlined" style={{ fontSize: '22px', fontVariationSettings: item.active ? "'FILL' 1" : "'FILL' 0" }}>
                  {item.icon}
                </span>
                <span style={{ fontFamily: 'var(--font-geist)', fontSize: '10px', fontWeight: 600, letterSpacing: '0.02em' }}>
                  {item.label}
                </span>
              </div>
            );

            if (!item.enabled || !item.href) {
              return (
                <div key={item.key} title="Open a project first">
                  {content}
                </div>
              );
            }

            return (
              <Link key={item.key} href={item.href} style={{ textDecoration: 'none' }}>
                {content}
              </Link>
            );
          })}
        </nav>

        {/* Footer: user + logout */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px', paddingTop: '12px', borderTop: '1px solid rgba(70,69,84,0.3)', width: '100%' }}>
          {user && (
            <div
              title={user.full_name || user.email}
              style={{
                width: '32px',
                height: '32px',
                borderRadius: '50%',
                background: 'var(--color-surface-container-highest)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '12px',
                fontWeight: 700,
                color: 'var(--color-primary)',
                fontFamily: 'var(--font-geist)',
              }}
            >
              {(user.full_name || user.email || '?').charAt(0).toUpperCase()}
            </div>
          )}
          <button
            onClick={handleLogout}
            title="Sign out"
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: '2px',
              padding: '6px',
              background: 'none',
              border: 'none',
              color: 'var(--color-error)',
              cursor: 'pointer',
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: '20px' }}>logout</span>
          </button>
        </div>
      </motion.aside>

      {/* ── Main Content Area ─────────────────────────────────────────── */}
      <div style={{ flex: 1, marginLeft: `${RAIL_WIDTH}px`, display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
        <main style={{ flex: 1, padding: '32px 40px' }}>
          {children}
        </main>
      </div>
    </div>
  );
}

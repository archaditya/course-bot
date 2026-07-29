'use client';

import { useEffect } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth-context';
import { motion } from 'framer-motion';
import { Spinner } from '@/design-system';

const RAIL_WIDTH = 64;

// The app now has exactly one real destination — chat — so the rail is
// just brand + account, not a nav bar. Conversations and sources live
// inside the chat page's own sidebars, not up here.
export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated, isLoading, logout } = useAuth();
  const router = useRouter();

  // ── Auth guard ────────────────────────────────────────────────────────
  // Every route under this layout is private. Without this, anyone could
  // open /chat directly and see the shell (with a null user) instead of
  // being sent to /login.
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace('/login');
    }
  }, [isLoading, isAuthenticated, router]);

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
    <div style={{ height: '100vh', background: 'var(--color-background)', display: 'flex', overflow: 'hidden' }}>
      {/* ── App Rail: brand + account only ───────────────────────────── */}
      <motion.aside
        initial={{ x: -RAIL_WIDTH, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
        className="glass-nav"
        style={{
          flexShrink: 0,
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
        <Link href="/chat" style={{ textDecoration: 'none' }}>
          <div
            title="archadiLM"
            style={{
              width: '38px',
              height: '38px',
              borderRadius: 'var(--radius-md)',
              background: 'var(--color-primary-container)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '8px',
            }}
          >
            <span className="material-symbols-outlined" style={{ color: 'var(--color-on-primary-container)', fontSize: '20px' }}>auto_stories</span>
          </div>
        </Link>

        <div style={{ flex: 1 }} />

        {/* Footer: user + logout */}
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px', paddingTop: '12px', borderTop: '1px solid var(--color-outline-variant)', width: '100%' }}>
          {user && (
            <div
              title={user.full_name || user.email}
              style={{
                width: '30px',
                height: '30px',
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
            <span className="material-symbols-outlined" style={{ fontSize: '19px' }}>logout</span>
          </button>
        </div>
      </motion.aside>

      {/* ── Main Content Area — full-bleed; the chat page owns its own
           internal sidebars and padding ───────────────────────────── */}
      <div style={{ flex: 1, minWidth: 0, height: '100vh', overflow: 'hidden' }}>
        {children}
      </div>
    </div>
  );
}

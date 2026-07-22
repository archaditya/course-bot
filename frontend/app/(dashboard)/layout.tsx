'use client';

import Link from 'next/link';
import { useAuth } from '@/lib/auth-context';

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();

  return (
    <div style={{ minHeight: '100vh', background: 'var(--color-paper)' }}>
      {/* Top Navbar */}
      <header
        style={{
          height: '64px',
          background: 'var(--color-surface)',
          borderBottom: '1px solid var(--color-border-subtle)',
          padding: '0 var(--space-8)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Link href="/" style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)', fontWeight: 700, color: 'var(--color-ink)' }}>
          archadi<em style={{ color: 'var(--color-accent)', fontStyle: 'normal' }}>LM</em>
        </Link>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)' }}>
          <Link href="/" style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-sm)', fontWeight: 500 }}>
            Projects
          </Link>

          {user && (
            <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-ink-muted)' }}>
              👤 {user.email}
            </span>
          )}

          <button
            onClick={logout}
            style={{
              padding: '6px 14px',
              background: 'none',
              border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-md)',
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              color: 'var(--color-ink-muted)',
            }}
          >
            Sign out
          </button>
        </div>
      </header>

      {/* Main Page Area */}
      <main style={{ padding: 'var(--space-8)' }}>
        {children}
      </main>
    </div>
  );
}

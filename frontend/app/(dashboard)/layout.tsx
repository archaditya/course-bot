'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { clearTokens } from '@/lib/api';

const navItems = [
  { href: '/', label: 'Dashboard', icon: '⊞' },
  { href: '/projects', label: 'Projects', icon: '📁' },
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();

  const handleLogout = () => {
    clearTokens();
    router.push('/login');
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: 'var(--color-paper)' }}>
      {/* Sidebar */}
      <aside style={{
        width: 'var(--sidebar-width)',
        background: 'var(--color-surface)',
        borderRight: '1px solid var(--color-border-subtle)',
        display: 'flex',
        flexDirection: 'column',
        position: 'fixed',
        top: 0,
        left: 0,
        height: '100vh',
        zIndex: 10,
      }}>
        <div style={{
          padding: 'var(--space-5) var(--space-4)',
          borderBottom: '1px solid var(--color-border-subtle)',
        }}>
          <Link href="/" style={{
            fontFamily: 'var(--font-display)',
            fontSize: 'var(--text-lg)',
            fontWeight: 700,
            color: 'var(--color-ink)',
          }}>
            Course Assistant
          </Link>
        </div>

        <nav style={{ flex: 1, padding: 'var(--space-4) var(--space-2)' }}>
          {navItems.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--space-3)',
                  padding: 'var(--space-2) var(--space-3)',
                  borderRadius: 'var(--radius-md)',
                  fontSize: 'var(--text-sm)',
                  fontWeight: active ? 600 : 400,
                  color: active ? 'var(--color-accent)' : 'var(--color-ink-secondary)',
                  background: active ? 'var(--color-accent-light)' : 'transparent',
                  transition: 'all var(--transition-fast)',
                  marginBottom: 'var(--space-1)',
                }}
              >
                <span>{item.icon}</span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div style={{
          padding: 'var(--space-4)',
          borderTop: '1px solid var(--color-border-subtle)',
        }}>
          <button
            onClick={handleLogout}
            id="btn-logout"
            style={{
              width: '100%',
              padding: 'var(--space-2) var(--space-3)',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--color-ink-muted)',
              fontSize: 'var(--text-sm)',
              textAlign: 'left',
              borderRadius: 'var(--radius-md)',
            }}
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main style={{
        flex: 1,
        marginLeft: 'var(--sidebar-width)',
        padding: 'var(--space-8)',
        maxWidth: `calc(100vw - var(--sidebar-width))`,
      }}>
        {children}
      </main>
    </div>
  );
}

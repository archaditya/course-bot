'use client';

import Link from 'next/link';
import { useAuth } from '@/lib/auth-context';
import { motion } from 'framer-motion';
import { useRouter } from 'next/navigation';

const NAV_ITEMS = [
  { icon: 'home', label: 'Home', href: '/' },
  { icon: 'folder_special', label: 'Projects', href: '/', filled: true },
  { icon: 'history', label: 'Recent', href: '#' },
  { icon: 'local_library', label: 'Library', href: '#' },
  { icon: 'settings', label: 'Settings', href: '#' },
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();

  const router = useRouter();

  const handleLogout = () => {
    logout();
    router.replace('/login');
  };

  return (
    <div style={{ minHeight: '100vh', background: 'var(--color-background)', display: 'flex' }}>
      {/* ── Left Sidebar ──────────────────────────────────────────── */}
      <motion.aside
        initial={{ x: -280, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        className="glass-nav"
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          height: '100vh',
          width: '280px',
          zIndex: 50,
          display: 'flex',
          flexDirection: 'column',
          padding: '16px 0',
          gap: '8px',
          overflowY: 'auto',
        }}
      >
        {/* Brand Header */}
        <div style={{ padding: '8px 16px', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div
              style={{
                width: '40px',
                height: '40px',
                borderRadius: '10px',
                background: 'var(--color-primary-container)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <span className="material-symbols-outlined" style={{ color: 'var(--color-on-primary-container)', fontSize: '22px' }}>memory</span>
            </div>
            <div>
              <h1 style={{ fontFamily: 'var(--font-geist)', fontSize: '18px', fontWeight: 700, color: 'var(--color-on-surface)', margin: 0 }}>
                archadi<em style={{ color: 'var(--color-primary)', fontStyle: 'normal' }}>LM</em>
              </h1>
              <p style={{ fontFamily: 'var(--font-inter)', fontSize: '11px', color: 'var(--color-on-surface-variant)', margin: 0 }}>
                Knowledge Workspace
              </p>
            </div>
          </div>
        </div>

        {/* Navigation */}
        <nav style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {NAV_ITEMS.map((item, i) => {
            const isActive = i === 1;
            return (
              <Link
                key={item.label}
                href={item.href}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '14px',
                  padding: '10px 16px',
                  fontFamily: 'var(--font-geist)',
                  fontSize: '12px',
                  fontWeight: 500,
                  letterSpacing: '0.05em',
                  color: isActive ? 'var(--color-secondary)' : 'var(--color-on-surface-variant)',
                  background: isActive ? 'rgba(0,165,114,0.12)' : 'transparent',
                  borderLeft: isActive ? '3px solid var(--color-secondary)' : '3px solid transparent',
                  transition: 'all 0.2s',
                  textDecoration: 'none',
                }}
              >
                <span
                  className="material-symbols-outlined"
                  style={{
                    fontSize: '20px',
                    fontVariationSettings: item.filled && isActive ? "'FILL' 1" : "'FILL' 0",
                  }}
                >
                  {item.icon}
                </span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* Footer Actions */}
        <div style={{ borderTop: '1px solid rgba(70,69,84,0.3)', paddingTop: '12px', display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {user && (
            <div style={{ padding: '8px 16px', fontSize: '12px', color: 'var(--color-on-surface-variant)', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              👤 {user.full_name || user.email}
            </div>
          )}
          {user && (
            <button
              onClick={handleLogout}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '14px',
                padding: '10px 16px',
                fontFamily: 'var(--font-geist)',
                fontSize: '12px',
                fontWeight: 500,
                color: 'var(--color-error)',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: '20px' }}>logout</span>
              Sign Out
            </button>
          )}
        </div>
      </motion.aside>

      {/* ── Main Content Area ─────────────────────────────────────── */}
      <div style={{ flex: 1, marginLeft: '280px', display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
        <main style={{ flex: 1, padding: '32px 40px' }}>
          {children}
        </main>
      </div>
    </div>
  );
}

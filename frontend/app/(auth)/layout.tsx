import type { Metadata } from 'next';

export const metadata: Metadata = { title: 'Sign in' };

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      minHeight: '100vh',
      background: 'var(--color-paper)',
      display: 'grid',
      placeItems: 'center',
      padding: 'var(--space-8)',
    }}>
      <div style={{ width: '100%', maxWidth: '420px' }}>
        <div style={{
          textAlign: 'center',
          marginBottom: 'var(--space-8)',
        }}>
          <a href="/" style={{
            fontFamily: 'var(--font-display)',
            fontSize: 'var(--text-2xl)',
            fontWeight: 700,
            color: 'var(--color-ink)',
          }}>
            Course Assistant
          </a>
        </div>
        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-2xl)',
          padding: 'var(--space-8)',
          boxShadow: 'var(--shadow-lg)',
        }}>
          {children}
        </div>
      </div>
    </div>
  );
}

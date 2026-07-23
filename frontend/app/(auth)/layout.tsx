import type { Metadata } from 'next';

export const metadata: Metadata = { title: 'Sign in' };

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        minHeight: '100vh',
        background: 'var(--color-background)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Atmospheric AI Aura Glows */}
      <div
        className="ai-aura"
        style={{
          position: 'absolute',
          top: '15%',
          left: '20%',
          width: '400px',
          height: '400px',
          background: 'radial-gradient(circle, rgba(192,193,255,0.2) 0%, transparent 70%)',
          pointerEvents: 'none',
        }}
      />
      <div
        className="ai-aura-drift"
        style={{
          position: 'absolute',
          bottom: '15%',
          right: '20%',
          width: '520px',
          height: '520px',
          background: 'radial-gradient(circle, rgba(78,222,163,0.12) 0%, transparent 70%)',
          animationDelay: '-4s',
          pointerEvents: 'none',
        }}
      />
      <div
        className="ai-aura"
        style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: '600px',
          height: '600px',
          background: 'radial-gradient(circle, rgba(160,120,255,0.06) 0%, transparent 70%)',
          animationDelay: '-2s',
          pointerEvents: 'none',
        }}
      />

      {/* Powered-by badge */}
      <div
        className="glass-card"
        style={{
          position: 'fixed',
          bottom: '24px',
          right: '24px',
          padding: '8px 16px',
          borderRadius: '9999px',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <span className="material-symbols-outlined" style={{ color: 'var(--color-secondary)', fontSize: '16px' }}>bolt</span>
        <span style={{ fontFamily: 'var(--font-geist)', fontSize: '11px', fontWeight: 500, color: 'var(--color-on-surface)', letterSpacing: '0.05em' }}>
          Powered by GPT-4o
        </span>
      </div>

      {/* Auth Card Area */}
      <div style={{ position: 'relative', zIndex: 10, width: '100%', maxWidth: '440px', padding: '0 16px' }}>
        {/* Brand Header */}
        <div style={{ textAlign: 'center', marginBottom: '32px' }}>
          <h1
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '48px',
              fontWeight: 700,
              letterSpacing: '-0.02em',
              color: 'var(--color-primary)',
              lineHeight: 1.1,
              marginBottom: '8px',
            }}
          >
            ArchadiLM
          </h1>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '14px', color: 'var(--color-on-surface-variant)' }}>
            Your AI knowledge workspace.
          </p>
        </div>

        {children}

        {/* Footer */}
        <div style={{ marginTop: '32px', textAlign: 'center' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', marginBottom: '12px' }}>
            <span
              style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                background: 'var(--color-secondary)',
                display: 'inline-block',
                animation: 'pulse 2s infinite',
              }}
            />
            <p style={{ fontFamily: 'var(--font-geist)', fontSize: '11px', fontWeight: 500, color: 'var(--color-secondary)', letterSpacing: '0.05em' }}>
              All systems operational
            </p>
          </div>
          <p style={{ fontFamily: 'var(--font-geist)', fontSize: '11px', color: 'var(--color-on-surface-variant)', opacity: 0.5, letterSpacing: '0.05em' }}>
            © 2024 ArchadiLM Corp. All rights reserved.
          </p>
        </div>
      </div>
    </div>
  );
}

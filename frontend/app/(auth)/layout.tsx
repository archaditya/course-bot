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
      {/* Single restrained vignette — no drifting gradient blobs */}
      <div
        style={{
          position: 'absolute',
          top: '-10%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '900px',
          height: '500px',
          background: 'radial-gradient(ellipse, rgba(217,164,65,0.07) 0%, transparent 65%)',
          pointerEvents: 'none',
        }}
      />

      {/* Faint ruled-notebook lines in the margins */}
      <div
        aria-hidden
        style={{
          position: 'absolute',
          inset: 0,
          backgroundImage:
            'repeating-linear-gradient(to bottom, transparent 0, transparent 39px, var(--color-outline-variant) 39px, var(--color-outline-variant) 40px)',
          opacity: 0.35,
          maskImage: 'radial-gradient(ellipse at center, transparent 30%, black 100%)',
          WebkitMaskImage: 'radial-gradient(ellipse at center, transparent 30%, black 100%)',
          pointerEvents: 'none',
        }}
      />

      {/* Auth Card Area */}
      <div style={{ position: 'relative', zIndex: 10, width: '100%', maxWidth: '420px', padding: '0 16px' }}>
        {/* Brand Header */}
        <div style={{ textAlign: 'center', marginBottom: '36px' }}>
          <div
            style={{
              width: '44px',
              height: '44px',
              borderRadius: 'var(--radius-md)',
              background: 'var(--color-primary-container)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 18px',
            }}
          >
            <span className="material-symbols-outlined" style={{ color: 'var(--color-on-primary-container)', fontSize: '24px' }}>
              auto_stories
            </span>
          </div>
          <h1
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: '30px',
              fontWeight: 700,
              letterSpacing: '-0.02em',
              color: 'var(--color-on-surface)',
              lineHeight: 1.1,
              marginBottom: '8px',
            }}
          >
            ArchadiLM
          </h1>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '13.5px', color: 'var(--color-on-surface-variant)' }}>
            Every answer, traced back to your sources.
          </p>
        </div>

        {children}

        {/* Footer */}
        <div style={{ marginTop: '28px', textAlign: 'center' }}>
          <p style={{ fontFamily: 'var(--font-geist)', fontSize: '11px', color: 'var(--color-on-surface-variant)', opacity: 0.6, letterSpacing: '0.03em' }}>
            © 2026 ArchadiLM Corp. All rights reserved.
          </p>
        </div>
      </div>
    </div>
  );
}

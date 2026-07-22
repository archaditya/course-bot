'use client';

import Link from 'next/link';

export default function LandingPage() {
  return (
    <main style={{
      minHeight: '100vh',
      background: 'var(--color-paper)',
      display: 'flex',
      flexDirection: 'column',
    }}>
      {/* Nav */}
      <nav style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: 'var(--space-4) var(--space-8)',
        borderBottom: '1px solid var(--color-border-subtle)',
      }}>
        <span style={{
          fontFamily: 'var(--font-display)',
          fontWeight: 700,
          fontSize: 'var(--text-xl)',
          color: 'var(--color-ink)',
        }}>
          archadi<em style={{ color: 'var(--color-accent)', fontStyle: 'normal' }}>LM</em>
        </span>
        <div style={{ display: 'flex', gap: 'var(--space-4)', alignItems: 'center' }}>
          <Link href="/login" style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-sm)', fontWeight: 500 }}>
            Log in
          </Link>
          <Link href="/signup" style={{
            padding: '8px 18px',
            background: 'var(--color-accent)',
            color: '#fff',
            borderRadius: 'var(--radius-md)',
            fontWeight: 500,
            fontSize: 'var(--text-sm)',
          }}>
            Get started
          </Link>
        </div>
      </nav>

      {/* Hero */}
      <section style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 'var(--space-20) var(--space-8)',
        textAlign: 'center',
        maxWidth: '800px',
        margin: '0 auto',
      }}>
        <span style={{
          display: 'inline-block',
          padding: '4px 14px',
          background: 'var(--color-accent-light)',
          border: '1px solid var(--color-accent-border)',
          borderRadius: 'var(--radius-full)',
          color: 'var(--color-accent)',
          fontSize: 'var(--text-sm)',
          fontWeight: 600,
          marginBottom: 'var(--space-6)',
          fontFamily: 'var(--font-ui)',
          letterSpacing: '0.02em',
        }}>
          Your AI study companion
        </span>

        <h1 style={{
          fontFamily: 'var(--font-display)',
          fontSize: 'clamp(2.25rem, 5vw, 3.75rem)',
          fontWeight: 700,
          lineHeight: 1.2,
          color: 'var(--color-ink)',
          marginBottom: 'var(--space-6)',
        }}>
          Chat with <em style={{ fontStyle: 'italic', color: 'var(--color-accent)' }}>any</em> learning material.
        </h1>

        <p style={{
          fontSize: 'var(--text-lg)',
          color: 'var(--color-ink-secondary)',
          lineHeight: 1.8,
          marginBottom: 'var(--space-10)',
          maxWidth: '560px',
        }}>
          Upload PDFs, videos, web pages, or paste text — ask any question and get
          grounded answers with citations that jump straight to the source.
        </p>

        <Link
          href="/signup"
          id="cta-signup"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            padding: '14px 32px',
            background: 'var(--color-accent)',
            color: '#fff',
            borderRadius: 'var(--radius-lg)',
            fontWeight: 600,
            fontSize: 'var(--text-lg)',
            fontFamily: 'var(--font-ui)',
            boxShadow: 'var(--shadow-md)',
            transition: 'background var(--transition-fast), box-shadow var(--transition-fast)',
          }}
        >
          Add your first source →
        </Link>
      </section>
    </main>
  );
}

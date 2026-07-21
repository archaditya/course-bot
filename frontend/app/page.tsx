import Link from 'next/link';
import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Course Assistant — Chat with your course material',
};

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
          Course Assistant
        </span>
        <div style={{ display: 'flex', gap: 'var(--space-4)', alignItems: 'center' }}>
          <Link href="/login" style={{ color: 'var(--color-ink-secondary)', fontSize: 'var(--text-sm)' }}>
            Log in
          </Link>
          <Link href="/signup" style={{
            padding: '8px 16px',
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
          Study smarter
        </span>

        <h1 style={{
          fontFamily: 'var(--font-display)',
          fontSize: 'clamp(2.25rem, 5vw, 3.75rem)',
          fontWeight: 700,
          lineHeight: 1.2,
          color: 'var(--color-ink)',
          marginBottom: 'var(--space-6)',
        }}>
          Have a conversation<br />
          <em style={{ fontStyle: 'italic', color: 'var(--color-accent)' }}>with your course.</em>
        </h1>

        <p style={{
          fontSize: 'var(--text-lg)',
          color: 'var(--color-ink-secondary)',
          lineHeight: 1.8,
          marginBottom: 'var(--space-10)',
          maxWidth: '560px',
        }}>
          Upload transcripts, slides, or video and ask any question.
          Get grounded answers with citations that jump straight to the exact timestamp.
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
          Upload your first course →
        </Link>

        <p style={{
          marginTop: 'var(--space-4)',
          fontSize: 'var(--text-sm)',
          color: 'var(--color-ink-muted)',
        }}>
          Free to start. No credit card required.
        </p>
      </section>

      {/* Features grid */}
      <section style={{
        padding: 'var(--space-16) var(--space-8)',
        background: 'var(--color-paper-subtle)',
        borderTop: '1px solid var(--color-border-subtle)',
      }}>
        <div style={{
          maxWidth: '900px',
          margin: '0 auto',
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
          gap: 'var(--space-8)',
        }}>
          {[
            { icon: '💬', title: 'Streaming answers', desc: 'Responses appear token by token — never a frozen loading screen.' },
            { icon: '📌', title: 'Grounded citations', desc: 'Every claim links to its source with a one-click "jump to timestamp" button.' },
            { icon: '🔍', title: 'Hybrid retrieval', desc: 'Semantic search + keyword matching so exact terms are never lost.' },
          ].map((f) => (
            <div key={f.title} style={{
              background: 'var(--color-surface)',
              border: '1px solid var(--color-border-subtle)',
              borderRadius: 'var(--radius-xl)',
              padding: 'var(--space-6)',
              boxShadow: 'var(--shadow-sm)',
            }}>
              <div style={{ fontSize: '2rem', marginBottom: 'var(--space-3)' }}>{f.icon}</div>
              <h3 style={{
                fontFamily: 'var(--font-display)',
                fontSize: 'var(--text-xl)',
                marginBottom: 'var(--space-2)',
              }}>{f.title}</h3>
              <p style={{ color: 'var(--color-ink-secondary)', lineHeight: 1.7 }}>{f.desc}</p>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}

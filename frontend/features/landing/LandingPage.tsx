'use client';

import Link from 'next/link';
import { motion } from 'framer-motion';

const fadeUp = {
  hidden: { opacity: 0, y: 30 },
  visible: (delay = 0) => ({
    opacity: 1,
    y: 0,
    transition: { duration: 0.7, delay, ease: 'easeOut' as const },
  }),
};

const stagger = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.12 } },
};

export default function LandingPage() {
  return (
    <main
      style={{
        minHeight: '100vh',
        background: 'var(--color-background)',
        color: 'var(--color-on-surface)',
        fontFamily: 'var(--font-inter)',
        overflowX: 'hidden',
      }}
    >
      {/* ── Atmospheric Aura Background ──────────────────────────────── */}
      <div style={{ position: 'fixed', inset: 0, zIndex: 0, pointerEvents: 'none' }}>
        <div
          className="ai-aura"
          style={{
            position: 'absolute',
            top: '-10%',
            left: '30%',
            width: '700px',
            height: '600px',
            background: 'radial-gradient(circle, rgba(192,193,255,0.25) 0%, transparent 70%)',
          }}
        />
        <div
          className="ai-aura-drift"
          style={{
            position: 'absolute',
            top: '20%',
            right: '-5%',
            width: '450px',
            height: '450px',
            background: 'radial-gradient(circle, rgba(78,222,163,0.15) 0%, transparent 70%)',
            animationDelay: '-6s',
          }}
        />
        <div
          className="ai-aura"
          style={{
            position: 'absolute',
            bottom: '10%',
            left: '-5%',
            width: '400px',
            height: '400px',
            background: 'radial-gradient(circle, rgba(160,120,255,0.12) 0%, transparent 70%)',
            animationDelay: '-3s',
          }}
        />
      </div>

      {/* ── Top Navigation ───────────────────────────────────────────── */}
      <motion.header
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: 'easeOut' }}
        className="glass-nav"
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          zIndex: 50,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 var(--spacing-lg)',
          height: '64px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-md)' }}>
          <div
            style={{
              width: '36px',
              height: '36px',
              borderRadius: '10px',
              background: 'var(--color-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <span className="material-symbols-outlined" style={{ color: 'var(--color-on-primary)', fontSize: '20px' }}>
              memory
            </span>
          </div>
          <span
            style={{
              fontFamily: 'var(--font-geist)',
              fontWeight: 700,
              fontSize: '18px',
              color: 'var(--color-on-surface)',
            }}
          >
            ArchadiLM
          </span>
          <nav style={{ display: 'flex', gap: 'var(--spacing-lg)', marginLeft: '32px' }}>
            {['Dashboard', 'Sources', 'Workspace'].map((item, i) => (
              <a
                key={item}
                href="#"
                style={{
                  fontSize: '12px',
                  fontFamily: 'var(--font-geist)',
                  fontWeight: i === 0 ? 700 : 500,
                  color: i === 0 ? 'var(--color-primary)' : 'var(--color-on-surface-variant)',
                  letterSpacing: '0.05em',
                  textDecoration: i === 0 ? 'none' : 'none',
                  borderBottom: i === 0 ? '2px solid var(--color-primary)' : 'none',
                  paddingBottom: i === 0 ? '2px' : '0',
                  transition: 'color 0.2s',
                }}
              >
                {item}
              </a>
            ))}
          </nav>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-md)' }}>
          <Link
            href="/login"
            style={{
              fontSize: '12px',
              fontFamily: 'var(--font-geist)',
              color: 'var(--color-on-surface-variant)',
              fontWeight: 500,
              letterSpacing: '0.05em',
            }}
          >
            Sign In
          </Link>
          <Link
            href="/signup"
            style={{
              padding: '8px 18px',
              background: 'var(--color-primary)',
              color: 'var(--color-on-primary)',
              borderRadius: '8px',
              fontFamily: 'var(--font-geist)',
              fontSize: '12px',
              fontWeight: 600,
              letterSpacing: '0.05em',
              boxShadow: '0 4px 20px rgba(192,193,255,0.25)',
              transition: 'all 0.2s',
              display: 'inline-block',
            }}
          >
            Get Started Free
          </Link>
        </div>
      </motion.header>

      {/* ── Hero Section ─────────────────────────────────────────────── */}
      <section
        style={{
          paddingTop: '140px',
          paddingBottom: '80px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          textAlign: 'center',
          padding: '140px var(--spacing-md) 80px',
          position: 'relative',
          zIndex: 1,
        }}
      >
        <motion.div
          initial="hidden"
          animate="visible"
          variants={stagger}
          style={{ maxWidth: '860px', margin: '0 auto' }}
        >
          {/* Badge */}
          <motion.div variants={fadeUp} custom={0}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '8px',
                padding: '4px 14px',
                background: 'var(--color-surface-container-high)',
                borderRadius: '9999px',
                border: '1px solid var(--color-outline-variant)',
                marginBottom: '24px',
              }}
            >
              <span
                style={{
                  width: '8px',
                  height: '8px',
                  borderRadius: '50%',
                  background: 'var(--color-secondary)',
                  animation: 'pulse 2s infinite',
                }}
              />
              <span
                style={{
                  fontFamily: 'var(--font-geist)',
                  fontSize: '12px',
                  fontWeight: 500,
                  color: 'var(--color-secondary)',
                  letterSpacing: '0.08em',
                  textTransform: 'uppercase',
                }}
              >
                Now with GPT-4o Integration
              </span>
            </div>
          </motion.div>

          {/* Headline */}
          <motion.h1
            variants={fadeUp}
            custom={0.1}
            style={{
              fontFamily: 'var(--font-geist)',
              fontSize: 'clamp(2.5rem, 6vw, 4rem)',
              fontWeight: 700,
              lineHeight: 1.1,
              letterSpacing: '-0.02em',
              color: 'var(--color-on-surface)',
              marginBottom: '24px',
            }}
          >
            Your{' '}
            <span className="shimmer-text">Second Brain</span>,{' '}
            <br />
            Supercharged by AI
          </motion.h1>

          {/* Subhead */}
          <motion.p
            variants={fadeUp}
            custom={0.2}
            style={{
              fontFamily: 'var(--font-inter)',
              fontSize: '16px',
              lineHeight: 1.7,
              color: 'var(--color-on-surface-variant)',
              maxWidth: '600px',
              margin: '0 auto 40px',
            }}
          >
            Connect your documents, web research, and personal notes into a unified knowledge graph.
            ArchadiLM indexes everything so you can chat with your context instantly.
          </motion.p>

          {/* CTAs */}
          <motion.div
            variants={fadeUp}
            custom={0.3}
            style={{ display: 'flex', gap: '16px', justifyContent: 'center', flexWrap: 'wrap' }}
          >
            <Link
              href="/signup"
              style={{
                padding: '14px 36px',
                background: 'var(--color-primary)',
                color: 'var(--color-on-primary)',
                borderRadius: '12px',
                fontFamily: 'var(--font-geist)',
                fontSize: '16px',
                fontWeight: 600,
                boxShadow: '0 8px 30px rgba(192,193,255,0.3)',
                transition: 'all 0.2s',
                display: 'inline-block',
              }}
            >
              Get Started Free
            </Link>
            <a
              href="#demo"
              className="glass-card-hover"
              style={{
                padding: '14px 36px',
                borderRadius: '12px',
                fontFamily: 'var(--font-geist)',
                fontSize: '16px',
                fontWeight: 500,
                color: 'var(--color-on-surface)',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <span className="material-symbols-outlined" style={{ fontSize: '20px' }}>play_circle</span>
              Watch Demo
            </a>
          </motion.div>
        </motion.div>

        {/* ── Product Preview ──────────────────────────────────────── */}
        <motion.div
          initial={{ opacity: 0, y: 60, scale: 0.97 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          transition={{ duration: 1, delay: 0.5, ease: [0.22, 1, 0.36, 1] }}
          style={{
            marginTop: '64px',
            width: '100%',
            maxWidth: '1100px',
            position: 'relative',
          }}
        >
          <div
            className="glass-card"
            style={{
              borderRadius: '32px',
              padding: '12px',
              overflow: 'hidden',
              position: 'relative',
            }}
          >
            <div
              style={{
                position: 'absolute',
                inset: 0,
                background: 'linear-gradient(135deg, rgba(192,193,255,0.04) 0%, transparent 60%)',
                pointerEvents: 'none',
              }}
            />
            <div
              style={{
                borderRadius: '22px',
                overflow: 'hidden',
                border: '1px solid var(--color-outline-variant)',
                aspectRatio: '16/9',
                background: 'var(--color-surface-container-low)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {/* Workspace Preview Mockup */}
              <div style={{ width: '100%', height: '100%', display: 'flex', position: 'relative' }}>
                {/* Left sidebar preview */}
                <div
                  style={{
                    width: '30%',
                    background: 'var(--color-surface-container)',
                    borderRight: '1px solid var(--color-outline-variant)',
                    padding: '20px 16px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontFamily: 'var(--font-geist)', fontWeight: 600, color: 'var(--color-primary)', marginBottom: '16px' }}>Add Sources</div>
                  {['PDF', 'YouTube', 'URL', 'Text'].map((s) => (
                    <div
                      key={s}
                      style={{
                        padding: '8px 12px',
                        borderRadius: '8px',
                        background: 'rgba(192,193,255,0.05)',
                        border: '1px solid rgba(255,255,255,0.05)',
                        marginBottom: '8px',
                        fontSize: '12px',
                        fontFamily: 'var(--font-geist)',
                        color: 'var(--color-on-surface-variant)',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                      }}
                    >
                      <span className="material-symbols-outlined" style={{ fontSize: '16px', color: 'var(--color-primary)' }}>description</span>
                      {s}
                    </div>
                  ))}
                </div>
                {/* Main chat preview */}
                <div style={{ flex: 1, padding: '24px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
                  <div style={{ fontSize: '16px', fontFamily: 'var(--font-geist)', fontWeight: 600, color: 'var(--color-on-surface)' }}>Your AI Learning Companion</div>
                  <div
                    style={{
                      alignSelf: 'flex-end',
                      padding: '12px 16px',
                      borderRadius: '16px',
                      background: 'var(--color-surface-container-high)',
                      border: '1px solid var(--color-outline-variant)',
                      fontSize: '12px',
                      fontFamily: 'var(--font-inter)',
                      color: 'var(--color-on-surface)',
                      maxWidth: '70%',
                    }}
                  >
                    Explain how React hooks work with this course
                  </div>
                  <div
                    className="glass-panel"
                    style={{
                      padding: '16px',
                      borderRadius: '16px',
                      fontSize: '12px',
                      fontFamily: 'var(--font-inter)',
                      color: 'var(--color-on-surface-variant)',
                      lineHeight: 1.6,
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                      <div style={{ width: '20px', height: '20px', borderRadius: '50%', background: 'rgba(192,193,255,0.2)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <span className="material-symbols-outlined" style={{ fontSize: '12px', color: 'var(--color-primary)' }}>bolt</span>
                      </div>
                      <span style={{ fontSize: '10px', color: 'var(--color-secondary)', fontFamily: 'var(--font-geist)', fontWeight: 600 }}>AI Answer • High Confidence</span>
                    </div>
                    Based on Module 7, React hooks allow functional components to manage state and side effects. The <code>useState</code> hook tracks component state, while <code>useEffect</code> runs side effects after renders...
                  </div>
                </div>
              </div>
            </div>

            {/* Hover badge */}
            <div
              className="glass-card"
              style={{
                position: 'absolute',
                bottom: '32px',
                left: '32px',
                padding: '12px 16px',
                borderRadius: '12px',
                border: '1px solid rgba(192,193,255,0.2)',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <span
                className="material-symbols-outlined"
                style={{ color: 'var(--color-secondary)', fontSize: '18px', fontVariationSettings: "'FILL' 1" }}
              >
                auto_awesome
              </span>
              <span style={{ fontFamily: 'var(--font-geist)', fontSize: '12px', fontWeight: 500, color: 'var(--color-on-surface)' }}>
                Indexing 3 sources...
              </span>
              <div
                style={{
                  width: '60px',
                  height: '4px',
                  borderRadius: '9999px',
                  background: 'var(--color-surface-variant)',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    width: '65%',
                    height: '100%',
                    background: 'var(--color-secondary)',
                    borderRadius: '9999px',
                  }}
                />
              </div>
            </div>
          </div>
        </motion.div>
      </section>

      {/* ── Feature Grid (Bento) ─────────────────────────────────────── */}
      <motion.section
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: '-80px' }}
        variants={stagger}
        style={{
          maxWidth: '1200px',
          margin: '0 auto',
          padding: '40px var(--spacing-lg) 80px',
          display: 'grid',
          gridTemplateColumns: 'repeat(12, 1fr)',
          gap: '24px',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {/* Card 1: Source Indexing (8 cols) */}
        <motion.div
          variants={fadeUp}
          className="glass-card-hover"
          style={{
            gridColumn: 'span 8',
            borderRadius: '32px',
            padding: '40px',
            display: 'flex',
            flexDirection: 'column',
            gap: '24px',
            overflow: 'hidden',
            position: 'relative',
            minHeight: '240px',
          }}
        >
          <span className="material-symbols-outlined" style={{ color: 'var(--color-primary)', fontSize: '40px' }}>database</span>
          <div>
            <h3 style={{ fontFamily: 'var(--font-geist)', fontSize: '28px', fontWeight: 600, marginBottom: '12px', letterSpacing: '-0.01em' }}>
              Source Indexing
            </h3>
            <p style={{ fontFamily: 'var(--font-inter)', fontSize: '15px', lineHeight: 1.6, color: 'var(--color-on-surface-variant)', maxWidth: '420px' }}>
              Upload PDFs, Markdown, and Web URLs. Our proprietary RAG pipeline vectorizes your data for lightning-fast retrieval.
            </p>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: 'auto' }}>
            {['PDF_Process.py', 'Vector_DB_Init', 'Semantic_Search_Ready'].map((tag, i) => (
              <span
                key={tag}
                style={{
                  padding: '6px 14px',
                  borderRadius: '9999px',
                  background: 'var(--color-surface-container-high)',
                  border: '1px solid var(--color-outline-variant)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '12px',
                  color: i === 0 ? 'var(--color-secondary)' : i === 1 ? 'var(--color-primary)' : 'var(--color-tertiary)',
                }}
              >
                {tag}
              </span>
            ))}
          </div>
        </motion.div>

        {/* Card 2: Contextual Chat (4 cols) */}
        <motion.div
          variants={fadeUp}
          custom={0.15}
          className="glass-card-hover"
          style={{
            gridColumn: 'span 4',
            borderRadius: '32px',
            padding: '40px',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            textAlign: 'center',
            minHeight: '240px',
          }}
        >
          <div
            style={{
              width: '72px',
              height: '72px',
              borderRadius: '50%',
              background: 'rgba(0,165,114,0.12)',
              border: '1px solid rgba(78,222,163,0.3)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '20px',
            }}
          >
            <span
              className="material-symbols-outlined"
              style={{ color: 'var(--color-secondary)', fontSize: '32px', fontVariationSettings: "'FILL' 1" }}
            >
              forum
            </span>
          </div>
          <h3 style={{ fontFamily: 'var(--font-geist)', fontSize: '22px', fontWeight: 600, marginBottom: '12px' }}>Contextual Chat</h3>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '13px', color: 'var(--color-on-surface-variant)', lineHeight: 1.6 }}>
            Ask questions across your entire library with full context awareness.
          </p>
        </motion.div>

        {/* Card 3: Smart Citations (4 cols) */}
        <motion.div
          variants={fadeUp}
          custom={0.3}
          className="glass-card-hover"
          style={{
            gridColumn: 'span 4',
            borderRadius: '32px',
            padding: '40px',
            minHeight: '220px',
          }}
        >
          <span className="material-symbols-outlined" style={{ color: 'var(--color-tertiary)', fontSize: '32px', marginBottom: '16px', display: 'block' }}>format_quote</span>
          <h3 style={{ fontFamily: 'var(--font-geist)', fontSize: '22px', fontWeight: 600, marginBottom: '12px' }}>Smart Citations</h3>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '13px', color: 'var(--color-on-surface-variant)', lineHeight: 1.6, marginBottom: '16px' }}>
            Every answer is linked back to your original sources.
          </p>
          <div
            style={{
              padding: '12px',
              background: 'rgba(45,52,73,0.4)',
              borderRadius: '8px',
              border: '1px solid var(--color-outline-variant)',
              fontFamily: 'var(--font-inter)',
              fontSize: '12px',
              color: 'var(--color-on-surface-variant)',
              fontStyle: 'italic',
            }}
          >
            "According to the 2024 Research Doc [pg. 12]..."
          </div>
        </motion.div>

        {/* Card 4: Asymmetric Connections (8 cols) */}
        <motion.div
          variants={fadeUp}
          custom={0.45}
          className="glass-card-hover"
          style={{
            gridColumn: 'span 8',
            borderRadius: '32px',
            padding: '40px',
            display: 'flex',
            alignItems: 'center',
            gap: '40px',
            minHeight: '220px',
          }}
        >
          <div style={{ flex: 1 }}>
            <h3 style={{ fontFamily: 'var(--font-geist)', fontSize: '28px', fontWeight: 600, marginBottom: '12px', letterSpacing: '-0.01em' }}>
              Asymmetric Connections
            </h3>
            <p style={{ fontFamily: 'var(--font-inter)', fontSize: '15px', color: 'var(--color-on-surface-variant)', lineHeight: 1.6, maxWidth: '400px' }}>
              Our AI identifies hidden links between disparate notes, creating a non-linear thinking environment that mirrors human cognition.
            </p>
          </div>
          <div style={{ position: 'relative', width: '140px', height: '140px', flexShrink: 0 }}>
            <div
              style={{
                position: 'absolute',
                inset: 0,
                background: 'linear-gradient(135deg, rgba(192,193,255,0.3), rgba(78,222,163,0.3))',
                borderRadius: '50%',
                filter: 'blur(20px)',
                animation: 'aura-pulse 4s ease-in-out infinite',
              }}
            />
            <div
              className="glass-card"
              style={{
                position: 'relative',
                width: '100%',
                height: '100%',
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid var(--color-outline)',
              }}
            >
              <span className="material-symbols-outlined" style={{ color: 'var(--color-primary)', fontSize: '56px' }}>hub</span>
            </div>
          </div>
        </motion.div>
      </motion.section>

      {/* ── CTA Section ──────────────────────────────────────────────── */}
      <motion.section
        initial={{ opacity: 0, y: 40 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, margin: '-60px' }}
        transition={{ duration: 0.8, ease: [0.22, 1, 0.36, 1] }}
        style={{
          padding: '60px var(--spacing-lg) 100px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          textAlign: 'center',
          position: 'relative',
          zIndex: 1,
        }}
      >
        <div
          className="glass-card"
          style={{
            maxWidth: '720px',
            width: '100%',
            borderRadius: '48px',
            padding: '64px 48px',
            border: '1px solid rgba(192,193,255,0.15)',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: '-60px',
              right: '-60px',
              width: '200px',
              height: '200px',
              background: 'rgba(192,193,255,0.08)',
              borderRadius: '50%',
              filter: 'blur(40px)',
            }}
          />
          <h2 style={{ fontFamily: 'var(--font-geist)', fontSize: '32px', fontWeight: 600, letterSpacing: '-0.01em', marginBottom: '16px' }}>
            Ready to scale your intelligence?
          </h2>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '16px', color: 'var(--color-on-surface-variant)', marginBottom: '40px', lineHeight: 1.7 }}>
            Join over 50,000 researchers and power users building their second brains on ArchadiLM.
          </p>
          <div style={{ display: 'flex', gap: '16px', justifyContent: 'center', flexWrap: 'wrap' }}>
            <Link
              href="/signup"
              style={{
                padding: '14px 32px',
                background: 'var(--color-primary)',
                color: 'var(--color-on-primary)',
                borderRadius: '12px',
                fontFamily: 'var(--font-geist)',
                fontSize: '16px',
                fontWeight: 600,
                boxShadow: '0 8px 30px rgba(192,193,255,0.25)',
                transition: 'all 0.2s',
              }}
            >
              Start Free Trial
            </Link>
            <button
              style={{
                padding: '14px 32px',
                background: 'transparent',
                border: 'none',
                color: 'var(--color-on-surface-variant)',
                fontFamily: 'var(--font-geist)',
                fontSize: '16px',
                fontWeight: 500,
                cursor: 'pointer',
                transition: 'color 0.2s',
              }}
            >
              Book a Demo
            </button>
          </div>
        </div>
      </motion.section>

      {/* ── Footer ───────────────────────────────────────────────────── */}
      <footer
        style={{
          background: 'var(--color-surface-container-lowest)',
          borderTop: '1px solid var(--color-outline-variant)',
          padding: 'var(--spacing-lg) var(--spacing-xl)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexWrap: 'wrap',
          gap: '16px',
        }}
      >
        <div>
          <div style={{ fontFamily: 'var(--font-geist)', fontSize: '16px', fontWeight: 700, marginBottom: '4px' }}>ArchadiLM</div>
          <p style={{ fontFamily: 'var(--font-inter)', fontSize: '12px', color: 'var(--color-on-surface-variant)' }}>
            © 2024 ArchadiLM Corp. All rights reserved.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 'var(--spacing-lg)' }}>
          {['Privacy Policy', 'Terms of Service', 'Docs', 'API'].map((link) => (
            <a
              key={link}
              href="#"
              style={{
                fontFamily: 'var(--font-geist)',
                fontSize: '12px',
                fontWeight: 500,
                color: 'var(--color-on-surface-variant)',
                letterSpacing: '0.05em',
                transition: 'color 0.2s',
              }}
            >
              {link}
            </a>
          ))}
        </div>
      </footer>
    </main>
  );
}

'use client';

import React from 'react';

// ── Button ────────────────────────────────────────────────────────────────

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  children,
  disabled,
  style,
  ...rest
}: ButtonProps) {
  const base: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '8px',
    fontFamily: 'var(--font-geist)',
    fontWeight: 600,
    letterSpacing: '0.02em',
    borderRadius: '10px',
    border: '1px solid transparent',
    cursor: disabled || loading ? 'not-allowed' : 'pointer',
    opacity: disabled || loading ? 0.6 : 1,
    transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
    whiteSpace: 'nowrap',
  };

  const sizes: Record<string, React.CSSProperties> = {
    sm: { padding: '6px 12px', fontSize: 'var(--text-sm)' },
    md: { padding: '10px 18px', fontSize: 'var(--text-base)' },
    lg: { padding: '14px 24px', fontSize: 'var(--text-lg)' },
  };

  const variants: Record<string, React.CSSProperties> = {
    primary: {
      background: 'var(--color-primary)',
      color: 'var(--color-on-primary)',
      borderColor: 'var(--color-primary)',
      boxShadow: '0 4px 20px rgba(192,193,255,0.2)',
    },
    secondary: {
      background: 'rgba(45,52,73,0.5)',
      color: 'var(--color-on-surface)',
      borderColor: 'var(--color-outline-variant)',
    },
    ghost: {
      background: 'transparent',
      color: 'var(--color-on-surface-variant)',
      borderColor: 'transparent',
    },
    danger: {
      background: 'transparent',
      color: 'var(--color-error)',
      borderColor: 'rgba(255,180,171,0.3)',
    },
  };

  return (
    <button
      disabled={disabled || loading}
      style={{ ...base, ...sizes[size], ...variants[variant], ...style }}
      {...rest}
    >
      {loading && <Spinner size={size === 'sm' ? 14 : 16} />}
      {children}
    </button>
  );
}

// ── Spinner ───────────────────────────────────────────────────────────────

export function Spinner({ size = 20, color = 'currentColor' }: { size?: number; color?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke={color}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ animation: 'spin 0.8s linear infinite' }}
      aria-label="Loading"
    >
      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
      <circle cx="12" cy="12" r="10" opacity="0.25" />
      <path d="M12 2a10 10 0 0 1 10 10" />
    </svg>
  );
}

// ── Badge ─────────────────────────────────────────────────────────────────

type BadgeVariant = 'default' | 'success' | 'warning' | 'error' | 'accent';

export function Badge({ children, variant = 'default' }: { children: React.ReactNode; variant?: BadgeVariant }) {
  const colors: Record<BadgeVariant, React.CSSProperties> = {
    default: { background: 'var(--color-paper-muted)', color: 'var(--color-ink-secondary)' },
    success: { background: 'var(--color-success-light)', color: 'var(--color-success)' },
    warning: { background: 'var(--color-warning-light)', color: 'var(--color-warning)' },
    error:   { background: 'var(--color-error-light)', color: 'var(--color-error)' },
    accent:  { background: 'var(--color-accent-light)', color: 'var(--color-accent)' },
  };
  return (
    <span style={{
      display: 'inline-flex',
      alignItems: 'center',
      padding: '2px 8px',
      borderRadius: 'var(--radius-full)',
      fontSize: 'var(--text-xs)',
      fontWeight: 600,
      fontFamily: 'var(--font-ui)',
      letterSpacing: '0.02em',
      textTransform: 'uppercase',
      ...colors[variant],
    }}>
      {children}
    </span>
  );
}

// ── CitationMarker — the signature UI element ─────────────────────────────
// Styled like a "sticky note" or "highlighted underline" reference.
// docs/AI_Course_Assistant_UI_Prompt.md#4-visual-direction

interface CitationMarkerProps {
  index: number;
  chunkId: string;
  title?: string;
  startTimestamp?: number;
  onJumpTo?: (ts?: number) => void;
}

export function CitationMarker({ index, title, startTimestamp, onJumpTo }: CitationMarkerProps) {
  return (
    <button
      onClick={() => onJumpTo?.(startTimestamp)}
      title={title ? `${title} (Click to view source)` : "Click to view source"}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "2px 6px",
        background: "rgba(140, 136, 255, 0.18)",
        border: "1px solid rgba(140, 136, 255, 0.4)",
        borderRadius: "4px",
        color: "var(--color-primary)",
        fontSize: "11px",
        fontFamily: "var(--font-mono)",
        fontWeight: 600,
        cursor: "pointer",
        verticalAlign: "middle",
        margin: "0 2px",
        transition: "all var(--transition-fast)",
      }}
    >
      [{index + 1}]
    </button>
  );
}


// ── ProcessingStepper ─────────────────────────────────────────────────────

const STAGES = [
  { key: 'UPLOADING',   label: 'Uploading' },
  { key: 'UPLOADED',    label: 'Upload complete' },
  { key: 'PARSING',     label: 'Extracting' },
  { key: 'NORMALIZING', label: 'Normalizing' },
  { key: 'CHUNKING',    label: 'Chunking' },
  { key: 'EMBEDDING',   label: 'Embedding' },
  { key: 'INDEXED',     label: 'Ready!' },
  { key: 'FAILED',      label: 'Failed' },
];

export function ProcessingStepper({ status }: { status: string }) {
  const currentIdx = STAGES.findIndex((s) => s.key === status);

  return (
    <ol style={{
      display: 'flex',
      flexDirection: 'column',
      gap: 'var(--space-2)',
      padding: 0,
      listStyle: 'none',
    }}>
      {STAGES.filter((s) => s.key !== 'FAILED').map((stage, i) => {
        const done = i < currentIdx;
        const active = i === currentIdx;
        const failed = status === 'FAILED' && i === currentIdx;

        return (
          <li key={stage.key} style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-3)',
            opacity: i > currentIdx && status !== 'FAILED' ? 0.4 : 1,
            transition: 'opacity var(--transition-normal)',
          }}>
            <span style={{
              width: '24px',
              height: '24px',
              borderRadius: 'var(--radius-full)',
              border: `2px solid ${done ? 'var(--color-success)' : active ? 'var(--color-accent)' : 'var(--color-border)'}`,
              background: done ? 'var(--color-success)' : active ? 'var(--color-accent-light)' : 'transparent',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              transition: 'all var(--transition-normal)',
            }}>
              {done && <span style={{ color: '#fff', fontSize: '12px' }}>✓</span>}
              {active && !failed && <Spinner size={12} color="var(--color-accent)" />}
              {failed && <span style={{ color: 'var(--color-error)', fontSize: '12px' }}>✕</span>}
            </span>
            <span style={{
              fontSize: 'var(--text-sm)',
              fontFamily: 'var(--font-ui)',
              fontWeight: active ? 600 : 400,
              color: done ? 'var(--color-ink-muted)' : active ? 'var(--color-ink)' : 'var(--color-ink-faint)',
            }}>
              {stage.label}
            </span>
          </li>
        );
      })}
    </ol>
  );
}

// ── Input ─────────────────────────────────────────────────────────────────

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export function Input({ label, error, id, style, ...rest }: InputProps) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
      {label && (
        <label htmlFor={id} style={{
          fontFamily: 'var(--font-ui)',
          fontSize: 'var(--text-sm)',
          fontWeight: 500,
          color: 'var(--color-ink-secondary)',
        }}>
          {label}
        </label>
      )}
      <input
        id={id}
        className="input-glow"
        style={{
          fontFamily: 'var(--font-inter)',
          fontSize: '13px',
          padding: '12px 14px',
          border: `1px solid ${error ? 'var(--color-error)' : 'var(--color-outline-variant)'}`,
          borderRadius: '10px',
          background: 'var(--color-surface-container-lowest)',
          color: 'var(--color-on-surface)',
          outline: 'none',
          transition: 'border-color 0.2s, box-shadow 0.2s',
          width: '100%',
          ...style,
        }}
        {...rest}
      />
      {error && (
        <span style={{ color: 'var(--color-error)', fontSize: 'var(--text-sm)' }}>
          {error}
        </span>
      )}
    </div>
  );
}

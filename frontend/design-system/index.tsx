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
    letterSpacing: '0.01em',
    borderRadius: 'var(--radius-md)',
    border: '1px solid transparent',
    cursor: disabled || loading ? 'not-allowed' : 'pointer',
    opacity: disabled || loading ? 0.55 : 1,
    transition: 'all var(--transition-fast)',
    whiteSpace: 'nowrap',
  };

  const sizes: Record<string, React.CSSProperties> = {
    sm: { padding: '6px 12px', fontSize: 'var(--text-sm)' },
    md: { padding: '10px 18px', fontSize: 'var(--text-base)' },
    lg: { padding: '13px 22px', fontSize: 'var(--text-lg)' },
  };

  const variants: Record<string, React.CSSProperties> = {
    primary: {
      background: 'var(--color-primary)',
      color: 'var(--color-on-primary)',
      borderColor: 'var(--color-primary)',
      boxShadow: '0 1px 0 rgba(0,0,0,0.15), 0 6px 18px rgba(217,164,65,0.16)',
    },
    secondary: {
      background: 'var(--color-surface-container-high)',
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
      borderColor: 'rgba(232,138,125,0.35)',
    },
  };

  return (
    <button
      disabled={disabled || loading}
      style={{ ...base, ...sizes[size], ...variants[variant], ...style }}
      onMouseEnter={(e) => {
        if (disabled || loading) return;
        if (variant === 'primary') e.currentTarget.style.background = 'var(--color-accent-hover)';
        if (variant === 'secondary') e.currentTarget.style.borderColor = 'var(--color-outline)';
        if (variant === 'ghost') e.currentTarget.style.color = 'var(--color-on-surface)';
      }}
      onMouseLeave={(e) => {
        if (disabled || loading) return;
        if (variant === 'primary') e.currentTarget.style.background = 'var(--color-primary)';
        if (variant === 'secondary') e.currentTarget.style.borderColor = 'var(--color-outline-variant)';
        if (variant === 'ghost') e.currentTarget.style.color = 'var(--color-on-surface-variant)';
      }}
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
      <circle cx="12" cy="12" r="10" opacity="0.25" />
      <path d="M12 2a10 10 0 0 1 10 10" />
    </svg>
  );
}

// ── Badge ─────────────────────────────────────────────────────────────────

type BadgeVariant = 'default' | 'success' | 'warning' | 'error' | 'accent';

export function Badge({ children, variant = 'default' }: { children: React.ReactNode; variant?: BadgeVariant }) {
  const colors: Record<BadgeVariant, React.CSSProperties> = {
    default: { background: 'var(--color-paper-muted)', color: 'var(--color-ink-secondary)', border: '1px solid var(--color-outline-variant)' },
    success: { background: 'var(--color-success-light)', color: 'var(--color-success)', border: '1px solid rgba(127,199,154,0.3)' },
    warning: { background: 'var(--color-warning-light)', color: 'var(--color-warning)', border: '1px solid rgba(226,166,59,0.3)' },
    error:   { background: 'var(--color-error-light)', color: 'var(--color-error)', border: '1px solid rgba(232,138,125,0.3)' },
    accent:  { background: 'var(--color-accent-light)', color: 'var(--color-accent)', border: '1px solid var(--color-accent-border)' },
  };
  return (
    <span style={{
      display: 'inline-flex',
      alignItems: 'center',
      padding: '2px 8px',
      borderRadius: 'var(--radius-sm)',
      fontSize: 'var(--text-xs)',
      fontWeight: 600,
      fontFamily: 'var(--font-ui)',
      letterSpacing: '0.03em',
      textTransform: 'uppercase',
      ...colors[variant],
    }}>
      {children}
    </span>
  );
}

// ── CitationMarker — the signature UI element ─────────────────────────────
// A small "index-card tab" in Marginalia gold with a torn top-right corner.
// Appears inline in assistant prose; clicking opens the source panel.

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
      title={title ? `${title} — view source` : "View source"}
      className="citation-tab"
      style={{
        minWidth: '20px',
        height: '18px',
        padding: '0 5px',
        fontSize: '10.5px',
        verticalAlign: 'text-top',
        margin: '0 1px',
      }}
    >
      {index + 1}
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
              width: '22px',
              height: '22px',
              borderRadius: 'var(--radius-full)',
              border: `2px solid ${done ? 'var(--color-tertiary)' : active ? 'var(--color-primary)' : 'var(--color-outline-variant)'}`,
              background: done ? 'var(--color-tertiary)' : active ? 'var(--color-accent-light)' : 'transparent',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              transition: 'all var(--transition-normal)',
            }}>
              {done && <span style={{ color: 'var(--color-on-tertiary)', fontSize: '11px', fontWeight: 700 }}>✓</span>}
              {active && !failed && <Spinner size={11} color="var(--color-primary)" />}
              {failed && <span style={{ color: 'var(--color-error)', fontSize: '11px' }}>✕</span>}
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
          borderRadius: 'var(--radius-md)',
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

// ── SourceTypeIcon — consistent iconography for source kinds ──────────────

export function SourceTypeIcon({ kind, size = 16 }: { kind?: string; size?: number }) {
  const map: Record<string, string> = {
    youtube: 'smart_display',
    video: 'smart_display',
    pdf: 'picture_as_pdf',
    web: 'language',
    url: 'language',
    text: 'description',
    txt: 'description',
    doc: 'article',
  };
  const icon = map[(kind || '').toLowerCase()] || 'description';
  return (
    <span className="material-symbols-outlined" style={{ fontSize: `${size}px` }}>
      {icon}
    </span>
  );
}

import type { ReactNode } from 'react';
import { useDarkMode } from '../../theme/useDarkMode';

interface PageShellProps {
  children: ReactNode;
  /** When provided, renders a back button that calls this handler. */
  onBack?: () => void;
  /** Accessible label for the back button. Defaults to "Back to home". */
  backLabel?: string;
}

/** Standard page layout wrapper with centered content, Reign heading, and dark mode toggle. */
export function PageShell({ children, onBack, backLabel = 'Back to home' }: PageShellProps) {
  const { isDark, toggle } = useDarkMode();

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '24px',
        padding: '24px 16px',
        minHeight: '100vh',
        backgroundColor: 'var(--color-background)',
        fontFamily: '"Nunito Sans", system-ui, sans-serif',
        color: 'var(--color-ink)',
      }}
    >
      {/* Header row: back button (left) | title (center) | dark mode toggle (right) */}
      <div
        data-testid="page-header"
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          width: '100%',
          maxWidth: 600,
        }}
      >
        {/* Left: back button or spacer */}
        {onBack ? (
          <button
            type="button"
            onClick={onBack}
            aria-label={backLabel}
            data-testid="back-button"
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: '1.25rem',
              padding: '8px',
              color: 'var(--color-ink)',
              lineHeight: 1,
              minWidth: 44,
              minHeight: 44,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {'\u2190'}
          </button>
        ) : (
          <div style={{ minWidth: 44 }} />
        )}

        {/* Center: title */}
        <h1
          style={{
            fontSize: '1.875rem',
            fontWeight: 800,
            letterSpacing: '-0.01em',
            margin: 0,
          }}
        >
          Reign
        </h1>

        {/* Right: dark mode toggle */}
        <button
          type="button"
          onClick={toggle}
          aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
          data-testid="dark-mode-toggle"
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            fontSize: '1.25rem',
            padding: '8px',
            color: 'var(--color-muted)',
            lineHeight: 1,
            minWidth: 44,
            minHeight: 44,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {isDark ? '\u2600' : '\u263E'}
        </button>
      </div>
      {children}
    </div>
  );
}

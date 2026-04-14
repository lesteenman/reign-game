import type { ReactNode } from 'react';
import { useDarkMode } from '../../theme/useDarkMode';

interface PageShellProps {
  children: ReactNode;
}

/** Standard page layout wrapper with centered content, Reign heading, and dark mode toggle. */
export function PageShell({ children }: PageShellProps) {
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
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
        }}
      >
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
        <button
          type="button"
          onClick={toggle}
          aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            fontSize: '1.25rem',
            padding: '4px',
            color: 'var(--color-muted)',
            lineHeight: 1,
          }}
        >
          {isDark ? '\u2600' : '\u263E'}
        </button>
      </div>
      {children}
    </div>
  );
}

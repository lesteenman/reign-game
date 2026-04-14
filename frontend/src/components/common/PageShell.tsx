import type { ReactNode } from 'react';

interface PageShellProps {
  children: ReactNode;
}

/** Standard page layout wrapper with centered content and Reign heading. */
export function PageShell({ children }: PageShellProps) {
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
      {children}
    </div>
  );
}

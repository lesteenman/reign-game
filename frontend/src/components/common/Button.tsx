import type { ReactNode, CSSProperties, MouseEventHandler } from 'react';

interface ButtonProps {
  children: ReactNode;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  disabled?: boolean;
  type?: 'button' | 'submit' | 'reset';
  'aria-label'?: string;
  'data-testid'?: string;
}

const baseStyle: CSSProperties = {
  padding: '12px 32px',
  border: '2px solid var(--color-ink)',
  borderRadius: 'var(--radius)',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: 700,
  fontSize: '1rem',
  cursor: 'pointer',
  transition: 'transform 100ms ease-out, box-shadow 100ms ease-out',
};

function handleEnter(e: React.MouseEvent<HTMLButtonElement>, shadowColor: string) {
  e.currentTarget.style.transform = 'translateY(1px)';
  e.currentTarget.style.boxShadow = `0 2px 0 ${shadowColor}`;
}

function handleLeave(e: React.MouseEvent<HTMLButtonElement>, shadowColor: string) {
  e.currentTarget.style.transform = 'translateY(0)';
  e.currentTarget.style.boxShadow = `0 3px 0 ${shadowColor}`;
}

/** Accent-colored primary action button. */
export function PrimaryButton({ children, onClick, disabled, type = 'button', ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      {...rest}
      style={{
        ...baseStyle,
        backgroundColor: 'var(--color-accent)',
        color: 'var(--color-on-accent)',
        boxShadow: '0 3px 0 var(--color-accent-shadow)',
      }}
      onMouseEnter={(e) => handleEnter(e, 'var(--color-accent-shadow)')}
      onMouseLeave={(e) => handleLeave(e, 'var(--color-accent-shadow)')}
    >
      {children}
    </button>
  );
}

/** Surface-colored secondary action button. */
export function SecondaryButton({ children, onClick, disabled, type = 'button', ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      {...rest}
      style={{
        ...baseStyle,
        backgroundColor: 'var(--color-surface)',
        color: 'var(--color-ink)',
        boxShadow: '0 3px 0 var(--color-ink)',
      }}
      onMouseEnter={(e) => handleEnter(e, 'var(--color-ink)')}
      onMouseLeave={(e) => handleLeave(e, 'var(--color-ink)')}
    >
      {children}
    </button>
  );
}

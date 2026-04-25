import type { ReactNode, CSSProperties, MouseEventHandler } from 'react';
import { pressIn, pressOut } from './press';

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
      onMouseEnter={(e) => pressIn(e, 'var(--color-accent-shadow)')}
      onMouseLeave={(e) => pressOut(e, 'var(--color-accent-shadow)')}
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
      onMouseEnter={(e) => pressIn(e, 'var(--color-ink)')}
      onMouseLeave={(e) => pressOut(e, 'var(--color-ink)')}
    >
      {children}
    </button>
  );
}

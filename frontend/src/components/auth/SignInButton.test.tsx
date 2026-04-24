import { render, screen, cleanup } from '../../test-utils';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { SignInButton } from './SignInButton';

// Clerk's SignInButton renders its children inside its own button when
// children are provided. We mock it to a plain button so we can assert
// the wrapped output without booting Clerk.
vi.mock('@clerk/clerk-react', () => ({
  SignInButton: ({ children, mode }: { children?: React.ReactNode; mode?: string }) => (
    <div data-testid="clerk-sign-in-button" data-mode={mode}>
      {children ?? <button type="button">Sign in</button>}
    </div>
  ),
}));

afterEach(() => {
  cleanup();
});

describe('SignInButton', () => {
  it('renders the default "Sign in" label', () => {
    // Arrange & Act
    render(<SignInButton />);

    // Assert
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('renders a custom label when provided', () => {
    // Arrange & Act
    render(<SignInButton label="Log in as admin" />);

    // Assert
    expect(
      screen.getByRole('button', { name: /log in as admin/i }),
    ).toBeInTheDocument();
  });

  it('requests modal presentation from Clerk (mode="modal")', () => {
    // Arrange & Act
    render(<SignInButton />);

    // Assert — mode is forwarded to Clerk's SignInButton
    expect(screen.getByTestId('clerk-sign-in-button')).toHaveAttribute(
      'data-mode',
      'modal',
    );
  });
});

import { render, screen, cleanup } from '../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { AdminLandingPage } from './AdminLandingPage';

// Mock Clerk's SignInButton/SignOutButton to plain buttons so we can
// assert on them without booting Clerk.
vi.mock('@clerk/clerk-react', () => ({
  SignInButton: ({ children, mode }: { children?: React.ReactNode; mode?: string }) => (
    <div data-testid="clerk-sign-in-button" data-mode={mode}>
      {children ?? <button type="button">Sign in</button>}
    </div>
  ),
  SignOutButton: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="clerk-sign-out-button">
      {children ?? <button type="button">Sign out</button>}
    </div>
  ),
}));

function renderInRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

afterEach(() => {
  cleanup();
});

describe('AdminLandingPage', () => {
  it('anonymous state shows "Admin Access" heading + sign-in prompt', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="anonymous" />);

    // Assert
    expect(
      screen.getByRole('heading', { name: /admin access/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/sign in to access the admin panel/i),
    ).toBeInTheDocument();
    expect(screen.getByTestId('clerk-sign-in-button')).toBeInTheDocument();
  });

  it('forbidden state shows "No Admin Access" heading + sign-out button', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="forbidden" />);

    // Assert
    expect(
      screen.getByRole('heading', { name: /no admin access/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/this account doesn't have admin access/i),
    ).toBeInTheDocument();
    expect(screen.getByTestId('clerk-sign-out-button')).toBeInTheDocument();
  });

  it('forbidden state does not render a sign-in button', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="forbidden" />);

    // Assert
    expect(screen.queryByTestId('clerk-sign-in-button')).not.toBeInTheDocument();
  });

  it('anonymous state does not render a sign-out button', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="anonymous" />);

    // Assert
    expect(screen.queryByTestId('clerk-sign-out-button')).not.toBeInTheDocument();
  });
});

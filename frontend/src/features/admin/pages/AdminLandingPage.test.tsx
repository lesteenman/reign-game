import { render, screen, cleanup } from '@shared/test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { AdminLandingPage } from './AdminLandingPage';

// Mock Clerk's SignInButton to a plain button so we can assert on it
// without booting Clerk.
vi.mock('@clerk/react', () => ({
  SignInButton: ({ children, mode }: { children?: React.ReactNode; mode?: string }) => (
    <div data-testid="clerk-sign-in-button" data-mode={mode}>
      {children ?? <button type="button">Sign in</button>}
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

  it('forbidden state shows "No Admin Access" heading + Home link to /', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="forbidden" />);

    // Assert
    expect(
      screen.getByRole('heading', { name: /no admin access/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/this account doesn't have admin access/i),
    ).toBeInTheDocument();
    const home = screen.getByRole('link', { name: /home/i });
    expect(home).toBeInTheDocument();
    expect(home).toHaveAttribute('href', '/');
  });

  it('forbidden state does not render a sign-in button', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="forbidden" />);

    // Assert
    expect(screen.queryByTestId('clerk-sign-in-button')).not.toBeInTheDocument();
  });

  it('anonymous state does not render a Home link', () => {
    // Arrange & Act
    renderInRouter(<AdminLandingPage state="anonymous" />);

    // Assert
    expect(screen.queryByRole('link', { name: /home/i })).not.toBeInTheDocument();
  });
});

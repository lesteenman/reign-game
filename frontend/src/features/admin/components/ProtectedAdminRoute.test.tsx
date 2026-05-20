import { render, screen, cleanup } from '@shared/test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';

// Mock Clerk before importing the module under test.
type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};

const useUserMock = vi.fn<() => UseUserReturn>();

vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
  // Used by the AdminLandingPage in the forbidden branch; we don't exercise
  // its behaviour here, just need it to render without requiring a provider.
  SignOutButton: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="clerk-sign-out-button">
      {children ?? <button type="button">Sign out</button>}
    </div>
  ),
  SignInButton: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="clerk-sign-in-button">
      {children ?? <button type="button">Sign in</button>}
    </div>
  ),
}));

// Stub AdminPage — it pulls in admin services / network code that we
// don't need for routing assertions.
vi.mock('@features/admin/pages/AdminPage', () => ({
  AdminPage: () => <div data-testid="admin-page">ADMIN UI</div>,
}));

import { ProtectedAdminRoute } from './ProtectedAdminRoute';

beforeEach(() => {
  useUserMock.mockReset();
});

afterEach(() => {
  cleanup();
});

function renderProtected() {
  return render(
    <MemoryRouter>
      <ProtectedAdminRoute />
    </MemoryRouter>,
  );
}

describe('ProtectedAdminRoute', () => {
  it('shows a loading indicator while Clerk is still initialising', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });

    // Act
    renderProtected();

    // Assert — loading state visible, admin UI not rendered.
    expect(screen.getByTestId('admin-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('admin-page')).not.toBeInTheDocument();
  });

  it('renders the anonymous landing when the visitor is not signed in', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });

    // Act
    renderProtected();

    // Assert
    expect(screen.getByTestId('admin-landing-anonymous')).toBeInTheDocument();
    expect(screen.queryByTestId('admin-page')).not.toBeInTheDocument();
  });

  it('renders the forbidden landing when a signed-in user lacks admin role', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'user' } },
    });

    // Act
    renderProtected();

    // Assert
    expect(screen.getByTestId('admin-landing-forbidden')).toBeInTheDocument();
    expect(screen.queryByTestId('admin-page')).not.toBeInTheDocument();
  });

  it('renders the AdminPage when signed in with role=admin', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });

    // Act
    renderProtected();

    // Assert
    expect(screen.getByTestId('admin-page')).toBeInTheDocument();
    expect(screen.queryByTestId('admin-landing-anonymous')).not.toBeInTheDocument();
    expect(screen.queryByTestId('admin-landing-forbidden')).not.toBeInTheDocument();
  });

  it('treats a missing publicMetadata.role as forbidden (non-admin)', () => {
    // Arrange — publicMetadata is present but role is absent.
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: {} },
    });

    // Act
    renderProtected();

    // Assert
    expect(screen.getByTestId('admin-landing-forbidden')).toBeInTheDocument();
    expect(screen.queryByTestId('admin-page')).not.toBeInTheDocument();
  });
});

import { render, screen, cleanup } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';

// Clerk's `<UserButton>` in real life opens a popover menu; we mock it
// so MenuItems/Link children render inline and we can assert on them.
// Children must render whenever the mock is signed-in so tests can
// verify presence/absence of the Admin link.
type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};

const useUserMock = vi.fn<() => UseUserReturn>();

vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
  UserButton: Object.assign(
    ({ children }: { children?: React.ReactNode }) => (
      <div data-testid="clerk-user-button">{children}</div>
    ),
    {
      MenuItems: ({ children }: { children?: React.ReactNode }) => (
        <div data-testid="clerk-menu-items">{children}</div>
      ),
      Link: ({
        href,
        label,
      }: {
        href: string;
        label: string;
        labelIcon: React.ReactNode;
      }) => (
        <a href={href} data-testid="clerk-menu-link">
          {label}
        </a>
      ),
      Action: ({ label }: { label: string }) => (
        <button type="button">{label}</button>
      ),
    },
  ),
}));

// Import after vi.mock so the mocked module is bound.
import { UserMenu } from './UserMenu';

beforeEach(() => {
  useUserMock.mockReset();
});

afterEach(() => {
  cleanup();
});

describe('UserMenu', () => {
  it('renders nothing while Clerk is still loading (avoids role-resolution flicker)', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });

    // Act
    const { container } = render(<UserMenu />);

    // Assert — no flicker: no button, no link, nothing on first paint.
    expect(container.firstChild).toBeNull();
  });

  it('renders the Clerk UserButton when a user is signed in without admin role', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'user' } },
    });

    // Act
    render(<UserMenu />);

    // Assert
    expect(screen.getByTestId('clerk-user-button')).toBeInTheDocument();
    // AS-10: admin link hidden for non-admins.
    expect(screen.queryByRole('link', { name: /admin/i })).not.toBeInTheDocument();
  });

  it('renders the Admin link only when the user has role=admin', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });

    // Act
    render(<UserMenu />);

    // Assert
    const adminLink = screen.getByRole('link', { name: /admin/i });
    expect(adminLink).toBeInTheDocument();
    expect(adminLink).toHaveAttribute('href', '/admin');
  });

  it('treats a missing role as non-admin (no Admin link)', () => {
    // Arrange — publicMetadata.role is absent (new Clerk account).
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: {} },
    });

    // Act
    render(<UserMenu />);

    // Assert
    expect(screen.getByTestId('clerk-user-button')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /admin/i })).not.toBeInTheDocument();
  });
});

import { render, screen, fireEvent, cleanup } from '../../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { PageShell } from './PageShell';
import { ClerkAvailabilityProvider } from '../auth/ClerkAvailability';

// Tests run outside a real ClerkProvider; the components that rely on
// Clerk hooks / auth gates (Show / UserMenu / SignInButton) are gated
// by `useClerkAvailable()`. Default the flag to `false` so the
// existing behavioural tests still run without booting Clerk.
// Post-v6 (core-3): `<SignedIn>`/`<SignedOut>` were replaced by
// `<Show when="signed-in|signed-out">`.
vi.mock('@clerk/react', () => ({
  Show: () => null,
  SignInButton: () => null,
  UserButton: () => null,
  useUser: () => ({ isLoaded: false, isSignedIn: false, user: null }),
}));

afterEach(() => {
  cleanup();
});

function renderShell(ui: React.ReactElement, clerkAvailable = false) {
  return render(
    <MemoryRouter>
      <ClerkAvailabilityProvider available={clerkAvailable}>
        {ui}
      </ClerkAvailabilityProvider>
    </MemoryRouter>,
  );
}

describe('PageShell', () => {
  it('renders children', () => {
    // Arrange & Act
    renderShell(<PageShell><p>Hello</p></PageShell>);

    // Assert
    expect(screen.getByText('Hello')).toBeInTheDocument();
  });

  it('renders Reign heading', () => {
    // Arrange & Act
    renderShell(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.getByRole('heading', { name: /reign/i })).toBeInTheDocument();
  });

  it('renders dark mode toggle', () => {
    // Arrange & Act
    renderShell(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('dark-mode-toggle')).toBeInTheDocument();
  });

  it('does not render back button when onBack is not provided', () => {
    // Arrange & Act
    renderShell(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.queryByTestId('back-button')).not.toBeInTheDocument();
  });

  it('renders back button when onBack is provided', () => {
    // Arrange
    const onBack = vi.fn();

    // Act
    renderShell(<PageShell onBack={onBack}><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('back-button')).toBeInTheDocument();
  });

  it('calls onBack when back button is clicked', () => {
    // Arrange
    const onBack = vi.fn();
    renderShell(<PageShell onBack={onBack}><p>content</p></PageShell>);

    // Act
    fireEvent.click(screen.getByTestId('back-button'));

    // Assert
    expect(onBack).toHaveBeenCalledOnce();
  });

  it('uses custom backLabel for aria-label', () => {
    // Arrange & Act
    renderShell(<PageShell onBack={vi.fn()} backLabel="Go back"><p>content</p></PageShell>);

    // Assert
    expect(screen.getByTestId('back-button')).toHaveAttribute('aria-label', 'Go back');
  });

  it('toggles dark mode on click', () => {
    // Arrange
    renderShell(<PageShell><p>content</p></PageShell>);
    const toggle = screen.getByTestId('dark-mode-toggle');
    const initialLabel = toggle.getAttribute('aria-label');

    // Act
    fireEvent.click(toggle);

    // Assert
    expect(toggle.getAttribute('aria-label')).not.toEqual(initialLabel);
  });
});

import { render, screen, fireEvent, cleanup } from '../../test-utils';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
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

  it('renders subtitle when provided', () => {
    // Arrange & Act
    renderShell(
      <PageShell subtitle="Daily Puzzle · May 11"><p>content</p></PageShell>,
    );

    // Assert
    expect(screen.getByTestId('page-subtitle')).toBeInTheDocument();
    expect(screen.getByTestId('page-subtitle')).toHaveTextContent('Daily Puzzle · May 11');
  });

  it('does not render subtitle when not provided', () => {
    // Arrange & Act
    renderShell(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.queryByTestId('page-subtitle')).not.toBeInTheDocument();
  });

  it('lays content out top-down (header is the first child of the shell flex column)', () => {
    // Arrange
    renderShell(<PageShell><p data-testid="body">content</p></PageShell>);

    // Act
    const header = screen.getByTestId('page-header');
    const shell = header.parentElement!;

    // Assert — the shell uses a column flex layout that does NOT
    // vertically center its children. On a tall viewport, vertically-
    // centering pushes the header off the top and floats content
    // outside the visible area on mobile.
    const computed = window.getComputedStyle(shell);
    expect(computed.flexDirection).toBe('column');
    expect(computed.justifyContent).not.toBe('center');
    // Header is the first DOM child so it pins to the top.
    expect(shell.firstElementChild).toBe(header);
  });
});

describe('PageShell — offline banner integration', () => {
  let originalDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalDescriptor = Object.getOwnPropertyDescriptor(window.navigator, 'onLine');
  });

  afterEach(() => {
    if (originalDescriptor) {
      Object.defineProperty(window.navigator, 'onLine', originalDescriptor);
    } else {
      // onLine was on the prototype; remove the own-property override we set.
      delete (window.navigator as unknown as { onLine?: boolean }).onLine;
    }
  });

  it('does not render the offline banner when online', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });

    // Act
    renderShell(<PageShell><p>content</p></PageShell>);

    // Assert
    expect(screen.queryByTestId('offline-banner')).not.toBeInTheDocument();
  });

  it('renders the offline banner when offline, positioned before children', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });

    // Act
    renderShell(<PageShell><p data-testid="body">content</p></PageShell>);

    // Assert
    const banner = screen.getByTestId('offline-banner');
    const body = screen.getByTestId('body');
    expect(banner).toBeInTheDocument();
    // Banner must appear earlier in the document than the body so
    // screen readers announce it first.
    expect(banner.compareDocumentPosition(body) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});

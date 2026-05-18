import { render, screen, cleanup } from '../test-utils';
import { fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

let originalFetch: typeof globalThis.fetch;

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

type UseUserReturn = {
  isLoaded: boolean;
  isSignedIn: boolean;
  user: { publicMetadata: { role?: string } } | null;
};
const useUserMock = vi.fn<() => UseUserReturn>();
vi.mock('@clerk/react', () => ({
  useUser: () => useUserMock(),
}));

import { LandingPage } from './LandingPage';

beforeEach(() => {
  mockNavigate.mockClear();
  useUserMock.mockReset();
});

afterEach(() => {
  cleanup();
});

function renderLandingPage() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  );
}

describe('LandingPage — tile visibility across Clerk states', () => {
  it('signed-out: shows Daily (enabled) + Packs (disabled), no Curation tile', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).toBeInTheDocument();
    expect(screen.getByTestId('tile-packs')).toBeInTheDocument();
    expect(screen.queryByTestId('tile-curation')).not.toBeInTheDocument();
    expect(screen.getByTestId('tile-daily')).not.toBeDisabled();
    expect(screen.getByTestId('tile-packs')).toBeDisabled();
  });

  it('signed-in user-role: same as signed-out — no Curation tile', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'user' } },
    });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).toBeInTheDocument();
    expect(screen.getByTestId('tile-daily')).not.toBeDisabled();
    expect(screen.getByTestId('tile-packs')).toBeInTheDocument();
    expect(screen.queryByTestId('tile-curation')).not.toBeInTheDocument();
  });

  it('signed-in admin: shows all three tiles; Daily + Curation enabled, Packs disabled', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).not.toBeDisabled();
    expect(screen.getByTestId('tile-packs')).toBeDisabled();
    const curation = screen.getByTestId('tile-curation');
    expect(curation).toBeInTheDocument();
    expect(curation).not.toBeDisabled();
  });

  it('Clerk still loading: hides the Curation tile to avoid role-resolution flicker', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: false, isSignedIn: false, user: null });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).toBeInTheDocument();
    expect(screen.queryByTestId('tile-curation')).not.toBeInTheDocument();
  });

  it('tile-daily is no longer in the "coming soon" disabled state', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });

    // Act
    renderLandingPage();

    // Assert
    const daily = screen.getByTestId('tile-daily');
    expect(daily).not.toBeDisabled();
    expect(daily).toHaveAttribute('aria-disabled', 'false');
    expect(daily).not.toHaveTextContent(/coming soon/i);
  });
});

describe('LandingPage — offline integration', () => {
  let originalDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalDescriptor = Object.getOwnPropertyDescriptor(window.navigator, 'onLine');
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    if (originalDescriptor) {
      Object.defineProperty(window.navigator, 'onLine', originalDescriptor);
    } else {
      // onLine was on the prototype; remove the own-property override we set.
      delete (window.navigator as unknown as { onLine?: boolean }).onLine;
    }
  });

  it('disables Daily + Curation tiles when offline', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).toBeDisabled();
    expect(screen.getByTestId('tile-curation')).toBeDisabled();
    // Packs is always disabled regardless of online state.
    expect(screen.getByTestId('tile-packs')).toBeDisabled();
  });

  it('Daily + Curation tiles are enabled when online', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).not.toBeDisabled();
    expect(screen.getByTestId('tile-curation')).not.toBeDisabled();
  });

  it('shows a connectivity tooltip on offline-disabled Daily + Curation tiles', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });

    // Act
    renderLandingPage();

    // Assert
    expect(screen.getByTestId('tile-daily')).toHaveAttribute(
      'title',
      'Connect to the internet to start a new puzzle',
    );
    expect(screen.getByTestId('tile-curation')).toHaveAttribute(
      'title',
      'Connect to the internet to start a new puzzle',
    );
    // Packs does NOT get the connectivity tooltip — it's "Coming soon", not a connectivity issue.
    expect(screen.getByTestId('tile-packs')).not.toHaveAttribute('title');
  });

});

describe('LandingPage — tile click behaviour', () => {
  it('clicking the Daily tile navigates to /play?flow=daily', () => {
    // Arrange
    useUserMock.mockReturnValue({ isLoaded: true, isSignedIn: false, user: null });
    renderLandingPage();

    // Act
    fireEvent.click(screen.getByTestId('tile-daily'));

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/play?flow=daily');
  });

  it('Daily tile navigates the same way for signed-in user-role identities', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'user' } },
    });
    renderLandingPage();

    // Act
    fireEvent.click(screen.getByTestId('tile-daily'));

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/play?flow=daily');
  });

  it('clicking the Packs tile (disabled) does NOT navigate', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });
    renderLandingPage();

    // Act — fireEvent.click respects the `disabled` attribute on
    // buttons (no click event fires); we still call to assert no
    // navigation happens.
    fireEvent.click(screen.getByTestId('tile-packs'));

    // Assert
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('clicking the Curation tile navigates to /curation (admin only)', () => {
    // Arrange
    useUserMock.mockReturnValue({
      isLoaded: true,
      isSignedIn: true,
      user: { publicMetadata: { role: 'admin' } },
    });
    renderLandingPage();

    // Act
    fireEvent.click(screen.getByTestId('tile-curation'));

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/curation');
  });
});

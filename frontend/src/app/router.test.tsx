import { render, screen, cleanup } from '@shared/test-utils';
import { describe, it, expect, vi, afterEach } from 'vitest';

// Stub every feature surface the router can mount so this test exercises
// the dispatch wiring (which element renders for which URL), not the
// feature internals. Each stub echoes the `?pack=` slug it receives so
// the pack branch's slug-passing is observable.
vi.mock('@features/landing/pages/LandingPage', () => ({
  LandingPage: () => <div data-testid="landing" />,
}));
vi.mock('@features/daily/screens/DailyFlow', () => ({
  DailyFlow: () => <div data-testid="daily" />,
}));
vi.mock('@features/curation/pages/PlayPuzzlePage', () => ({
  PlayPuzzlePage: () => <div data-testid="curation-play" />,
}));
vi.mock('@features/curation/pages/CurationPage', () => ({
  CurationPage: () => <div data-testid="curation" />,
}));
vi.mock('@features/admin/components/ProtectedAdminRoute', () => ({
  ProtectedAdminRoute: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="admin">{children}</div>
  ),
}));
vi.mock('@features/admin/pages/AdminPacksPage', () => ({
  AdminPacksPage: () => <div data-testid="admin-packs" />,
}));
vi.mock('@features/packs/pages/PacksBrowsePage', () => ({
  PacksBrowsePage: () => <div data-testid="packs-browse" />,
}));
vi.mock('@features/packs/screens/PackFlow', () => ({
  PackFlow: ({ slug }: { slug: string | null }) => (
    <div data-testid="pack-flow" data-slug={slug ?? '<null>'} />
  ),
}));

import { Router } from './router';

afterEach(() => {
  cleanup();
});

function renderAt(path: string) {
  window.history.pushState({}, '', path);
  return render(<Router />);
}

describe('Router — /play flow dispatch', () => {
  it('dispatches flow=pack to PackFlow with the ?pack slug', async () => {
    // Arrange + Act
    renderAt('/play?flow=pack&pack=starter');

    // Assert
    const flow = await screen.findByTestId('pack-flow');
    expect(flow).toHaveAttribute('data-slug', 'starter');
  });

  it('passes a null slug to PackFlow when ?pack is missing', async () => {
    // Arrange + Act
    renderAt('/play?flow=pack');

    // Assert
    const flow = await screen.findByTestId('pack-flow');
    expect(flow).toHaveAttribute('data-slug', '<null>');
  });

  it('dispatches flow=daily to the daily flow', async () => {
    // Arrange + Act
    renderAt('/play?flow=daily');

    // Assert
    expect(await screen.findByTestId('daily')).toBeInTheDocument();
  });

  it('redirects an unknown flow home', async () => {
    // Arrange + Act
    renderAt('/play?flow=bogus');

    // Assert
    expect(await screen.findByTestId('landing')).toBeInTheDocument();
  });

  it('mounts the packs-browse surface at /packs', async () => {
    // Arrange + Act
    renderAt('/packs');

    // Assert
    expect(await screen.findByTestId('packs-browse')).toBeInTheDocument();
  });
});

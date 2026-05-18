import type { CSSProperties } from 'react';
import { useInstallPrompt } from '../../shared/hooks/useInstallPrompt';

// Style: inline CSSProperties using theme tokens (no Tamagui).
// Tamagui migration tracked in #176.
// Style intentionally close to LandingPage's tileBaseStyle but uses
// the accent palette to distinguish "install" from puzzle tiles.

const tileStyle: CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '2px solid var(--color-accent)',
  borderRadius: 'var(--radius)',
  boxShadow: '0 3px 0 var(--color-accent-shadow)',
  padding: '24px',
  display: 'flex',
  flexDirection: 'column',
  gap: '4px',
  textAlign: 'left',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  textDecoration: 'none',
  color: 'var(--color-ink)',
  cursor: 'pointer',
  width: '100%',
  transition: 'transform 100ms ease-out',
};

const titleStyle: CSSProperties = {
  fontSize: '1.25rem',
  fontWeight: 800,
  margin: 0,
};

const subtitleStyle: CSSProperties = {
  fontSize: '0.875rem',
  color: 'var(--color-muted)',
  margin: 0,
  fontStyle: 'italic',
};

/**
 * Install-as-app CTA tile for the LandingPage. Visible only when the
 * browser surfaced a deferred install prompt and the app isn't
 * already running standalone. On iOS Safari (no
 * beforeinstallprompt support) this is always null — users install
 * via Share → Add to Home Screen.
 */
export function InstallAppTile() {
  const { canInstall, isStandalone, promptInstall } = useInstallPrompt();
  if (!canInstall || isStandalone) return null;

  return (
    <button
      type="button"
      data-testid="tile-install"
      onClick={() => { void promptInstall(); }}
      style={tileStyle}
    >
      <h2 style={titleStyle}>Install Reign</h2>
      <p style={subtitleStyle}>Add to home screen — play offline</p>
    </button>
  );
}

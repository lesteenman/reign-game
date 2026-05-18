import type { CSSProperties } from 'react';
import { Download } from 'lucide-react';
import { Icon } from '../../shared/components/Icon';
import { useInstallPrompt } from '../../shared/hooks/useInstallPrompt';

// Style: small text button in the PageShell header cluster. Visually
// parallel to the dark-mode toggle (transparent background,
// color-muted text, 44px hit target). Tamagui migration tracked in
// #176.

const buttonStyle: CSSProperties = {
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  fontSize: '0.875rem',
  fontWeight: 600,
  padding: '8px 12px',
  color: 'var(--color-muted)',
  lineHeight: 1,
  minHeight: 44,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

/**
 * Compact install CTA in the PageShell header. Visible only when the
 * browser surfaced a deferred install prompt and the app isn't
 * already running standalone. On iOS Safari and other browsers that
 * don't fire beforeinstallprompt, this is always null.
 */
export function InstallButton() {
  const { canInstall, isStandalone, promptInstall } = useInstallPrompt();
  if (!canInstall || isStandalone) return null;
  return (
    <button
      type="button"
      data-testid="install-button"
      onClick={() => { void promptInstall(); }}
      aria-label="Install Reign as an app"
      title="Install Reign"
      style={buttonStyle}
    >
      <Icon as={Download} size={16} />
      <span style={{ marginLeft: 6 }}>Install</span>
    </button>
  );
}

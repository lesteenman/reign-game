import { Download } from 'lucide-react';
import { styled, Text, View } from 'tamagui';
import { Icon } from './Icon';
import { useInstallPrompt } from '@shared/hooks/useInstallPrompt';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const InstallButtonBase = styledAny(View, {
  name: 'InstallButton',
  flexDirection: 'row',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 6,
  paddingHorizontal: 12,
  paddingVertical: 8,
  minHeight: 44,
  borderWidth: 0,
  backgroundColor: 'transparent',
  cursor: 'pointer',
  color: '$muted',
  animation: 'quickerLessBouncy',
  hoverStyle: { color: '$ink' },
});

const InstallLabel = styledAny(Text, {
  name: 'InstallButtonLabel',
  fontSize: '$3',
  fontWeight: '600',
});

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
    <InstallButtonBase
      render={<button type="button" />}
      onClick={() => { void promptInstall(); }}
      aria-label="Install Reign as an app"
      title="Install Reign"
      data-testid="install-button"
    >
      <Icon as={Download} size={16} />
      <InstallLabel>Install</InstallLabel>
    </InstallButtonBase>
  );
}

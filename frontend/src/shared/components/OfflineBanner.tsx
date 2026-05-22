import { styled, Text, View } from 'tamagui';
import { useConnectivity } from '@shared/hooks/useConnectivity';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const Banner = styledAny(View, {
  name: 'OfflineBanner',
  width: '100%',
  maxWidth: 600,
  paddingHorizontal: 16,
  paddingVertical: 8,
  borderRadius: 10,
  backgroundColor: '$destructiveBg',
});

const BannerText = styledAny(Text, {
  name: 'OfflineBannerText',
  color: '$destructive',
  fontSize: '$3',
  fontWeight: '600',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

/**
 * Global offline indicator. Renders nothing while online; renders a
 * non-interruptive status banner when navigator.onLine === false.
 * Mounted by PageShell so every route gets the same signal.
 */
export function OfflineBanner() {
  const online = useConnectivity();
  if (online) return null;
  return (
    <Banner data-testid="offline-banner" role="status">
      <BannerText>
        You&apos;re offline — connect to start a new puzzle or sync results.
      </BannerText>
    </Banner>
  );
}

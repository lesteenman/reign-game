import { useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { styled, Text } from 'tamagui';
import { PageShell } from '@shared/components/PageShell';
import { PrimaryButton, SecondaryButton } from '@shared/components/Button';
import { Card } from '@shared/components/Card';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const Heading = styledAny(Text, {
  name: 'PackCompleteHeading',
  fontSize: '$7',
  fontWeight: '700',
  color: '$success',
  textAlign: 'center',
  render: <h2 />,
  margin: 0,
});

const Body = styledAny(Text, {
  name: 'PackCompleteBody',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: '600',
  fontSize: '$4',
  color: '$body',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

export interface PackCompleteScreenProps {
  packName: string;
  puzzleCount: number;
}

/**
 * Terminal pack state — shown when every puzzle in the pack is solved.
 * Mirrors the celebratory completion idiom (BRAND_GUIDELINES §5.5/§5.6)
 * with affordances back to the browse list and home.
 */
export function PackCompleteScreen({
  packName,
  puzzleCount,
}: PackCompleteScreenProps) {
  const navigate = useNavigate();

  const handleBrowse = useCallback(() => {
    navigate('/packs');
  }, [navigate]);

  const handleHome = useCallback(() => {
    navigate('/');
  }, [navigate]);

  return (
    <PageShell onBack={handleBrowse} subtitle={packName}>
      <Card size="compact" data-testid="pack-complete">
        <Heading>Pack complete!</Heading>
        <Body>
          You solved all {puzzleCount} puzzles in {packName}.
        </Body>
        <PrimaryButton onClick={handleBrowse} data-testid="pack-complete-browse">
          More packs
        </PrimaryButton>
        <SecondaryButton onClick={handleHome} data-testid="pack-complete-home">
          Home
        </SecondaryButton>
      </Card>
    </PageShell>
  );
}

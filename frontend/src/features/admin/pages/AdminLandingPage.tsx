import { styled, Text } from 'tamagui';
import { PageShell } from '@shared/components/PageShell';
import { SignInButton } from '@shared/auth/SignInButton';
import { Card } from '@shared/components/Card';
import { CompactSecondaryLink } from '@shared/components/Button';

type LandingState = 'anonymous' | 'forbidden';

interface AdminLandingPageProps {
  state: LandingState;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

// Render as `<h2>` so screen readers + Testing Library's heading-role
// queries find the card title. Tamagui Text defaults to `<span>`;
// the `render: <h2 />` override matches the pattern used by
// PageShell's title (#176 PageShell slice). Margin: 0 zeros out the
// browser UA stylesheet's default block margin on `<h2>`.
const CardHeading = styledAny(Text, {
  name: 'AdminCardHeading',
  fontSize: '$7',
  fontWeight: '800',
  letterSpacing: -0.16,
  color: '$ink',
  textAlign: 'center',
  render: <h2 />,
  margin: 0,
});

const CardBody = styledAny(Text, {
  name: 'AdminCardBody',
  fontSize: '$4',
  color: '$body',
  lineHeight: '$1',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

/**
 * Landing page rendered at `/admin` when the visitor is not authorised
 * to see the admin UI. Two mutually-exclusive states per AS-09:
 *
 *   - 'anonymous' — no session; invite to sign in.
 *   - 'forbidden' — signed in but `publicMetadata.role !== 'admin'`;
 *     offer a way back to the game. Sign-out is reachable from the
 *     header avatar; duplicating it here would be redundant.
 */
export function AdminLandingPage({ state }: AdminLandingPageProps) {
  if (state === 'anonymous') {
    return (
      <PageShell>
        <Card size="large" data-testid="admin-landing-anonymous">
          <CardHeading>Admin Access</CardHeading>
          <CardBody>Sign in to access the admin panel.</CardBody>
          <SignInButton />
        </Card>
      </PageShell>
    );
  }

  return (
    <PageShell>
      <Card size="large" data-testid="admin-landing-forbidden">
        <CardHeading>No Admin Access</CardHeading>
        <CardBody>This account doesn&apos;t have admin access.</CardBody>
        <CompactSecondaryLink to="/">Home</CompactSecondaryLink>
      </Card>
    </PageShell>
  );
}

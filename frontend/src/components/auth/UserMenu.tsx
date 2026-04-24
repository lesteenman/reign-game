import { UserButton, useUser } from "@clerk/react";

/**
 * Narrow the shape we consume from Clerk's publicMetadata. Clerk types
 * it as `Record<string, unknown>`; we only care about `role`, and only
 * accept the string form documented in auth-surface.md AS-03.
 */
function resolveRole(
  publicMetadata: Record<string, unknown> | undefined,
): string {
  if (!publicMetadata) return '';
  const role = publicMetadata.role;
  return typeof role === 'string' ? role : '';
}

/**
 * UserMenu wraps Clerk's `<UserButton>` with a custom "Admin" menu item
 * that only appears when the signed-in user has
 * `publicMetadata.role === 'admin'` (spec: auth-surface.md AS-10).
 *
 * First-paint correctness: while Clerk is still loading we render
 * nothing instead of a placeholder. This avoids the role-resolution
 * flicker called out in AC-4 / CLAUDE.md lesson 5.
 */
export function UserMenu() {
  const { isLoaded, isSignedIn, user } = useUser();

  // Not loaded yet — parent already decided to render us, but the
  // useUser data hasn't arrived. Returning null keeps the header free
  // of a placeholder button that would flash into the menu.
  if (!isLoaded || !isSignedIn || !user) {
    return null;
  }

  const role = resolveRole(user.publicMetadata);
  const isAdmin = role === 'admin';

  return (
    <UserButton>
      {isAdmin && (
        <UserButton.MenuItems>
          <UserButton.Link
            label="Admin"
            labelIcon={<AdminIcon />}
            href="/admin"
          />
        </UserButton.MenuItems>
      )}
    </UserButton>
  );
}

/**
 * Simple shield icon for the Admin menu entry. Kept inline so we don't
 * pull in an icon library for a single glyph. 16x16 with 2px stroke to
 * follow the brand guideline on even-pixel values.
 */
function AdminIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M8 2l5 2v4c0 3-2.2 5.4-5 6-2.8-.6-5-3-5-6V4l5-2z" />
    </svg>
  );
}

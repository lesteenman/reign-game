/**
 * Extract the role claim from Clerk's `publicMetadata`.
 *
 * Clerk types `publicMetadata` as `Record<string, unknown>`; we accept
 * only the string form documented in auth-surface.md AS-03 and treat
 * anything else (missing, non-string, malformed) as "no role". Returning
 * the raw string (not a boolean) keeps the canonical `=== 'admin'`
 * comparison in the consumer — see AS-10 on hidden-not-disabled gating.
 */
export function getClerkUserRole(
  publicMetadata: Record<string, unknown> | undefined,
): string {
  if (!publicMetadata) return '';
  const role = publicMetadata.role;
  return typeof role === 'string' ? role : '';
}

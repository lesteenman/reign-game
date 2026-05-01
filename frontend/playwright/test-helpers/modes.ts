/**
 * Shared types + predicate for GET /api/config/modes. Public,
 * unauthenticated endpoint; both admin-config-flow.spec.ts and
 * dynamic-modes.spec.ts read it.
 */

export interface Mode {
  size: number;
  mode: string;
}

export interface ModesResponse {
  modes: Mode[];
}

export const containsCombo = (
  modes: Mode[],
  size: number,
  mode: string,
): boolean => modes.some((m) => m.size === size && m.mode === mode);

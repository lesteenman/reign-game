/**
 * Date helpers for UTC-bound flows (e.g. the Daily Puzzle). Centralised
 * so the service and the feature screens don't each carry their own
 * one-line copy of the same string manipulation.
 */

/** Returns today's date as YYYY-MM-DD in UTC. Optional `now` for tests. */
export function todayUtcDate(now: Date = new Date()): string {
  return now.toISOString().slice(0, 10);
}

/**
 * Extracts the YYYY-MM-DD UTC date component from an RFC3339 timestamp.
 * Used to derive a per-flow storage key from a server-provided
 * assignedAt without re-deriving the date on the client side.
 */
export function dateFromAssignedAt(assignedAt: string): string {
  return new Date(assignedAt).toISOString().slice(0, 10);
}

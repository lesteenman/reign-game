/**
 * DailyPuzzlePayload mirrors the GET /api/daily/{date} response body.
 * Defined here (not in services/) so screens and tests import types
 * from the source rather than through the service module.
 */
export interface DailyPuzzlePayload {
  puzzleId: string;
  grid: number;
  regions: number[][];
  assignedAt: string;
  outcome: 'started' | 'solved';
  serverElapsedMs?: number;
  submittedAt?: string;
}

/**
 * DailySubmitResponse mirrors the POST /api/daily/{date}/result response body.
 */
export interface DailySubmitResponse {
  serverElapsedMs: number;
  leaderboardRank?: number;
}

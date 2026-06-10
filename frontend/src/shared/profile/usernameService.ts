// Profile username service.
//
// Wraps the backend PUT /api/profile/username endpoint (signed-in only;
// the Clerk session cookie authenticates the request). The backend
// validates format + profanity, then performs the Clerk write and
// enforces uniqueness, mapping the outcomes to status codes:
//   200 → success
//   400 → invalid format
//   409 → username taken
//   422 → not allowed (profanity / reserved)
//
// Callers consume this through the useSetUsername hook, not directly —
// hooks own I/O per the feature-folder rules.

import { apiPut } from '@shared/api';

interface SetUsernameResponse {
  username: string;
}

/**
 * Set the signed-in player's leaderboard username. Throws ApiError on
 * non-2xx; callers map err.status to user-facing copy.
 */
export async function setUsername(username: string): Promise<string> {
  const res = await apiPut<SetUsernameResponse>('/api/profile/username', {
    username,
  });
  return res.username;
}

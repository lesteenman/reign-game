import { useMutation } from '@tanstack/react-query';
import { useUser } from '@clerk/react';
import { setUsername } from '@shared/profile/usernameService';
import { ApiError } from '@shared/api';

/**
 * `useMutation` wrapping the set-username call. On success it reloads the
 * Clerk user so `useUser().user.username` reflects the new name without
 * waiting for the next session refresh — the prompt then disappears and
 * the name shows immediately.
 *
 * Errors bubble as ApiError; callers map `err.status` to copy
 * (400 → invalid format, 409 → taken, 422 → not allowed).
 */
export function useSetUsername() {
  const { user } = useUser();

  return useMutation<string, ApiError | Error, string>({
    mutationFn: async (username: string) => {
      const saved = await setUsername(username);
      // Refresh the cached Clerk user so the new username propagates to
      // every useUser() consumer (the prompt's own visibility guard
      // included). reload() is best-effort — the server write already
      // succeeded, so a reload failure shouldn't surface as an error.
      try {
        await user?.reload();
      } catch {
        // Ignore: the name is set server-side; the next natural refresh
        // will pick it up if this reload didn't.
      }
      return saved;
    },
  });
}

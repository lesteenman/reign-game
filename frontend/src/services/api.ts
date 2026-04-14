/**
 * Lightweight API client for the Reign backend.
 *
 * Uses VITE_API_URL if set; otherwise requests go to the same origin
 * (handled by Vite's dev proxy in development, or same-domain API Gateway
 * in production).
 */
const API_BASE_URL = import.meta.env.VITE_API_URL || '';

/** Fetch JSON from the backend API. Throws on non-2xx responses. */
export async function apiFetch<T>(
  path: string,
  params?: Record<string, string>,
): Promise<T> {
  const url = new URL(path, API_BASE_URL || window.location.origin);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      url.searchParams.set(key, value);
    }
  }
  const response = await fetch(url.toString());
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(
      (body as Record<string, string>).message ||
        `API error: ${response.status}`,
    );
  }
  return response.json() as Promise<T>;
}

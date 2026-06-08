/**
 * Lightweight API client for the Reign backend.
 *
 * Uses `VITE_API_URL` if set; otherwise requests go to the same origin
 * (handled by Vite's dev proxy in development, or same-domain API
 * Gateway in production).
 *
 * Single internal `apiRequest(method, path, opts)` does the work;
 * `apiGet` / `apiPut` / `apiPost` are thin method-specific wrappers
 * that exist for caller-side readability — `apiGet('/api/x')` is
 * shorter and more grep-able than `apiRequest('GET', '/api/x')`.
 *
 * Consolidated from the pre-#120 trio of near-duplicate `apiFetch` /
 * `apiPut` / `apiPost` (three copies of URL building + error handling).
 * Empty-body / 204 responses now resolve uniformly to `{}` instead of
 * throwing on `apiGet` only.
 */

const API_BASE_URL = import.meta.env.VITE_API_URL || '';

// Browser-distributed API key bound to the AWS API Gateway usage plan.
// Baked into the bundle at build time by CD. Anyone can scrape it from
// the bundle — that's intentional. Its role is to identify callers to
// the usage plan so the per-key throttle applies. Empty in local dev
// (Vite proxies straight to the Go backend, no API Gateway) and in
// tests; the header is only attached when non-empty so dev + test
// don't ship an empty `x-api-key: ` line that API Gateway would 403.
const API_KEY = import.meta.env.VITE_API_KEY || '';

/**
 * Returns the `x-api-key` header object, or an empty object when no key
 * is configured (local dev / tests). Callers spread it into a fetch
 * `headers` so the header is omitted entirely when the key is empty —
 * the same truthy guard `apiRequest` applies, exported so non-client
 * fetches (e.g. the connectivity probe) bind to the usage plan without
 * duplicating the env read.
 */
export function apiKeyHeader(): Record<string, string> {
  return API_KEY ? { 'x-api-key': API_KEY } : {};
}

/** Error thrown on non-2xx API responses. Includes the HTTP status code. */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export interface ApiOptions {
  params?: Record<string, string>;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

type Method = 'GET' | 'PUT' | 'POST';

async function apiRequest<T>(
  method: Method,
  path: string,
  body: unknown | undefined,
  options?: ApiOptions,
): Promise<T> {
  const url = new URL(path, API_BASE_URL || window.location.origin);
  if (options?.params) {
    for (const [key, value] of Object.entries(options.params)) {
      url.searchParams.set(key, value);
    }
  }

  // Only declare Content-Type when we have a JSON body to send.
  // Adding Content-Type to a GET request would still be valid per the
  // RFC but it's noise — the backend ignores it on bodyless requests
  // and CORS preflight may upgrade simple-GETs to preflighted on some
  // browsers when an explicit Content-Type appears.
  const hasBody = body !== undefined;
  const headers: Record<string, string> = { ...options?.headers };
  if (hasBody && headers['Content-Type'] === undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (headers['x-api-key'] === undefined) {
    Object.assign(headers, apiKeyHeader());
  }

  const response = await fetch(url.toString(), {
    method,
    headers,
    signal: options?.signal,
    ...(hasBody ? { body: JSON.stringify(body) } : {}),
  });

  if (!response.ok) {
    // Try to extract a server-provided `{ message }` envelope. If the
    // body isn't JSON (or is empty), fall back to a status-only message
    // so the thrown error still carries useful debug info.
    const errBody = await response.json().catch(() => ({}));
    throw new ApiError(
      (errBody as Record<string, string>).message ||
        `API error: ${response.status}`,
      response.status,
    );
  }

  // Unified empty-body handling: read body as text, then JSON.parse only
  // if non-empty. Pre-#120, `apiFetch` called `response.json()` directly
  // and threw on empty responses (204 / empty 200); `apiPut`/`apiPost`
  // already did the text-then-parse dance. Applying the safe version
  // to every method is the whole point of the consolidation.
  const text = await response.text();
  return (text ? JSON.parse(text) : {}) as T;
}

/** Fetch JSON from the backend API (GET). Throws ApiError on non-2xx. */
export function apiGet<T>(path: string, options?: ApiOptions): Promise<T> {
  return apiRequest<T>('GET', path, undefined, options);
}

/** Send a PUT request with a JSON body. Throws ApiError on non-2xx. */
export function apiPut<T>(
  path: string,
  body: unknown,
  options?: ApiOptions,
): Promise<T> {
  return apiRequest<T>('PUT', path, body, options);
}

/** Send a POST request with a JSON body. Throws ApiError on non-2xx. */
export function apiPost<T>(
  path: string,
  body: unknown,
  options?: ApiOptions,
): Promise<T> {
  return apiRequest<T>('POST', path, body, options);
}

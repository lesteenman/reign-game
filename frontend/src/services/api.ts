/**
 * Lightweight API client for the Reign backend.
 *
 * Uses VITE_API_URL if set; otherwise requests go to the same origin
 * (handled by Vite's dev proxy in development, or same-domain API Gateway
 * in production).
 */
const API_BASE_URL = import.meta.env.VITE_API_URL || '';

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

/** Fetch JSON from the backend API. Throws ApiError on non-2xx responses. */
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
    throw new ApiError(
      (body as Record<string, string>).message ||
        `API error: ${response.status}`,
      response.status,
    );
  }
  return response.json() as Promise<T>;
}

/** Send a PUT request with a JSON body to the backend API. Throws ApiError on non-2xx. */
export async function apiPut<T>(
  path: string,
  body: unknown,
  params?: Record<string, string>,
): Promise<T> {
  const url = new URL(path, API_BASE_URL || window.location.origin);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      url.searchParams.set(key, value);
    }
  }
  const response = await fetch(url.toString(), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const respBody = await response.json().catch(() => ({}));
    throw new ApiError(
      (respBody as Record<string, string>).message ||
        `API error: ${response.status}`,
      response.status,
    );
  }
  // Return empty object for void-like responses
  const text = await response.text();
  return (text ? JSON.parse(text) : {}) as T;
}

/** Send a POST request with a JSON body to the backend API. Throws ApiError on non-2xx. */
export async function apiPost<T>(
  path: string,
  body: unknown,
  params?: Record<string, string>,
): Promise<T> {
  const url = new URL(path, API_BASE_URL || window.location.origin);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      url.searchParams.set(key, value);
    }
  }
  const response = await fetch(url.toString(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const respBody = await response.json().catch(() => ({}));
    throw new ApiError(
      (respBody as Record<string, string>).message ||
        `API error: ${response.status}`,
      response.status,
    );
  }
  // Return empty object for void-like responses
  const text = await response.text();
  return (text ? JSON.parse(text) : {}) as T;
}

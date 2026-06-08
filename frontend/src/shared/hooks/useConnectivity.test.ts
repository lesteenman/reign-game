import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, waitFor } from '@testing-library/react';
import { renderHook } from '@shared/test-utils';

// The hook reads the api key via `apiKeyHeader()` (shared/api/client),
// which reads `import.meta.env.VITE_API_KEY` at module load. Cases that
// assert the key path use `vi.stubEnv` + `vi.resetModules` (global
// beforeEach) and dynamically import the hook so the stubbed value is
// picked up — mirroring client.test.ts.

describe('useConnectivity', () => {
  let originalDescriptor: PropertyDescriptor | undefined;
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    vi.resetModules();
    originalDescriptor = Object.getOwnPropertyDescriptor(window.navigator, 'onLine');
    originalFetch = globalThis.fetch;
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });
  });

  afterEach(() => {
    if (originalDescriptor) {
      Object.defineProperty(window.navigator, 'onLine', originalDescriptor);
    } else {
      delete (window.navigator as unknown as { onLine?: boolean }).onLine;
    }
    globalThis.fetch = originalFetch;
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it('returns true when navigator says online and the probe succeeds', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled());

    // Assert
    expect(result.current).toBe(true);
  });

  it('omits x-api-key on the probe when VITE_API_KEY is empty (dev / test path)', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    renderHook(() => useConnectivity());
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled());

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const [url, init] = calls[0]!;
    expect(url).toBe('/api/health');
    expect(init).toMatchObject({ method: 'HEAD', cache: 'no-store' });
    expect((init.headers as Record<string, string>)['x-api-key']).toBeUndefined();
  });

  it('attaches x-api-key on the probe when VITE_API_KEY is set', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', 'sample-api-key-from-aws');
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    renderHook(() => useConnectivity());
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled());

    // Assert
    const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
    const init = calls[0]![1];
    expect((init.headers as Record<string, string>)['x-api-key']).toBe(
      'sample-api-key-from-aws',
    );
  });

  it('returns false when the probe fetch rejects (offline)', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(result.current).toBe(false));

    // Assert — already asserted in waitFor
  });

  it('returns false when the probe responds with !ok', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 503 } as Response);

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(result.current).toBe(false));
  });

  it('returns false when navigator says offline regardless of probe result', async () => {
    // Arrange
    vi.stubEnv('VITE_API_KEY', '');
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
    const { useConnectivity } = await import('./useConnectivity');
    const { result } = renderHook(() => useConnectivity());
    // Wait for the probe to settle so we test the combined state, not a transient
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled());

    // Assert
    expect(result.current).toBe(false);
  });

  it('re-probes when the browser dispatches an online event', async () => {
    // Arrange: probe always fails until the `online` event fires. StrictMode
    // double-invokes effects, so the first two calls (setup + cleanup re-mount)
    // both fail; the third call (re-probe on `online`) succeeds.
    vi.stubEnv('VITE_API_KEY', '');
    let callCount = 0;
    const fetchMock = vi.fn().mockImplementation(() => {
      callCount++;
      if (callCount <= 2) {
        return Promise.reject(new Error('Network error'));
      }
      return Promise.resolve({ ok: true } as Response);
    });
    globalThis.fetch = fetchMock;

    const { useConnectivity } = await import('./useConnectivity');
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(result.current).toBe(false));

    // Act: simulate navigator dispatching `online`
    act(() => {
      window.dispatchEvent(new Event('online'));
    });

    // Assert: re-probe succeeded, hook reports online
    await waitFor(() => expect(result.current).toBe(true));
  });
});

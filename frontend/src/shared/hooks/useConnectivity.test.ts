import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, waitFor } from '@testing-library/react';
import { renderHook } from '../../test-utils';
import { useConnectivity } from './useConnectivity';

describe('useConnectivity', () => {
  let originalDescriptor: PropertyDescriptor | undefined;
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
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
    vi.restoreAllMocks();
  });

  it('returns true when navigator says online and the probe succeeds', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled());

    // Assert
    expect(result.current).toBe(true);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/health', {
      method: 'HEAD',
      cache: 'no-store',
    });
  });

  it('returns false when the probe fetch rejects (offline)', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(result.current).toBe(false));

    // Assert — already asserted in waitFor
  });

  it('returns false when the probe responds with !ok', async () => {
    // Arrange
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 503 } as Response);

    // Act
    const { result } = renderHook(() => useConnectivity());
    await waitFor(() => expect(result.current).toBe(false));
  });

  it('returns false when navigator says offline regardless of probe result', async () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true } as Response);

    // Act
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
    let callCount = 0;
    const fetchMock = vi.fn().mockImplementation(() => {
      callCount++;
      if (callCount <= 2) {
        return Promise.reject(new Error('Network error'));
      }
      return Promise.resolve({ ok: true } as Response);
    });
    globalThis.fetch = fetchMock;

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

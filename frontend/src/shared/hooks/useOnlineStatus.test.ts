import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { act } from '@testing-library/react';
import { renderHook } from '../../test-utils';
import { useOnlineStatus } from './useOnlineStatus';

describe('useOnlineStatus', () => {
  let originalDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalDescriptor = Object.getOwnPropertyDescriptor(window.navigator, 'onLine');
  });

  afterEach(() => {
    if (originalDescriptor) {
      Object.defineProperty(window.navigator, 'onLine', originalDescriptor);
    } else {
      // onLine was on the prototype; remove the own-property override we set.
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      delete (window.navigator as any).onLine;
    }
  });

  it('returns navigator.onLine snapshot on mount (true)', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });

    // Act
    const { result } = renderHook(() => useOnlineStatus());

    // Assert
    expect(result.current).toBe(true);
  });

  it('returns navigator.onLine snapshot on mount (false)', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });

    // Act
    const { result } = renderHook(() => useOnlineStatus());

    // Assert
    expect(result.current).toBe(false);
  });

  it('updates to false when window dispatches offline event', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });
    const { result } = renderHook(() => useOnlineStatus());
    expect(result.current).toBe(true);

    // Act
    act(() => {
      Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });
      window.dispatchEvent(new Event('offline'));
    });

    // Assert
    expect(result.current).toBe(false);
  });

  it('updates to true when window dispatches online event', () => {
    // Arrange
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: false });
    const { result } = renderHook(() => useOnlineStatus());
    expect(result.current).toBe(false);

    // Act
    act(() => {
      Object.defineProperty(window.navigator, 'onLine', { configurable: true, value: true });
      window.dispatchEvent(new Event('online'));
    });

    // Assert
    expect(result.current).toBe(true);
  });
});

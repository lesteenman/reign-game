import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTimer } from './useTimer';

describe('useTimer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('initial state: elapsed is 0, not running', () => {
    const { result } = renderHook(() => useTimer());
    expect(result.current.elapsed).toBe(0);
    expect(result.current.isRunning).toBe(false);
  });

  it('start then advance 3 seconds -> elapsed is 3', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.start();
    });

    expect(result.current.isRunning).toBe(true);

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current.elapsed).toBe(3);
  });

  it('start, pause at 2s, start again, advance 3s -> elapsed is 5', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.start();
    });

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(result.current.elapsed).toBe(2);

    act(() => {
      result.current.pause();
    });

    expect(result.current.isRunning).toBe(false);
    expect(result.current.elapsed).toBe(2);

    act(() => {
      result.current.start();
    });

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current.elapsed).toBe(5);
  });

  it('stop prevents further accumulation', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.start();
    });

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    act(() => {
      result.current.stop();
    });

    expect(result.current.elapsed).toBe(2);
    expect(result.current.isRunning).toBe(false);

    // Trying to start after stop should not work
    act(() => {
      result.current.start();
    });

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current.elapsed).toBe(2);
    expect(result.current.isRunning).toBe(false);
  });

  it('reset zeros elapsed', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.start();
    });

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    act(() => {
      result.current.reset();
    });

    expect(result.current.elapsed).toBe(0);
    expect(result.current.isRunning).toBe(false);
  });

  it('restore with lastResumedAt resumes automatically', () => {
    vi.setSystemTime(new Date(1700000005000));

    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.restore({
        elapsedAtLastPause: 10,
        lastResumedAt: 1700000003000, // 2 seconds ago
      });
    });

    // Should be running and account for 2 seconds since lastResumedAt
    expect(result.current.isRunning).toBe(true);
    expect(result.current.elapsed).toBe(12); // 10 accumulated + 2 since resume

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current.elapsed).toBe(15); // 12 + 3 more
  });

  it('restore with null lastResumedAt does not resume', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.restore({
        elapsedAtLastPause: 10,
        lastResumedAt: null,
      });
    });

    expect(result.current.isRunning).toBe(false);
    expect(result.current.elapsed).toBe(10);
  });

  it('timerState reflects internal state for persistence', () => {
    const { result } = renderHook(() => useTimer());

    act(() => {
      result.current.start();
    });

    expect(result.current.timerState.lastResumedAt).not.toBeNull();
    expect(result.current.timerState.elapsedAtLastPause).toBe(0);

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    act(() => {
      result.current.pause();
    });

    expect(result.current.timerState.lastResumedAt).toBeNull();
    expect(result.current.timerState.elapsedAtLastPause).toBe(2);
  });
});

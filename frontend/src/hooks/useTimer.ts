import { useState, useCallback, useRef, useEffect } from 'react';

/** Persisted timer state for IndexedDB storage. */
export interface TimerState {
  elapsedAtLastPause: number;
  lastResumedAt: number | null;
}

/** Return value of the useTimer hook. */
export interface UseTimerReturn {
  elapsed: number;
  isRunning: boolean;
  timerState: TimerState;
  start: () => void;
  pause: () => void;
  stop: () => void;
  reset: () => void;
  restore: (state: TimerState) => void;
}

/**
 * Timer hook with pause/resume support and persistence-friendly state.
 *
 * Uses setInterval(1s) to tick the displayed elapsed time when running.
 * The actual elapsed time is computed from Date.now() for accuracy.
 */
export function useTimer(): UseTimerReturn {
  const [elapsedAtLastPause, setElapsedAtLastPause] = useState(0);
  const [lastResumedAt, setLastResumedAt] = useState<number | null>(null);
  const [tick, setTick] = useState(0); // forces re-render every second
  const [stopped, setStopped] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const isRunning = lastResumedAt !== null;

  const clearTick = useCallback(() => {
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const startTick = useCallback(() => {
    clearTick();
    intervalRef.current = setInterval(() => {
      setTick((t) => t + 1);
    }, 1000);
  }, [clearTick]);

  // Compute current elapsed from state + Date.now()
  const elapsed = isRunning
    ? elapsedAtLastPause + Math.floor((Date.now() - lastResumedAt) / 1000)
    : elapsedAtLastPause;

  // Suppress unused variable warning - tick is used to trigger re-renders
  void tick;

  const start = useCallback(() => {
    if (stopped) return;
    if (lastResumedAt !== null) return; // already running
    setLastResumedAt(Date.now());
    startTick();
  }, [stopped, lastResumedAt, startTick]);

  const pause = useCallback(() => {
    if (lastResumedAt === null) return;
    const now = Date.now();
    const additional = Math.floor((now - lastResumedAt) / 1000);
    setElapsedAtLastPause((prev) => prev + additional);
    setLastResumedAt(null);
    clearTick();
  }, [lastResumedAt, clearTick]);

  const stop = useCallback(() => {
    if (lastResumedAt !== null) {
      const now = Date.now();
      const additional = Math.floor((now - lastResumedAt) / 1000);
      setElapsedAtLastPause((prev) => prev + additional);
      setLastResumedAt(null);
      clearTick();
    }
    setStopped(true);
  }, [lastResumedAt, clearTick]);

  const reset = useCallback(() => {
    setElapsedAtLastPause(0);
    setLastResumedAt(null);
    setStopped(false);
    clearTick();
  }, [clearTick]);

  const restore = useCallback(
    (state: TimerState) => {
      setStopped(false);
      setElapsedAtLastPause(state.elapsedAtLastPause);
      setLastResumedAt(state.lastResumedAt);
      if (state.lastResumedAt !== null) {
        startTick();
      } else {
        clearTick();
      }
    },
    [startTick, clearTick],
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearTick();
    };
  }, [clearTick]);

  const timerState: TimerState = {
    elapsedAtLastPause,
    lastResumedAt,
  };

  return {
    elapsed,
    isRunning,
    timerState,
    start,
    pause,
    stop,
    reset,
    restore,
  };
}

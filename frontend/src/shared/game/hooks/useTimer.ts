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
    // Trigger an immediate re-render so the timer display updates instantly,
    // then tick every second after that.
    setTick((t) => t + 1);
    intervalRef.current = setInterval(() => {
      setTick((t) => t + 1);
    }, 1000);
  }, [clearTick]);

  // Compute current elapsed from state + Date.now(). Reading the wall
  // clock on demand is the deliberate accuracy mechanism: the `tick`
  // interval forces a re-render every second and the value is recomputed
  // from the live clock, so the display resyncs immediately after a
  // throttled/backgrounded tab and `elapsed` is correct the instant it is
  // read (callers depend on `restore()` yielding the right value
  // synchronously, before any interval fires). Driving `elapsed` off an
  // interval-set state instead would lag the clock and break that
  // synchronous-read contract, so the impure read stays in render here.
  const elapsed =
    lastResumedAt !== null
      ? // eslint-disable-next-line react-hooks/purity
        elapsedAtLastPause + Math.floor((Date.now() - lastResumedAt) / 1000)
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

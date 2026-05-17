import { renderHook } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useDailyService } from './useDailyService';

describe('useDailyService', () => {
  it('returns a stable object with getDaily and submitDailyResult functions', () => {
    // Arrange + Act
    const { result, rerender } = renderHook(() => useDailyService());

    // Assert
    expect(typeof result.current.getDaily).toBe('function');
    expect(typeof result.current.submitDailyResult).toBe('function');

    // Stability: same function references across re-renders (module-level
    // identity — the hook returns the imported functions directly).
    const firstRef = result.current;
    rerender();
    expect(result.current.getDaily).toBe(firstRef.getDaily);
    expect(result.current.submitDailyResult).toBe(firstRef.submitDailyResult);
  });
});

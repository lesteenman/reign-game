import { describe, it, expect } from 'vitest';
import { cellKey } from './cellKey';

describe('cellKey', () => {
  it('returns "0,0" for (0, 0)', () => {
    expect(cellKey(0, 0)).toBe('0,0');
  });

  it('returns "7,9" for (7, 9)', () => {
    expect(cellKey(7, 9)).toBe('7,9');
  });

  it('handles negative arguments without throwing', () => {
    expect(cellKey(-1, -2)).toBe('-1,-2');
  });
});

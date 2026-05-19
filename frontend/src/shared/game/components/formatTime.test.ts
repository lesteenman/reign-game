import { describe, it, expect } from 'vitest';
import { formatTime } from './GameBoard';

describe('formatTime', () => {
  it('formats zero seconds as 00:00', () => {
    expect(formatTime(0)).toBe('00:00');
  });

  it('formats sub-minute as 00:SS', () => {
    expect(formatTime(59)).toBe('00:59');
  });

  it('formats exactly one minute as 01:00', () => {
    expect(formatTime(60)).toBe('01:00');
  });

  it('formats a few minutes as MM:SS', () => {
    expect(formatTime(303)).toBe('05:03');
  });

  it('formats just under an hour as 59:59', () => {
    expect(formatTime(3599)).toBe('59:59');
  });

  it('formats exactly one hour as 1:00:00', () => {
    expect(formatTime(3600)).toBe('1:00:00');
  });

  it('formats one hour two minutes three seconds as 1:02:03', () => {
    expect(formatTime(3723)).toBe('1:02:03');
  });

  it('formats 5h 50m 05s as 5:50:05 (regression: was 350:05)', () => {
    expect(formatTime(21005)).toBe('5:50:05');
  });

  it('formats ten hours as 10:00:00', () => {
    expect(formatTime(36000)).toBe('10:00:00');
  });
});

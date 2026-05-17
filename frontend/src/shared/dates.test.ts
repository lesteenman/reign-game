import { describe, it, expect } from 'vitest';
import { todayUtcDate, dateFromAssignedAt } from './dates';

describe('todayUtcDate', () => {
  it('returns YYYY-MM-DD for a UTC date mid-day', () => {
    // Arrange
    const now = new Date('2026-05-17T15:30:00Z');

    // Act
    const result = todayUtcDate(now);

    // Assert
    expect(result).toBe('2026-05-17');
  });

  it('returns YYYY-MM-DD for a UTC date at end of day', () => {
    // Arrange
    const now = new Date('2026-05-17T23:59:59Z');

    // Act
    const result = todayUtcDate(now);

    // Assert
    expect(result).toBe('2026-05-17');
  });
});

describe('dateFromAssignedAt', () => {
  it('extracts YYYY-MM-DD from a UTC RFC3339 timestamp at midnight', () => {
    // Arrange
    const assignedAt = '2026-05-17T00:00:00Z';

    // Act
    const result = dateFromAssignedAt(assignedAt);

    // Assert
    expect(result).toBe('2026-05-17');
  });

  it('extracts YYYY-MM-DD from a UTC RFC3339 timestamp at end of day', () => {
    // Arrange
    const assignedAt = '2026-05-17T23:59:59Z';

    // Act
    const result = dateFromAssignedAt(assignedAt);

    // Assert
    expect(result).toBe('2026-05-17');
  });

  it('converts offset timestamp to UTC before extracting date', () => {
    // Arrange
    // 2026-05-17T23:59:59+01:00 == 2026-05-17T22:59:59Z — still May 17 UTC.
    const assignedAt = '2026-05-17T23:59:59+01:00';

    // Act
    const result = dateFromAssignedAt(assignedAt);

    // Assert
    expect(result).toBe('2026-05-17');
  });
});

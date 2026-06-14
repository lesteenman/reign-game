import { describe, test, expect } from 'vitest';
import { addId, removeId, moveUp, moveDown } from './reorder';

describe('addId', () => {
  test('appends a new id', () => {
    // Arrange
    const ids = ['a', 'b'];
    // Act
    const next = addId(ids, 'c');
    // Assert
    expect(next).toEqual(['a', 'b', 'c']);
    expect(ids).toEqual(['a', 'b']); // input not mutated
  });

  test('is a no-op when the id is already present', () => {
    // Arrange
    const ids = ['a', 'b'];
    // Act
    const next = addId(ids, 'b');
    // Assert
    expect(next).toEqual(['a', 'b']);
  });
});

describe('removeId', () => {
  test('removes the id', () => {
    // Arrange
    const ids = ['a', 'b', 'c'];
    // Act
    const next = removeId(ids, 'b');
    // Assert
    expect(next).toEqual(['a', 'c']);
  });
});

describe('moveUp', () => {
  test('swaps an item one slot earlier', () => {
    // Arrange
    const ids = ['a', 'b', 'c'];
    // Act
    const next = moveUp(ids, 2);
    // Assert
    expect(next).toEqual(['a', 'c', 'b']);
  });

  test('is a no-op at the head', () => {
    // Arrange
    const ids = ['a', 'b'];
    // Act
    const next = moveUp(ids, 0);
    // Assert
    expect(next).toEqual(['a', 'b']);
  });
});

describe('moveDown', () => {
  test('swaps an item one slot later', () => {
    // Arrange
    const ids = ['a', 'b', 'c'];
    // Act
    const next = moveDown(ids, 0);
    // Assert
    expect(next).toEqual(['b', 'a', 'c']);
  });

  test('is a no-op at the tail', () => {
    // Arrange
    const ids = ['a', 'b'];
    // Act
    const next = moveDown(ids, 1);
    // Assert
    expect(next).toEqual(['a', 'b']);
  });
});

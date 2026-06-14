/**
 * Pure helpers for the ordered puzzle list in the pack-assembly UI.
 * Each returns a new array; the input is never mutated.
 */

/** Append an id to the list if not already present. */
export function addId(ids: string[], id: string): string[] {
  return ids.includes(id) ? ids : [...ids, id];
}

/** Remove an id from the list. */
export function removeId(ids: string[], id: string): string[] {
  return ids.filter((existing) => existing !== id);
}

/** Move the item at `index` one slot earlier. No-op at the head. */
export function moveUp(ids: string[], index: number): string[] {
  if (index <= 0 || index >= ids.length) return ids;
  const next = [...ids];
  [next[index - 1], next[index]] = [next[index]!, next[index - 1]!];
  return next;
}

/** Move the item at `index` one slot later. No-op at the tail. */
export function moveDown(ids: string[], index: number): string[] {
  if (index < 0 || index >= ids.length - 1) return ids;
  const next = [...ids];
  [next[index], next[index + 1]] = [next[index + 1]!, next[index]!];
  return next;
}

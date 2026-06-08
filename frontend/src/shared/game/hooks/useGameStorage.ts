import type { GameStorage } from '@reign/core/storage';
import { indexedDbGameStorage } from '@storage/db';

/** Hook providing IndexedDB persistence for game state and completions. */
export function useGameStorage(): GameStorage {
  return indexedDbGameStorage;
}

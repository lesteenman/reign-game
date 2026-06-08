import type { CompletionRecord, FlowType, GameState } from './types';

/**
 * Platform-agnostic persistence contract for game state and completions.
 *
 * The web app implements this over IndexedDB (`frontend/src/storage/db.ts`
 * + `useGameStorage`); the future React Native client will implement it over
 * its own native store. Both consume the same Flow Slot identity (`idFor`)
 * and persisted shapes (`GameState`, `CompletionRecord`).
 */
export interface GameStorage {
  /** Upsert the game state for its composite Flow Slot key. */
  saveState(state: GameState): Promise<void>;
  /** Load the game state for a Flow Slot, or null if none is stored. */
  loadState(flowType: FlowType, flowId: string): Promise<GameState | null>;
  /** Remove the stored game state for a Flow Slot. */
  clearState(flowType: FlowType, flowId: string): Promise<void>;
  /** Append a completed-puzzle record to history. */
  addCompletion(record: CompletionRecord): Promise<void>;
}

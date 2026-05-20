import { getDaily, submitDailyResult } from '@services/dailyService';

/**
 * Passthrough hook that provides the daily service functions.
 *
 * Screens must not import services directly (leaf-I/O rule). This hook
 * is the dependency boundary; the I/O lives in the service. DailyFlow's
 * state machine drives the calls manually — a full useQuery rewrite
 * would change the IDB short-circuit / ApiError / cancellation behavior.
 */
export function useDailyService() {
  return { getDaily, submitDailyResult };
}

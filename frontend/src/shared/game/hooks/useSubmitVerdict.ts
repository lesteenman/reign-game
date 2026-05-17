import { useMutation } from '@tanstack/react-query';
import { submitVerdict, type SubmitVerdictArgs } from '../../../services/verdictService';

/** Wraps the verdict service in a TanStack useMutation so leaf components
 *  don't import services directly (leaf-I/O rule). */
export function useSubmitVerdict() {
  return useMutation({ mutationFn: (args: SubmitVerdictArgs) => submitVerdict(args) });
}

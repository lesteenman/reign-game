/**
 * Brand-colored 32×32 spinning loader. CSS keyframes (not Tamagui
 * animations) because Tamagui's animation system targets discrete
 * pseudo-state transitions (`hoverStyle`, `pressStyle`) rather than
 * continuous infinite rotations — those are still better expressed
 * with a hand-rolled `@keyframes`.
 *
 * Honors `prefers-reduced-motion`: the @media block sets `animation:
 * none` so the indicator falls back to a static donut shape for
 * users who've opted out of motion (WCAG 2.3.3 / SC 2.3.3 Animation
 * from Interactions).
 *
 * Extracted from `features/daily/screens/DailyFlow.tsx` in the #176
 * chrome-cards slice — was an inline `function Spinner()` colocated
 * with the loading + submitting states. Now reusable across any
 * async surface (future: TanStack `isPending` indicators in
 * curation's PlayPuzzlePage, admin's pool-status fetcher, etc.).
 */
export function Spinner() {
  return (
    <div
      role="status"
      aria-label="Loading"
      style={{
        width: 32,
        height: 32,
        borderRadius: '50%',
        border: '3px solid var(--color-border)',
        borderTopColor: 'var(--color-accent)',
        animation: 'reign-spinner-spin 800ms linear infinite',
      }}
    >
      <style>{`
        @keyframes reign-spinner-spin {
          to { transform: rotate(360deg); }
        }
        @media (prefers-reduced-motion: reduce) {
          [role="status"][aria-label="Loading"] { animation: none; }
        }
      `}</style>
    </div>
  );
}

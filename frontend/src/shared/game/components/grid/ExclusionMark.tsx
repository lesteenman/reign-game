import type { ExclusionMarkProps } from "@reign/core/theme";

/** Small dot indicating an excluded cell. */
export function ExclusionMark({ size }: ExclusionMarkProps) {
  const center = size / 2;
  const radius = size * 0.08;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      aria-hidden="true"
    >
      <circle cx={center} cy={center} r={radius} fill="currentColor" />
    </svg>
  );
}

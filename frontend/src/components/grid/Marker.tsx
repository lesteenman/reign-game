import type { MarkerProps } from "../../theme/types";

/** Filled circle marker for the Tactile theme. */
export function Marker({ size, regionIndex: _regionIndex }: MarkerProps) {
  const radius = size * 0.35;
  const center = size / 2;

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

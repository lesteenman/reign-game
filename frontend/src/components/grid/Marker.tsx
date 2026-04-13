import type { MarkerProps } from "../../theme/types";

/** Rounded square marker for the Tactile theme. */
export function Marker({ size, regionIndex: _regionIndex }: MarkerProps) {
  const padding = size * 0.18;
  const sideLength = size - padding * 2;
  const rx = 3;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      aria-hidden="true"
    >
      <rect
        x={padding}
        y={padding}
        width={sideLength}
        height={sideLength}
        rx={rx}
        fill="currentColor"
      />
    </svg>
  );
}

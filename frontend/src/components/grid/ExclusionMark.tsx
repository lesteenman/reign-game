import type { ExclusionMarkProps } from "../../theme/types";

/** SVG cross mark indicating an excluded cell. */
export function ExclusionMark({ size }: ExclusionMarkProps) {
  const padding = size * 0.25;
  const start = padding;
  const end = size - padding;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      aria-hidden="true"
    >
      <line
        x1={start}
        y1={start}
        x2={end}
        y2={end}
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
      />
      <line
        x1={end}
        y1={start}
        x2={start}
        y2={end}
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
      />
    </svg>
  );
}

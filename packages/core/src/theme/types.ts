import type React from "react";

/** Props for a theme's marker (placed piece) component. */
export interface MarkerProps {
  size: number;
  regionIndex: number;
}

/** Props for a theme's exclusion-mark (eliminated cell) component. */
export interface ExclusionMarkProps {
  size: number;
}

/**
 * A visual theme that controls how markers, exclusion marks,
 * and animations are rendered on the puzzle grid.
 */
export interface Theme {
  id: string;
  name: string;
  marker: React.ComponentType<MarkerProps>;
  exclusionMark: React.ComponentType<ExclusionMarkProps>;
  animations: {
    placement: string;
    conflict: string;
    completion: string;
  };
}

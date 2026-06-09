import { ExclusionMark } from "@shared/game/components/grid/ExclusionMark";
import { Marker } from "@shared/game/components/grid/Marker";
import type { Theme } from "@reign/core/theme";

/** The default "Tactile" theme — minimalist filled circles and thin crosses. */
export const tactileTheme: Theme = {
  id: "tactile",
  name: "Tactile",
  marker: Marker,
  exclusionMark: ExclusionMark,
  animations: {
    placement: "animate-placement",
    conflict: "animate-conflict",
    completion: "animate-completion",
  },
};

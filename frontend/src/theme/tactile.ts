import { ExclusionMark } from "../components/grid/ExclusionMark";
import { Marker } from "../components/grid/Marker";
import type { Theme } from "./types";

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

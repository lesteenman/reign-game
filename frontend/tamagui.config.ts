// Minimal Tamagui 2 RC configuration. The full token/theme migration
// from theme/tactile.ts and component-level usage lands in Track 3
// (code review + refactor). This file exists as the foundation so
// `import { ... } from 'tamagui'` works and the architecture skill's
// "Tamagui configured at frontend root" rule has a concrete file to
// point at.
//
// When Track 3 starts:
// 1. Replace the placeholder tokens below with values pulled from
//    `theme/tactile.ts`.
// 2. Define light + dark themes (consume `theme/useDarkMode.ts` to
//    decide which one).
// 3. Add the Tamagui Vite plugin to `vite.config.ts` for
//    build-time style extraction.
// 4. Wrap `App.tsx` in `<TamaguiProvider config={config}>`.
// 5. Migrate first component (recommended: shared/components/Button).

import { defaultConfig } from "@tamagui/config/v4";
import { createTamagui } from "tamagui";

const config = createTamagui(defaultConfig);

export default config;

export type AppConfig = typeof config;

declare module "tamagui" {
  // Inform Tamagui's type system about our config so component
  // props (e.g. `<Button backgroundColor="$primary" />`) typecheck
  // against our tokens. Currently uses Tamagui defaults; Track 3
  // swaps these for the Reign palette.
  interface TamaguiCustomConfig extends AppConfig {}
}

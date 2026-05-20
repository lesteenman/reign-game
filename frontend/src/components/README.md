# `src/components/`

Cross-feature React components organized by subdomain. Each subfolder has its own README; this file is a router.

| Subfolder | Responsibility | README |
|---|---|---|
| `common/` | Cross-feature UI primitives: PageShell, Button variants, button styles, press handlers. | `common/README.md` |
| `grid/` | Custom hand-built grid UI: Grid, Cell, Marker, ExclusionMark, RegionBorderOverlay. | `grid/README.md` |
| `landing/` | Landing-page-specific UI: PuzzleSelector. | `landing/README.md` |

Note: `components/game/` (VerdictSurface) moved to `shared/game/components/` in Track 3. `components/auth/ProtectedAdminRoute` moved to `features/admin/components/` in Track 3. The rest of `components/auth/` (`ClerkAvailability`, `SignInButton`, `UserMenu`, `role.ts`) moved to `shared/auth/` (#176 — cross-feature usage required a shared-layer home; see issue body for the BR analysis).

# `src/components/`

Legacy cross-feature React components organized by subdomain. New code does NOT land here — the BR-correct layer is `src/shared/` (cross-feature reusables) or `src/features/<feature>/` (feature-internal). What remains here is awaiting its own #176 slice.

| Subfolder | Responsibility | README |
|---|---|---|
| `grid/` | Custom hand-built grid UI: Grid, Cell, Marker, ExclusionMark, RegionBorderOverlay. Moves to `features/game/components/` in a later #176 slice. | `grid/README.md` |

Already migrated out of this folder:

- `components/game/` (VerdictSurface) → `shared/game/components/` (Track 3). Later moved to `features/curation/components/` in #176.
- `components/auth/ProtectedAdminRoute` → `features/admin/components/` (Track 3).
- `components/auth/` (`ClerkAvailability`, `SignInButton`, `UserMenu`, `role.ts`) → `shared/auth/` (#176, PR #196).
- `components/common/*` (`PageShell`, `Button`, `buttonStyles`, `press`, `OfflineBanner`, `InstallButton`) → `shared/components/` (#176, PR #197).
- `components/landing/PuzzleSelector` → `features/curation/components/` (#176, this PR — the folder was always misnamed; `PuzzleSelector` is curation-only).

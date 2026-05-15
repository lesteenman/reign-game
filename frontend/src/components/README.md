# `src/components/`

Cross-feature React components organized by subdomain. Each subfolder has its own README; this file is a router.

| Subfolder | Responsibility | README |
|---|---|---|
| `auth/` | Clerk integration: provider availability flag, sign-in button, user menu, role extractor, admin-route gate. | `auth/README.md` |
| `common/` | Cross-feature UI primitives: PageShell, Button variants, button styles, press handlers. | `common/README.md` |
| `game/` | Game-specific (non-grid) UI: VerdictSurface for admin curation. | `game/README.md` |
| `grid/` | Custom hand-built grid UI: Grid, Cell, Marker, ExclusionMark, RegionBorderOverlay. | `grid/README.md` |
| `landing/` | Landing-page-specific UI: PuzzleSelector. | `landing/README.md` |

## Track 3 mapping

Target architecture moves this directory out entirely:
- `auth/` → `features/auth/components/`
- `common/` → `shared/components/`
- `game/` + `grid/` → `features/game/components/`
- `landing/PuzzleSelector` → `features/curation/components/` (it's only used by `CurationPage`, not `LandingPage`)

See `frontend/INDEX.md` (top-level layout migration table) for the full mapping.

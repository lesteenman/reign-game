# features/

Product features, each self-contained in a Bulletproof React feature-folder.

## Layout

```
features/
  <feature>/
    pages/       Components mounted directly by the router (one or a few per feature).
    screens/     Sub-flow components composed by a page — NOT router-mounted.
    components/  Leaf components specific to this feature.
    hooks/       Feature-specific hooks (own I/O via useQuery/useMutation).
    services/    OPTIONAL — feature-specific API surface.
    types/       Feature-specific types.
```

## Rules

- No cross-feature imports. `features/X` never imports from `features/Y`. Cross-feature dependencies go through `shared/`, `engine/`, or `theme/`.
- `pages/` holds ONLY router-mounted components.
- `screens/` holds sub-flow components used inside a page.
- No page-to-page imports, even within the same feature.

See `frontend/CLAUDE.md` and `.claude/skills/architecture/SKILL.md` for the full ruleset and drift-detection greps.

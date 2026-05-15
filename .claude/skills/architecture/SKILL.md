---
name: architecture
description: Per-subproject architecture rules and design-time + review-time checks. Read at design-time (during brainstorming) to validate a proposed change against layered/feature-folder rules, and at review-time (during requesting-code-review) to grep the diff for cross-layer violations. Covers backend (handler/service/domain layering), frontend (Bulletproof React feature-folders, hooks own I/O, Tamagui primitives), and infra (modules vs envs). Triggers: any new feature, structural refactor, or PR review.
---

# Architecture Skill

Per-subproject rules. Invoked at two moments in the workflow:

1. **Design-time check** — inside Superpowers `brainstorming` once a design starts taking shape. Validates the design against the rules below. Outputs a verdict: pass / "redesign the X layer" / "extract a Y interface".
2. **Review-time drift check** — inside Superpowers `requesting-code-review`. Greps the diff for cross-layer violations. Outputs a finding list with file:line.

The corresponding subdirectory `CLAUDE.md` files (`backend/CLAUDE.md`, `frontend/CLAUDE.md`, `infra/CLAUDE.md`) reproduce the rules close to the code; this skill is the canonical source the review and design phases consult.

---

## Backend — Three-layer architecture

| Layer | Directory | Allowed callees | Forbidden callees |
|---|---|---|---|
| **Handler** (frontend) | `backend/internal/handler/` | service | repository, queue, AWS SDK |
| **Service** (application) | `backend/internal/service/` | domain, repository | handler |
| **Domain** (generic + repo-callers) | `backend/internal/domain/`, `backend/internal/repository/` | AWS SDK, external libs | handler, service |

### Design-time check (backend)

When a proposed change introduces or modifies a backend file, ask:

1. Which layer does this file belong to (handler/service/domain/repository)?
2. What does the new code call? Are all callees in the allowed-callees list for that layer?
3. If a handler needs persistence: does the design route through a service, or did it shortcut into the repository?
4. If a service needs another service: are they in the same bounded context? Cross-service composition is fine via interfaces; circular service-to-service dependencies are not.

If the answer to (3) is "shortcut," redesign to insert a service layer.

### Review-time drift check (backend)

Run these greps against the diff (`git diff main...HEAD --name-only` for the file list, then per file):

```sh
# Handler importing repository or queue directly — forbidden
grep -rn "internal/repository\|internal/queue" backend/internal/handler/

# Service importing handler — forbidden (would be a cycle)
grep -rn "internal/handler" backend/internal/service/

# Domain importing handler or service — forbidden
grep -rn "internal/handler\|internal/service" backend/internal/domain/ backend/internal/repository/
```

Any non-empty result is a finding. Report as: `architecture: <layer> drift in <file>:<line> — imports <forbidden-callee>; route through <correct-layer>`.

---

## Frontend — Bulletproof React feature-folders

```
frontend/src/
  app/          app composition (router, providers, entry)
  engine/       domain layer — pure TS, no React, no I/O (→ @reign/core later)
  features/     product features
    auth/  game/  daily/  curation/  admin/  landing/
  shared/       cross-cutting reusables
    components/ Tamagui-wrapped primitives
    hooks/      generic hooks
    lib/        api base, fetch utilities
    types/      cross-feature types
  theme/        design tokens (→ @reign/core later)
  storage/      IndexedDB wrapper
```

### Rules

| Rule | What it means | Why |
|---|---|---|
| **No cross-feature imports** | `features/X` never imports from `features/Y` | Features must be independently deletable |
| **Shared kernel only** | Cross-feature dependencies go via `shared/`, `engine/`, or `theme/` | Single source of truth |
| **Pages import features, not vice versa** | The route table in `app/` imports features; no feature imports a page from another feature | Clean import direction |
| **No `services/*` imports below `pages/`** | Leaf components consume hooks; hooks own I/O | Testable, composable, no hidden side effects |
| **`engine/` is pure** | No React, no I/O, no DOM, no `fetch`. Only imports external libs | It's the cross-platform domain |
| **`app/` is the top** | Nothing imports from `app/` | It composes the rest |
| **Tamagui for chrome** | Use `tamagui` package components for Button, Sheet, Dialog, Select, etc. | Cross-platform accessibility via Radix internals |
| **Custom on Tamagui primitives** | Game UI (Grid, Cell, Marker) uses `<View>`/`<Stack>`/`<Text>` from `tamagui` | Same code path, ready for RN |
| **No raw Tailwind in new code** | Tamagui props or theme tokens only | Tailwind is being retired in Track 3 |
| **Server state via TanStack** | `useQuery`/`useMutation` for backend reads/writes | Eliminates manual LoadState boilerplate |
| **Client state via React** | `useState`/`useReducer` for in-component state | No Zustand/Redux until cross-feature client state emerges |

### Design-time check (frontend)

When a proposed change introduces or modifies a frontend file:

1. Which feature does it belong to? (Or shared/engine/theme/app/storage?)
2. Does it import from another feature? Reject.
3. Is it a leaf component or a page? If leaf, does it import `services/*`? Reject — should consume a hook.
4. Is it consuming the backend? If so, design uses `useQuery`/`useMutation`, not manual fetch.
5. Is it visual? If so, are primitives from Tamagui or hand-rolled HTML? New chrome components must use Tamagui.

### Review-time drift check (frontend)

```sh
# Cross-feature imports — forbidden
for f in $(ls frontend/src/features/); do
  grep -rn "from .*features/[a-z]" "frontend/src/features/$f" | grep -v "features/$f"
done

# Leaf I/O — forbidden (services imported below pages)
find frontend/src/features/*/components -name "*.tsx" -exec grep -l "from .*services/" {} \;

# Engine purity — forbidden (React, fetch, DOM)
grep -rn "from 'react'\|fetch(\|document\.\|window\." frontend/src/engine/

# Cross-direction — forbidden (anything imports from app/)
grep -rn "from .*app/" frontend/src/ --include='*.ts' --include='*.tsx' | grep -v "from '\\.\\./app"

# Manual LoadState — flag for review (not strictly forbidden in legacy code, but discouraged in new)
grep -rn "useState<LoadState>\|useState<.*FlowState>" frontend/src/

# Raw Tailwind in new code — flag for review
git diff --name-only main...HEAD -- 'frontend/src/**/*.tsx' | xargs grep -l 'className=' 2>/dev/null
```

Any non-empty result is a finding. Report as: `architecture: frontend drift in <file>:<line> — <rule>; <fix>`.

### Known legacy violations (pre-Track-3)

These exist today; Track 3 will refactor:
- `frontend/src/components/auth/ProtectedAdminRoute.tsx` imports from `pages/`
- `frontend/src/components/game/VerdictSurface.tsx` calls `submitVerdict()` and `updatePuzzleStatus()` directly
- `frontend/src/services/dailyService.ts` bypasses `api.ts` for header injection
- `frontend/src/pages/DailyGameBoard.tsx` imports from `pages/GamePage.tsx`
- `frontend/src/hooks/useGame.ts` re-exports `cellKey` (consumed by `grid/Grid.tsx`)

When the review-time check fires on these, mark as "pre-Track-3 known violation, do not block this PR" unless the PR is specifically Track-3 refactor work.

---

## Infra — Modules vs envs

| Layer | Directory | Rule |
|---|---|---|
| **Modules** | `infra/modules/*/` | Self-contained, reusable. **Must NOT reference each other directly.** Composition happens at env level. |
| **Environments / root** | `infra/` (root for now), `infra/envs/*` (future per-env) | Calls modules and wires them together. Must NOT define inline resources that belong in a module. |

### Design-time check (infra)

1. Is the new resource cross-cutting (logging, monitoring, IAM root role)? → Root or env, not a module.
2. Is the new resource specific to one service (Lambda, queue, table)? → A module for that service.
3. Does the new module need data from another module? → Compose at the env layer (pass module-A output to module-B input), don't reference module-A from inside module-B.

### Review-time drift check (infra)

```sh
# Module-to-module references — forbidden
grep -rn 'module\.' infra/modules/

# Inline resources at root that should be in a module
grep -E '^resource\b' infra/*.tf | grep -vE 'aws_(provider|terraform|backend)'
```

Findings: `architecture: infra drift in <file>:<line> — module-to-module reference`. The second grep is a smell, not always a violation; review case-by-case.

### CI/CD symmetry rule

Critical sub-rule: **CI's `terraform plan` and CD's `terraform apply` must pass the same `TF_VAR_*` set.** Mismatched vars cause phantom plan diffs that mask real changes. When adding a new `TF_VAR_*` or `-var=`:
- Update both `.github/workflows/cd.yml` (apply step) AND `.github/workflows/ci.yml` (plan step).
- If the var is only meaningful at apply (not at plan), document the asymmetry inline with a comment explaining why.

This rule lives here as well as in `infra/CLAUDE.md`. Drift between the two is what caused issue #155 and the silent-CD incident.

---

## Invocation cheat-sheet

| Context | What to do |
|---|---|
| Starting Superpowers `brainstorming` for a feature | Read this skill. Validate the emerging design against the rules above. Append verdict to the brainstorm output. |
| Starting Superpowers `writing-plans` for a feature | Read the verdict from the design-time check. Each task in the plan should respect the rules. |
| Running Superpowers `requesting-code-review` | Run all three review-time greps. Add findings to the review output with severity. |
| `code-review-final` agent | Same as above; this skill is the canonical check. |
| Anyone proposing to add a new top-level directory | Stop. Read this skill. The directory tree is part of the architecture. |

## Updating this skill

When a layered rule changes (new layer, renamed directory, new forbidden callee), update:
1. This file (canonical)
2. The relevant subdir CLAUDE.md (`backend/CLAUDE.md` / `frontend/CLAUDE.md` / `infra/CLAUDE.md`)
3. Any test that codifies the rule (lint config, custom check)

Drift between these is itself a finding — the review process should catch it.

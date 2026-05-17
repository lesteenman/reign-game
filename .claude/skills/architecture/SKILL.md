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

## Backend — Layered architecture

Entry points (`cmd/*/main.go`) compose dependencies and dispatch to the appropriate edge package. Two edges exist: `handler/` for HTTP and `worker/` for SQS consumers. Both depend on `service/` for orchestration. The service layer is where multi-leg DDB transactions live, NOT in the repository.

| Layer | Directory | Allowed callees | Forbidden callees |
|---|---|---|---|
| **Edge: HTTP** | `backend/internal/handler/` | service, mode, httperr, generator (only for the `/api/puzzles/generate` debug endpoint) | repository, queue, AWS SDK directly |
| **Edge: SQS consumer** | `backend/internal/worker/` | service, mode, generator | handler, repository, queue directly |
| **Service** (application) | `backend/internal/service/` | repository, queue, domain, mode, generator, awsclient | handler, worker |
| **Persistence** | `backend/internal/repository/`, `backend/internal/queue/` | AWS SDK, domain, mode | handler, worker, service |
| **Pure / domain** | `backend/internal/domain/` (if created), `backend/internal/mode/`, `backend/internal/generator/` | external libs only | anything else under `internal/` |
| **Infra adapters** | `backend/internal/awsclient/`, `backend/internal/auth/`, `backend/internal/httperr/` | AWS SDK, external libs, domain | handler, worker (callable but not imported) |

**Notes on the moves codified by Track 3:**
- `MarksPerUnitFromMode`, `ModeStandard`, `ModeDouble`, `isMode` live in `backend/internal/mode/`. Both `handler/` and `worker/` import them from there.
- Multi-leg DDB transactions (`SubmitPlayTransactionally`, `FinalizeDailyTransaction`) move from `repository/daily.go` into a `service/daily/` package.
- Handlers that currently call repository directly (admin_pool, admin_config, daily, replenish, serve, status, verdict, config_modes, etc.) refactor to call service interfaces.

### Design-time check (backend)

When a proposed change introduces or modifies a backend file, ask:

1. Which layer does this file belong to (edge/service/persistence/pure/infra-adapter)?
2. What does the new code call? Are all callees in the allowed-callees list for that layer?
3. If a handler or worker needs persistence: does the design route through service, or shortcut into repository/queue?
4. If a service needs another service: are they in the same bounded context? Cross-service composition is fine via interfaces; circular service-to-service dependencies are not.
5. If the new code is a multi-step DDB operation: it goes in service, not repository. Repository methods are single transactions of single-row scope OR a single `TransactWriteItems`/`TransactGetItems` call with no orchestration logic.

If the answer to (3) is "shortcut," redesign to insert a service call.

### Review-time drift check (backend)

Run these greps against the diff (`git diff main...HEAD --name-only` for the file list, then per file):

```sh
# Handler or worker importing repository/queue directly — forbidden
grep -rn "internal/repository\|internal/queue" backend/internal/handler/ backend/internal/worker/

# Service importing handler/worker — forbidden (cycle)
grep -rn "internal/handler\|internal/worker" backend/internal/service/

# Worker importing handler — forbidden (worker is a peer, not "behind" handler)
grep -rn "internal/handler" backend/internal/worker/

# Repository/queue importing service or above — forbidden (cycle)
grep -rn "internal/handler\|internal/worker\|internal/service" backend/internal/repository/ backend/internal/queue/

# Pure layers importing anything internal/ — forbidden
grep -rn "internal/handler\|internal/worker\|internal/service\|internal/repository\|internal/queue" backend/internal/mode/ backend/internal/generator/ backend/internal/domain/ 2>/dev/null
```

Any non-empty result is a finding. Report as: `architecture: <layer> drift in <file>:<line> — imports <forbidden-callee>; route through <correct-layer>`.

---

## Frontend — Bulletproof React feature-folders

```
frontend/src/
  app/          app composition (router, providers, entry)
  engine/       domain layer — pure TS, no React, no I/O (→ @reign/core later)
  features/     product features (each one self-contained)
    auth/
      pages/        components mounted by the router (one or a few)
      screens/      OPTIONAL — sub-flow components used inside a page, NOT routed
      components/   leaf components specific to this feature
      hooks/        feature-specific hooks (own I/O via useQuery/useMutation)
      services/     OPTIONAL — feature-specific API surface (or skip; wire useQuery directly)
      types/        feature-specific types
    game/  daily/  curation/  admin/  landing/  (same shape)
  shared/       cross-cutting reusables (Tamagui-wrapped chrome, generic hooks, api base, cross-feature types)
  theme/        design tokens (→ @reign/core later)
  storage/      IndexedDB wrapper (or fold into shared/lib/storage/ in Track 3 if it stays single-purpose)
```

### Rules

| Rule | What it means | Why |
|---|---|---|
| **No cross-feature imports** | `features/X` never imports from `features/Y` | Features must be independently deletable |
| **Shared kernel only** | Cross-feature dependencies go via `shared/`, `engine/`, or `theme/` | Single source of truth |
| **`pages/` are routes only** | `features/X/pages/` holds ONLY components mounted directly by the router. Sub-flow components (intermediate screens within a flow) live under `features/X/screens/`. Leaf components live under `features/X/components/`. | Eliminates page-to-page imports; clear distinction between "routed" and "rendered-by-another-component" |
| **No page-to-page imports** | A page never imports another page, even within the same feature | Use `screens/` for shared sub-flow components |
| **No `services/*` imports below `pages/`** | Leaf components and screens consume hooks; hooks own I/O. Direct `import { fooFn } from 'services/...'` from a leaf or screen is a violation | Testable, composable, no hidden side effects |
| **No type imports from `services/*`** | Type definitions belong in `types/` or `engine/`; services may re-export types but consumers must import them from the source | One source of truth per type |
| **`engine/` is pure** | No React, no I/O, no DOM, no `fetch`. Only imports external libs | It's the cross-platform domain |
| **`app/` is the top** | Nothing imports from `app/` | It composes the rest |
| **Tamagui for chrome** | Use `tamagui` package components for Button, Sheet, Dialog, Select, etc. | Cross-platform accessibility via Radix internals |
| **Custom on Tamagui primitives** | Game UI (Grid, Cell, Marker) uses `<View>`/`<Stack>`/`<Text>` from `tamagui` | Same code path, ready for RN |
| **No raw Tailwind** | Tamagui props or theme tokens only. Tailwind is retired in Track 3. | One styling system |
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
for f in $(ls frontend/src/features/ 2>/dev/null); do
  grep -rn "from .*features/[a-z]" "frontend/src/features/$f" | grep -v "features/$f"
done

# Page-to-page imports — forbidden
grep -rn "from .*pages/" frontend/src/features/*/pages/ 2>/dev/null

# Leaf I/O (components / screens importing services) — forbidden.
# `shared/` is included because shared leaf components (e.g. shared/game/components/)
# are subject to the same rule — Track 3 review M1 found a violation slipping
# through when only features/ was scoped.
find frontend/src/features/*/components frontend/src/features/*/screens frontend/src/shared/*/components -name "*.tsx" 2>/dev/null \
  | xargs grep -l "from .*services/" 2>/dev/null

# Type imports from services — forbidden. Both forms:
#   import type { Foo } from 'services/...'    (whole-import)
#   import { bar, type Foo } from 'services/...'   (mixed-syntax)
# The whole-import grep alone misses the mixed form — Track 3 review M2 found
# one slipping through via `useSubmitVerdict.ts: import { submitVerdict, type SubmitVerdictArgs } from '...services/verdictService'`.
grep -rEn "from .*services/" frontend/src/ --include='*.ts' --include='*.tsx' 2>/dev/null | grep -E "(^[^:]+:[0-9]+:import type|, type )"

# Engine purity — forbidden (React, fetch, DOM)
grep -rn "from 'react'\|fetch(\|document\.\|window\." frontend/src/engine/

# Anything imports from app/ — forbidden
grep -rn "from .*app/" frontend/src/ --include='*.ts' --include='*.tsx' | grep -v "from '\\.\\./app"

# Manual LoadState — flag for review (legacy migration target)
grep -rn "useState<LoadState>\|useState<.*FlowState>" frontend/src/

# Raw Tailwind anywhere — forbidden
grep -rn 'className=' frontend/src/ --include='*.tsx' --include='*.ts'
```

Any non-empty result is a finding. Report as: `architecture: frontend drift in <file>:<line> — <rule>; <fix>`.

### Known legacy violations

None — all Track 3 violations resolved on branch `chore/track-3-code-review-refactor` (commit history under the `Track 3` prefix). New violations discovered after merge should be filed as architecture-drift issues in GitHub.

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

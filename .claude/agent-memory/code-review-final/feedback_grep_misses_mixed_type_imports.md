---
name: grep-misses-mixed-type-imports
description: The architecture skill's frontend type-import grep misses the mixed-syntax form `import { foo, type Bar }` — only catches whole-import `import type {...}`. Use a broader pattern for review-time greps.
metadata:
  type: feedback
---

When checking the rule "No type imports from `services/*`" during code review, the architecture-skill grep `grep -rn "import type .* from .*services/"` catches only the whole-import form. TypeScript also allows `import { foo, type Bar } from '...services/...'` (inline-marked types), and the architecture grep silently misses it.

**Why:** Found one real instance in Track 3 review (`frontend/src/shared/game/hooks/useSubmitVerdict.ts:2` — `import { submitVerdict, type SubmitVerdictArgs } from '.../services/verdictService'`). Drift gate let it through.

**How to apply:** When running review-time greps for this rule, also run `grep -rn ", type " frontend/src/ | grep "services/"` as a follow-up. Consider proposing a SKILL.md update so the canonical grep catches both forms — e.g. `grep -rEn "from .*services/" frontend/src/ | grep -E "(import type|, type )"`.

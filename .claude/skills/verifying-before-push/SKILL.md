---
name: verifying-before-push
description: Use the moment you are about to `git push` a branch, or before `gh pr create`/`gh pr merge` — any time finished work (feature, fix, refactor) is about to leave your machine. Fires BEFORE the push, especially when the diff crosses a service boundary (frontend↔backend, backend↔DB/SQS, frontend↔SW, frontend↔CloudFront edge) or adds/changes an integration or Playwright e2e spec. Triggers include the thoughts "CI will run it", "I'll let the pipeline catch it", or pushing an e2e/integration spec you have not run locally.
---

# Verifying Before Push

## Core principle

**The integration verification runs on YOUR machine before the push — CI is the backstop, never the first run.** A push that defers the real-wire check to CI is an unverified push.

**Violating the letter of this gate is violating the spirit of it.**

This is the enforcement point for CLAUDE.md Change-Workflow step 6 (Integration verification) + step 9 (hard gate). It exists because that gate is easy to *intend* and skip under momentum — and a skipped real-wire check is exactly how `#179` (HEAD/405) and `#329` (e2e assertion bug) shipped/failed.

## The gate — every box, before `git push`

1. **Build + unit tests + lint green locally** (the pre-push hook covers most; confirm output, don't assume).
2. **Cross-boundary change → run the real wire locally.** If the diff crosses frontend↔backend, backend↔DB/SQS, frontend↔SW, or frontend↔CloudFront: exercise the actual contract (a `playwright-cli` smoke, a LocalStack itest, or the durable e2e spec) **against a running, seeded stack** — and *see it pass*. Skip ONLY when the diff stays within one layer.
3. **A new/changed integration or e2e spec MUST be executed locally before push.** Authoring a spec and pushing so CI runs it first is forbidden — you have not verified it. Bring up the stack (`task e2e:up` / `task dev:up`), seed what it needs, run the spec, watch it pass, tear down.
4. **Code review + security gate** have run on the branch diff (per the Change Workflow), findings resolved.

Only then `git push`.

## Red flags — STOP, you are about to push unverified

- "CI will run the e2e / integration test." → Then YOU haven't. Run it locally first.
- "I wrote the spec, the pipeline is its first run." → Forbidden (step 3).
- "It's a cross-boundary change but unit tests pass on both sides." → Mocked-both-sides hides wire divergence (`#179`). Run the real wire.
- "The stack isn't up / seeding is a hassle." → That cost is the point of the gate. Bring it up.
- "I'll push now and verify while CI runs." → Verify, then push.

## Rationalizations

| Excuse | Reality |
|--------|---------|
| "CI is the authoritative gate, so CI running it counts." | CI is the *backstop*. If the spec fails there, you pushed a red branch and pay a full cycle. Run it locally first; CI then confirms. |
| "The contract is statically obvious / DTOs were mirrored." | Static match ≠ run. `#329` had a correct contract and still failed at runtime (auto-advance unmounted the board). Only running it surfaces that. |
| "It's just a small change." | Small cross-boundary changes break wires too. The skip, not the size, is the failure. |
| "Bringing up the stack is slow." | Slower than a failed CI cycle + a debug session? No. |

## What this is not

Not a replacement for CI — CI still runs everything. This guarantees the human-in-the-loop step (you) verified the real wire *before* spending a CI cycle, so CI confirms rather than discovers.

> Baseline (RED): `#329` — a Playwright e2e spec was authored and pushed expecting CI to run it first; it failed in CI on an assertion the author never executed locally. This skill fires at the push to force the local run.

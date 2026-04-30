# Spec: Glossary Terms

New domain terms introduced by Phase 7. Each is added to `GLOSSARY.md` in the slice's docs sweep (folded into R-082's PR per `tasks.md`).

The terms below MUST appear in `GLOSSARY.md` with the exact wording specified, in the indicated section, before Phase 7 closes. Code, specs, and PR descriptions reference them; drift between the glossary and the artifacts is a review-blocker.

## GT-01: Verdict (new term)

**Section:** Puzzle Lifecycle (between `Curated Puzzle` and `Daily Puzzle`).

**Definition:**

> **Verdict**
> A rater's judgment of a puzzle as a piece of content — `up` (good puzzle, keep it) or `down` (bad puzzle, retire it). Distinct from `Status`, which describes what happened during a play attempt (solved / skipped). Submitted via `PUT /api/admin/puzzles/{id}/verdict` after a play attempt ends. Persisted as a per-rater row in the `puzzle-pool` DynamoDB table; a denormalized `Verdict Summary` projection on the `PuzzleRecord` carries the running counts. In Phase 7, only `Admin` role users can submit verdicts; the schema is multi-rater-ready for a future public-rater role.

**Value.** Disambiguates "verdict" (curation judgment) from "status" (play-attempt outcome). The two were colliding in the ROADMAP wording — this entry locks the meaning.

## GT-02: Verdict Summary (new term)

**Section:** Puzzle Lifecycle (immediately after `Verdict`).

**Definition:**

> **Verdict Summary**
> The denormalized verdict projection stored on `PuzzleRecord` as the `verdictSummary` attribute: `{up: int, down: int, lastUpdatedAt: ISO 8601}`. Recomputed on every verdict write by reading the row family for the puzzle. The verdict row family is the source of truth; the summary is a cached projection — recoverable by re-running `RecomputeVerdictSummary` if it ever drifts.

**Value.** Names the projection so `design.md`, `repository.md`, and `proposal.md` can reference it without spelling out the contract every time.

## GT-03: Rater (new term)

**Section:** Users & Access (after `Admin`, before `Premium`).

**Definition:**

> **Rater**
> The role that submits verdicts on puzzles. In Phase 7, only `Admin` users are raters — the verdict route lives under `/api/admin/*` and is gated by the Phase 6 admin middleware chain. The verdict row schema is keyed by `(raterRole, raterId)` to leave room for a future public-rater role; that role is not yet defined and is explicitly out of Phase 7 scope.

**Value.** Names the abstract concept ("the entity that submits a verdict") so the schema and design docs have a stable noun for what is, today, just an admin. Avoids forcing a rename when the public-rater role lands.

## GT-04: Verdict Surface (new term)

**Section:** Puzzle Lifecycle (after `Verdict Summary`).

**Definition:**

> **Verdict Surface**
> The frontend UI affordance through which a `Rater` submits a `Verdict` — two buttons ("Good puzzle" / "Bad puzzle") rendered after a play attempt ends (completion overlay or post-skip transient state). Cosmetically gated by `publicMetadata.role === 'admin'`; the source of truth is the backend `RequireAdmin` middleware. Lives inside `frontend/src/components/game/VerdictSurface.tsx`.

**Value.** Names the UI artifact distinctly from the abstract `Verdict` concept. When a future spec talks about "moving the Verdict Surface to a different page," there is no ambiguity about what is moving.

## GT-05: Status (clarification — existing concept, no glossary change required)

**Section:** Puzzle Lifecycle (existing entries — `Candidate Puzzle`, `Curated Puzzle`).

**Rule.** No new entry. `Status` is already implicit in the `Puzzle Lifecycle` section through `Candidate Puzzle` and `Curated Puzzle`. The `Verdict` entry (GT-01) explicitly contrasts with status to prevent confusion. If a future slice promotes `Status` to a glossary term, that entry must reference `Verdict` symmetrically.

**Verification.** Grep `GLOSSARY.md` after the docs sweep: every new term above appears verbatim in the section indicated. No term is duplicated. Cross-references (`Verdict` → `Verdict Summary`, `Verdict Summary` → `Verdict`, `Rater` → `Verdict`, `Verdict Surface` → `Rater` and `Verdict`) are bidirectional.

## GT-06: Term consistency in code

**Rule.** The Go and TypeScript identifiers match the glossary terms with conventional casing:

| Glossary term | Go identifier | TS identifier |
|---|---|---|
| Verdict (the value) | `Verdict` (struct field), `value` (DTO) | `value` (DTO) |
| Verdict Summary | `VerdictSummary` (struct), `verdictSummary` (DDB attr / JSON) | `VerdictSummary` (interface) |
| Rater | `RaterID`, `RaterRole` (struct fields) | `raterId`, `raterRole` (when surfaced) |
| Verdict Surface | (no Go-side counterpart) | `VerdictSurface` (component) |

**Value.** A reviewer reading `repository.go` and `GLOSSARY.md` side-by-side sees the same nouns. No `JudgmentRecord` or `RatingProjection` synonyms creep in.

**Verification.** Grep at slice-close: every `Verdict*` identifier in the diff maps to a glossary entry; no synonym (`Judgment`, `Rating`, `Vote`, `Score`) appears in new code or docs unless explicitly defined.

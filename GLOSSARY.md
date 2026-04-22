# Glossary

Shared vocabulary for this project. Managed via the `/glossary` skill.

Consult this file before using domain terms in specs, designs, and code. If a term is missing or ambiguous, use `/glossary add <term>` to propose a definition.

---

## Grid & Board

**Grid**
The N x N square board on which the puzzle is played. Defined by its size (e.g., 5x5, 7x7, 9x9).

**Cell**
A single square within the grid, identified by its row and column coordinates (zero-indexed).

**Region**
A contiguous group of cells sharing the same color. Each region must contain exactly the required number of queens (1 in Standard Mode, 2 in Double Queens Mode). Regions are irregular in shape and do not follow a fixed pattern.

**Region Map**
The data structure defining which region each cell belongs to. A 2D array of region IDs.

## Pieces & Constraints

**Marker**
The generic term for the piece placed by the player on a cell. The visual representation depends on the active theme (dot, queen, gem, flower, etc.). In code and specs, always use "marker" — never a theme-specific term.

**Queen**
A theme-specific marker icon (chess queen). Used in the Queens classic theme. Not the default.

**Adjacency Constraint**
No two queens may occupy cells that are horizontally, vertically, or diagonally adjacent. This applies to all queens regardless of region membership.

**Row Constraint**
Each row must contain exactly the required number of queens (1 in Standard, 2 in Double Queens).

**Column Constraint**
Each column must contain exactly the required number of queens (1 in Standard, 2 in Double Queens).

**Region Constraint**
Each region must contain exactly the required number of queens (1 in Standard, 2 in Double Queens).

## Game Modes

**Standard Mode**
The default game mode. One queen per row, column, and region. The adjacency constraint applies.

**Double Queens Mode**
An advanced game mode. Two queens per row, column, and region. The adjacency constraint applies. Requires larger solution spaces and deeper deduction.

## Difficulty

**Easy**
5x5 grid, 5 regions. Simple region shapes, solvable with basic elimination.

**Medium**
7x7 grid, 7 regions. Irregular region shapes, requires systematic elimination logic.

**Hard**
9x9 grid, 9 regions. Complex interlocking regions, requires multi-step deduction chains.

**Deduction Chain**
A sequence of logical elimination steps required to determine a queen's placement. Deeper chains indicate higher difficulty.

## Puzzle Lifecycle

**Candidate Puzzle**
A puzzle produced by the generator that has not yet been curated. May be accepted, rejected, or modified.

**Curated Puzzle**
A puzzle that has been reviewed and approved by a human curator. Assigned to a practice pool or scheduled as a daily puzzle.

**Daily Puzzle**
A curated puzzle assigned to a specific date, mode, and difficulty level. The same puzzle for all players worldwide on that date.

**Practice Puzzle**
A curated puzzle available in the practice pool. Can be played at any time, without time pressure or leaderboard scoring.

**Puzzle ID**
A stable, unique identifier for a puzzle. Used for leaderboard references, completion records, and sharing.

## Scoring & Competition

**Completion**
A record of a player finishing a puzzle, including the puzzle ID, completion time, and timestamp.

**Completion Time**
The elapsed time in seconds from the player's first cell interaction to submitting a correct solution.

**Percentile Rank**
Where a player's completion time falls relative to all players who completed the same daily puzzle. Expressed as a percentage (e.g., "Top 15%").

**Absolute Position**
The player's numeric rank out of total completions for a daily puzzle (e.g., "512 / 1,247").

**Streak**
The count of consecutive days on which the player completed at least one daily puzzle.

## Users & Access

**Device Identity**
An anonymous, device-linked identifier used to associate completions and local stats before account creation (Phase 1).

**User Account**
An authenticated identity (via OAuth) that enables cross-device sync, persistent leaderboard names, and premium features (Phase 2).

**Free Tier**
Default access level. Includes daily challenges, limited practice puzzle pool, and basic stats.

**Premium Tier**
One-time purchase. Includes full puzzle archive, leaderboard identity, detailed stats, cross-device sync, custom themes, and future premium features.

## Themes

**Theme**
A coherent visual package that changes the game's appearance: marker icon, grid style, region rendering, background, animations, and color palette. Purely cosmetic — gameplay mechanics are identical across themes.

**Tactile Theme**
The default theme. Abstract and clean with physical depth: filled circle markers, thick ink borders between regions, bold saturated region fills, layered offset shadows. The brand identity comes from the tactile style and warm ink palette.

**Queens Classic Theme**
A free built-in theme with chess-inspired visuals: queen markers, traditional grid, parchment background, crown flourish animations.

**Premium Theme**
A theme available only to premium tier subscribers. Examples: Gems, Garden, Neon, Cosmos.

**Theme Token**
A design variable (color, spacing, animation config, icon reference) consumed by UI components via React Context. Components never hard-code visual values — they read theme tokens.

## Technical

**Puzzle Generator**
The algorithm that produces candidate puzzles. Takes parameters (grid size, mode, target difficulty) and outputs a puzzle definition with its solution. Open source (MIT).

**Puzzle Solver**
The algorithm that validates solutions and verifies puzzle uniqueness. Used both in the generator pipeline and for server-side completion validation.

**Region Shape Complexity**
A measure of how irregular a region's shape is. More complex shapes create harder puzzles because elimination logic becomes less intuitive.

---

## Generator (Phase 5+)

Terms in this section are internal to the Phase 5 generator rework (`backend/internal/generator`). They name concepts inside the new algorithm pipeline and the difficulty classifier. Consult this section before writing or reviewing generator code — every term here has a single agreed-upon meaning.

**Solution Sampler**
The first stage of the generator pipeline. Produces a valid marker configuration (row/column counts satisfied, adjacency constraint honored) via row-by-row backtracking with `uint16` bitmasks and k-combination enumeration per row. Input: N and marksPerUnit (k). Output: a fully marked grid with exactly `N*k` markers. Does NOT produce regions — that is the Region Grower's job. Supersedes the old "sampler" terminology used in `input-spec.md` with a k-parameterized implementation.

**Deductive Solver**
The rule-based solver used during generation to prove a candidate puzzle is solvable by pure deduction. Applies tiered rules R1..R9 in a fixed-point loop (on any rule firing, restart from Tier 1) until Solved, Stalled, or Contradiction. Used both (a) inside the generator to classify difficulty via the Rule Trace it emits, and (b) in the orchestrator's accept/reject gate — a puzzle that cannot be solved by the Deductive Solver alone is discarded. Distinct from the Brute Solver; cross-checked against it in test builds.

**Brute Solver**
A pure-function backtracking solver (`bruteSolveAll(regionMap, n, k, maxSolutions)`) that exists independently of the Deductive Solver. Used exclusively to prove uniqueness — a candidate puzzle is accepted only if the Brute Solver returns exactly one solution. Running it with `maxSolutions=2` short-circuits as soon as a second solution is found. Its output is cross-checked against the Deductive Solver's solution in test builds; divergence is a hard failure because it indicates an unsound rule (locked decision #8).

**Rule Trace**
The ordered record of every deductive rule that fired during a Deductive Solver run on a candidate puzzle. Each trace event records the rule ID (R1..R9), the tier (1..4), and the cells or candidates it eliminated. The Classifier reads the Rule Trace to compute `MaxTier`, `TierCounts`, and `TraceLen`. Trace recording is **toggleable**: off during region-grower scoring (hot-loop allocation hygiene), on during the final classification pass.

**Mutation Loop**
The iterative region-shape refinement stage that runs when the Deductive Solver stalls on a grown Region Map. On each iteration the Mutator attempts a single-cell boundary swap between two 4-adjacent regions; the swap is accepted only if it strictly increases the Deductive Solver's solved-cell count AND preserves 4-connectivity of both regions AND keeps all Region Seeds in their regions. Capped at `K` iterations per attempt (default K=50, configurable via `WithMaxMutations`). Exceeding K discards the attempt and restarts from the Sampler.

**Region Seed**
A group of `k` marker positions (from the Sampler's solution) that the Region Grower uses as the starting cells of one region. Exactly `N` seeds exist per puzzle (one per region). For k=1 each seed is a single marker; for k=2 the Pairer groups the 2N markers into N seed-pairs via greedy nearest-neighbor Manhattan pairing. A region always contains all `k` of its seeds — this invariant is enforced by the Grower and defended by the Mutator.

**Expert (difficulty tier)**
The top tier of the generator's `Difficulty` classification, assigned when `MaxTier == 4` in the Rule Trace (i.e., the puzzle requires the Deductive Solver's Tier 4 rule R8 — the k-parameterized X-wing analogue — to solve). Persisted on `PuzzleRecord.Difficulty`. **Not** surfaced to players in v1 — the difficulty field is computed and stored but the player-facing difficulty selector is R-034 (Phase 9). Sits above Easy (MaxTier ≤ 1), Medium (MaxTier == 2), and Hard (MaxTier == 3). Distinct from the existing "Hard" entry in the Difficulty section above, which describes a grid-size tier (9x9) for the player UI, not a rule-trace-based classification.

---

## Testing

Terminology for how tests in this project are categorized. The category describes what the test exercises, not which tool runs it — a Playwright test can be either category depending on whether the backend is real or mocked.

**End-to-end test (e2e)**
A test that exercises the full stack running locally: the React frontend, the Go backend, and LocalStack (DynamoDB + SQS). Preference is for normal user flows — click buttons, read the DOM. Direct database peeks or API inspection are allowed when useful (verify `status=served` after a play, seed fixture rows via `task e2e:seed`), but should not replace the user-flow assertion. The canonical e2e suite is the Playwright `e2e` project under `frontend/playwright/e2e/`; fixtures live in `frontend/playwright/e2e/fixtures/puzzles/*.json`. E2E tests point at a second backend instance on `:5182` backed by a separate DynamoDB table (`puzzle-pool-e2e`) so the dev pool is never touched by a test run.

**Integration test**
A test that exercises one side of the system — frontend OR backend — with multiple units running together and other services mocked. Examples: a Vitest file that renders `AdminPage.tsx` with a mock fetch and a real `adminService` is a frontend integration test. A Go test that wires `ReplenishHandler` + `repository.PuzzleRepository` + a fake `queue.Publisher` is a backend integration test. Playwright tests that use `page.route` to mock `/api/*` responses are frontend integration tests, not e2e — they do not cross the HTTP boundary. The canonical frontend Playwright integration suite is the `integration` project under `frontend/playwright/integration/`.

---

<!-- Add new terms below as they emerge from design discussions -->

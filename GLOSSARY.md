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

<!-- Add new terms below as they emerge from design discussions -->

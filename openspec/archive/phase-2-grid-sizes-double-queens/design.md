# Phase 2: Design Document

Authoritative design reference for Phase 2 implementation. See design-grill-summary.md for the full decision log and rationale.

## Generator Architecture (R-020, R-030, R-031)

### Pluggable Interface

The generator pipeline splits into two pluggable components:

```go
type SolverStrategy interface {
    // GenerateSolution produces a random valid marker placement.
    GenerateSolution(gridSize int, markersPerUnit int, timeout time.Duration) ([][]bool, error)
    // CountSolutions returns the number of valid solutions (stops at 2).
    CountSolutions(regionMap [][]int, gridSize int, markersPerUnit int) int
}

type RegionStrategy interface {
    // GenerateRegions grows regions around solution markers.
    GenerateRegions(solution [][]bool, gridSize int, opts RegionOpts) ([][]int, error)
}

type RegionOpts struct {
    Variance    float64 // 0.0 = uniform, 1.0 = max variation
    MinSize     int     // 3 for standard, 4 for double
}
```

The top-level `Generate` function accepts strategy choices and delegates:

```go
func Generate(gridSize int, mode Mode, solver SolverStrategy, regions RegionStrategy, opts GenerateOpts) (*Puzzle, error)
```

The generation loop remains: generate solution → grow regions → verify uniqueness → retry if needed.

### Solver Strategies

**Backtrack (current):** Row-by-row backtracking with random column permutations. For Double Queens, picks `markersPerUnit` columns per row (iterating over column combinations). `placedCol` and `placedRegion` change from `[]bool` to `[]int` counters. `markerCols` changes from `[]int` to `[][]int`.

**Constraint Propagation (new):** Same backtracking structure, plus forward-looking logic after each placement:
1. Mark same-row, same-column, adjacent cells as unavailable
2. Mark same-region cells as unavailable if region is at quota
3. If any unit has zero available cells → backtrack immediately (fail fast)
4. If any unit has exactly one available cell → place automatically, propagate again

Produces different puzzle personality: forced moves bias the solution distribution toward tighter, more constrained arrangements.

### Region Strategies

**BFS (current):** Round-robin BFS growth from marker seed cells. Modified to support variable region sizes: each region gets a target size drawn from a distribution controlled by `Variance`. Min size enforced (3 for standard, 4 for double).

**WFC (new):** Wave Function Collapse for region assignment. Each cell starts with all region IDs as possibilities. Iteratively collapse the lowest-entropy cell (fewest possibilities), propagate constraints (connectivity, size bounds). Lowest-entropy-first produces more organic, varied region shapes than BFS.

### Variable Region Sizes

Regions no longer need to be exactly `gridSize` cells. The `Variance` parameter controls the spread:

- 0.0: all regions have `gridSize` cells (uniform, matches Phase 1)
- 1.0: maximum variation within constraints (min size 3/4, total = gridSize^2)

The generator distributes cells across regions, respecting:
- Minimum size: 3 (standard) or 4 (double queens)
- Total cells: `gridSize * gridSize`
- Number of regions: `gridSize`
- Contiguity: each region must be connected

### Double Queens Mode

Standard mode: `markersPerUnit = 1`. Double Queens: `markersPerUnit = 2`.

Double Queens constraints:
- Each row has exactly 2 markers
- Each column has exactly 2 markers
- Each region has exactly 2 markers
- No two markers are adjacent (unchanged)

Total markers on an N×N grid: 2N.

**Phase 2 restricts Double Queens to 9x9 only.** The API accepts any size+mode combination, but the standard UI only offers 9x9 Double Queens. Expand after playtesting.

### Benchmarks

Benchmark suite covers all 4 strategy combinations at validated sizes:
- 5x5 Standard
- 7x7 Standard
- 9x9 Standard
- 9x9 Double Queens

Each benchmark measures: average generation time, p95 generation time, retry rate, success rate within timeout. Strategies that consistently exceed 3 seconds at a given size are flagged.

## API (R-020, R-030)

### Generate Endpoint

```
GET /puzzles/generate?size=7&mode=standard&solver=backtrack&regions=bfs&regionVariance=0.5
```

Parameters:
- `size`: integer, 3-15 (required)
- `mode`: `standard` | `double` (required)
- `solver`: `backtrack` | `propagation` (optional, default `backtrack`)
- `regions`: `bfs` | `wfc` (optional, default `bfs`)
- `regionVariance`: float 0.0-1.0 (optional, default `0.0`)

Response shape unchanged:
```json
{
  "puzzleId": "550e8400-e29b-41d4-a716-446655440000",
  "gridSize": 7,
  "mode": "standard",
  "regionMap": [[...]]
}
```

Error responses unchanged (400 for invalid params, 500 for generation failure). The handler validates ranges but does not restrict to UI-exposed combinations.

## Frontend (R-022, R-033)

### Landing Page Changes

The landing page gains a mode/size selector when starting a new game:

**Standard selectors (always visible when starting new game):**
Four preset buttons: 5x5 Standard, 7x7 Standard, 9x9 Standard, 9x9 Double Queens.

**Advanced mode (collapsed by default):**
Toggle reveals: free-form grid size input (3-15), mode toggle, solver strategy dropdown, region generator dropdown, region variance slider (5 discrete stops: Uniform, Slight, Moderate, High, Wild mapping to 0.0, 0.25, 0.5, 0.75, 1.0).

Flow:
- No active puzzle: show selectors + "Play" button
- Active puzzle: show "Resume" + "New Puzzle". "New Puzzle" reveals selectors.

### Game Page Changes

**Header:** Back button and dark mode toggle share a header row. Exact layout deferred to UI/UX designer, but both must be consistently positioned (e.g. back top-left, dark mode top-right).

**Adaptive cell sizing:** Minimum cell size varies by grid size:
- 5x5, 7x7: 44px minimum
- 9x9: 38px minimum
- Larger (advanced mode): scales down further based on available width

### Constraint Parameterization

All constraint functions accept `markersPerUnit`:

```typescript
checkRowConstraint(cells, gridSize, markersPerUnit): Conflict[]
checkColumnConstraint(cells, gridSize, markersPerUnit): Conflict[]
checkRegionConstraint(cells, regionMap, markersPerUnit): Conflict[]
checkAdjacencyConstraint(cells, gridSize): Conflict[]  // unchanged
getAllConflicts(cells, regionMap, gridSize, markersPerUnit): Conflict[]
```

Flag when count > `markersPerUnit` (not > 1).

### Completion

`validateSolution` accepts `markersPerUnit`. Solved when `markerCount === gridSize * markersPerUnit && conflicts.length === 0`.

`useGame` hook receives `markersPerUnit` derived from the puzzle's mode.

### API Client

`puzzleService.ts` expanded to pass all parameters:

```typescript
interface GenerateOptions {
  size: number
  mode: 'standard' | 'double'
  solver?: 'backtrack' | 'propagation'
  regions?: 'bfs' | 'wfc'
  regionVariance?: number
}

function generatePuzzle(options: GenerateOptions): Promise<PuzzleData>
```

### Game State

No schema changes. The existing `GameState.puzzle` field stores `gridSize` and `mode`. Generator strategy parameters are not persisted (only the resulting puzzle matters). Single active game — starting a new puzzle discards the old one.

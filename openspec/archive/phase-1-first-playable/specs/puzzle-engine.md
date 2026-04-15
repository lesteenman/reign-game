# Spec: Puzzle Engine

Covers R-010 (data model), R-011 (solver), R-012 (generator), R-013 (generate endpoint).

## Requirements

### PE-01: Puzzle Data Model

- `internal/model/puzzle.go` defines `Puzzle` struct: `ID string`, `GridSize int`, `Mode string`, `RegionMap [][]int`, `Solution [][]bool`
- `RegionMap[row][col]` is the region ID (0-indexed) for that cell
- For a 5x5 grid: 5 regions (IDs 0–4), each containing exactly 5 contiguous cells
- `Solution[row][col]` is true if a marker belongs at that cell
- `Mode` is `"standard"` for Phase 1 (one marker per row/column/region)

### PE-02: Region Validation

- A region map is valid if:
  - All region IDs are in range `[0, gridSize)`
  - Each region contains exactly `gridSize` cells
  - Each region is contiguous (connected via horizontal/vertical adjacency)
- `internal/generator/region.go` provides a `ValidateRegionMap(regionMap [][]int, gridSize int) error` function
- Table-driven tests covering valid maps, non-contiguous regions, wrong cell counts, out-of-range IDs

### PE-03: Constraint Solver

- `internal/generator/solver.go` implements constraint-based solving
- Takes a puzzle (grid size, region map) and returns all valid solutions
- Checks four constraints: row (1 marker per row), column (1 marker per column), region (1 marker per region), adjacency (no two markers horizontally, vertically, or diagonally adjacent)
- Returns the number of solutions found (0, 1, or 2+ — stops searching after finding 2)
- Table-driven tests: puzzles with 0, 1, and multiple solutions

### PE-04: Puzzle Generator

- `internal/generator/generator.go` produces valid 5x5 Standard Mode puzzles
- Each generated puzzle has exactly one valid solution (verified by solver)
- Regions are contiguous and each contains exactly one solution marker
- Generator retries internally if a candidate fails uniqueness check
- Configurable generation timeout (default 5 seconds, returns error if exceeded)
- Table-driven tests verifying generated puzzles satisfy all constraints
- Benchmark tests in `generator_bench_test.go` measuring generation time across N runs

### PE-05: Region Shape Generation

- `internal/generator/region.go` generates random contiguous region maps around a given solution
- Heuristics for region shape variety are isolated and testable (not hardcoded into the generation loop)
- Tests verify generated regions are valid per PE-02

### PE-06: Generate Endpoint

- `internal/handler/generate.go` handles `GET /puzzles/generate`
- Query params: `size` (required, integer), `mode` (required, string)
- Phase 1 accepts only `size=5` and `mode=standard` — returns 400 for other values
- Success response (200): `{"puzzleId": "<uuid>", "gridSize": 5, "mode": "standard", "regionMap": [[...]]}`
- `puzzleId` is a UUID v4 generated server-side
- Solution is never included in the response
- Generation failure response (500): `{"error": "generation_failed", "message": "..."}`
- Invalid params response (400): `{"error": "invalid_params", "message": "..."}`
- Table-driven tests for success, invalid size, invalid mode, and generation failure (mocked)

### PE-07: Chi Router Registration

- Generate endpoint registered on the existing chi router in `cmd/api/main.go`
- Route: `GET /puzzles/generate`
- Health check (`GET /health`) continues to work

## Acceptance Criteria

All PE-01 through PE-07 requirements pass. Generator produces valid 5x5 puzzles with unique solutions. Benchmark suite runs and reports generation times. Endpoint returns correct JSON and error codes.

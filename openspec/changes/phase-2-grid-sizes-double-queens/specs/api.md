# Spec: API Expansion

Covers R-020 (larger grids) and R-030/R-031 (Double Queens) from the API perspective.

## Requirements

### AP-01: Expanded Parameter Validation

- `internal/handler/generate.go` accepts expanded parameter values:
  - `size`: integer 3-15 (was: only 5)
  - `mode`: `standard` | `double` (was: only `standard`)
  - `solver`: `backtrack` | `propagation` (new, optional, default `backtrack`)
  - `regions`: `bfs` | `wfc` (new, optional, default `bfs`)
  - `regionVariance`: float 0.0-1.0 (new, optional, default `0.0`)
- Invalid values return 400 with descriptive error message
- Tests: table-driven for valid combos, out-of-range size, invalid mode, invalid solver, invalid regions, invalid variance

### AP-02: Strategy Parameter Mapping

- Handler maps `solver` and `regions` string params to the corresponding strategy implementations
- Default strategy: backtrack solver + BFS regions (matches Phase 1 behavior)
- `regionVariance` passed through to `RegionOpts.Variance`
- `MinSize` derived from mode: 3 for `standard`, 4 for `double`
- `markersPerUnit` derived from mode: 1 for `standard`, 2 for `double`
- Tests: verify correct strategy selection for each parameter value

### AP-03: Response Shape

- Response JSON unchanged: `{puzzleId, gridSize, mode, regionMap}`
- No strategy parameters in the response (client doesn't need to know how the puzzle was generated)
- `mode` in response matches the request (`"standard"` or `"double"`)
- Tests: verify response shape for standard and double modes

### AP-04: Error Messages

- Generation failure (500): generic message, no internal details (matches Phase 1 security stance)
- Invalid params (400): descriptive but non-leaking (e.g., "size must be between 3 and 15")
- Tests: error responses have correct status codes and message format

## Acceptance Criteria

All AP-01 through AP-04 pass. API accepts all valid parameter combinations. Defaults match Phase 1 behavior. Error messages are descriptive but not leaking.

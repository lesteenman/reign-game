# Phase 4: Backend Specs

## BE-10: Config Repository Methods

Add to `internal/repository/puzzle.go`:

### ConfigRecord struct

```go
type ConfigRecord struct {
    Size           int     `dynamodbav:"-"`
    Mode           string  `dynamodbav:"-"`
    Pipeline       string  `dynamodbav:"pipeline"`
    Solver         string  `dynamodbav:"solver"`
    Regions        string  `dynamodbav:"regions"`
    RegionVariance float64 `dynamodbav:"regionVariance"`
    Deducible      bool    `dynamodbav:"deducible"`
    Concurrency    int     `dynamodbav:"concurrency"`
    Threshold      int     `dynamodbav:"threshold"`
    Enabled        bool    `dynamodbav:"enabled"`
}
```

### Methods

- `GetAllConfigs(ctx) ([]ConfigRecord, error)` — Query PK="CONFIG", parse SK into Size/Mode.
- `GetConfig(ctx, size, mode) (*ConfigRecord, error)` — GetItem PK="CONFIG", SK="{size}#{mode}". Returns nil, nil if not found.
- `PutConfig(ctx, config *ConfigRecord) error` — PutItem with PK="CONFIG", SK="{size}#{mode}".
- `CreateConfig(ctx, config *ConfigRecord) error` — PutItem with `attribute_not_exists(PK)` condition. Returns a typed error on conflict.

### DynamoDBAPI interface update

Add `GetItem` to the interface:
```go
GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
```

### Tests (TDD)

- GetAllConfigs: empty table, multiple configs, parse SK correctly
- GetConfig: found, not found
- PutConfig: writes and reads back
- CreateConfig: success, conflict returns error

## BE-11: Admin Pool Handler

New file `internal/handler/admin_pool.go`.

### GET /admin/pool

Handler: `AdminPoolHandler(repo ConfigAndCountRepo) http.HandlerFunc`

Interface `ConfigAndCountRepo`:
```go
type ConfigAndCountRepo interface {
    GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
    CountReady(ctx context.Context, size int, mode string) (int, error)
}
```

Logic:
1. Call GetAllConfigs
2. For each config where `enabled=true`, call CountReady
3. For disabled configs, set readyCount=0
4. Return merged response

### Tests (TDD)

- All combos with counts
- Mix of enabled/disabled (disabled shows count=0)
- Empty config list returns empty combos array
- GetAllConfigs error returns 500

## BE-12: Admin Config Handlers

New file `internal/handler/admin_config.go`.

### PUT /admin/config/{size}/{mode}

Handler: `UpdateConfigHandler(repo ConfigRepo) http.HandlerFunc`

Logic:
1. Parse {size} and {mode} from URL path (chi)
2. Validate size (3-15 int), mode (standard/double)
3. Decode JSON body into config fields
4. Validate all fields (same rules as generate params)
5. Check config exists (GetConfig) — 404 if not
6. PutConfig with validated fields
7. Return 200 with updated config

### POST /admin/config

Handler: `CreateConfigHandler(repo ConfigRepo) http.HandlerFunc`

Logic:
1. Decode JSON body including size and mode
2. Validate all fields
3. Call CreateConfig — 409 on conflict
4. Return 201 with created config

### Validation

Reuse the same validation rules from `ParseGenerateParams`:
- pipeline: region-first, iterative, constraint-aware
- solver: backtrack, propagation
- regions: bfs, wfc
- regionVariance: 0.0-1.0, not NaN/Inf
- concurrency: 1-8
- threshold: >= 1
- size: 3-15
- mode: standard, double

### Tests (TDD)

- PUT: valid update, missing combo (404), invalid fields (400), missing body fields
- POST: valid create, duplicate (409), invalid fields (400)

## BE-13: Replenish Refactor

Modify `internal/handler/replenish.go`.

### Changes

1. Add `ConfigReader` to `ReplenishHandler` dependencies:
   ```go
   type ConfigReader interface {
       GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
   }
   ```
2. Replace hardcoded `sizeModeCombos` iteration with `GetAllConfigs()` call
3. Filter to `enabled=true` configs
4. Use config's `threshold` instead of `PoolThreshold` constant
5. Use config's generation params (pipeline, solver, regions, etc.) for SQS messages
6. Remove `PoolThreshold` constant and `sizeModeCombos` var
7. Keep optional size/mode query param filter (existing behavior)

### Tests (TDD)

- All configs enabled, all below threshold → triggers all
- Mix enabled/disabled → skips disabled
- Per-combo threshold respected (different thresholds)
- Generation params from config used in SQS message
- Filter by size/mode still works
- Empty config list → empty response

## BE-14: Route Registration

Update `cmd/api/main.go` `newRouter()`:

```go
// Admin routes
r.Get("/admin/pool", handler.AdminPoolHandler(repo))
r.Put("/admin/config/{size}/{mode}", handler.UpdateConfigHandler(repo))
r.Post("/admin/config", handler.CreateConfigHandler(repo))
r.Post("/admin/replenish", handler.ReplenishHandler(repo, pub))
```

Update `ReplenishHandler` signature to accept `ConfigReader` (the repo satisfies both interfaces).

### Tests

- Verify new routes are registered and reachable (integration-level)

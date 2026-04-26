# Phase 4: Design Document

Authoritative design reference for Phase 4 implementation. See design-grill-summary.md for the full decision log and rationale.

## CONFIG Items in DynamoDB (R-050)

### Schema

CONFIG items share the `puzzle-pool` table. They are distinguished by their PK.

| Attribute | Type | Description |
|-----------|------|-------------|
| `PK` | String | `CONFIG` (literal) |
| `SK` | String | `{size}#{mode}` (e.g., `7#standard`) |
| `pipeline` | String | `region-first`, `iterative`, `constraint-aware` |
| `solver` | String | `backtrack`, `propagation` |
| `regions` | String | `bfs`, `wfc` |
| `regionVariance` | Number | 0.0 - 1.0 |
| `deducible` | Boolean | Whether to enforce deducibility |
| `concurrency` | Number | 1 - 8 |
| `threshold` | Number | Minimum ready puzzles before replenish triggers (default 3) |
| `enabled` | Boolean | Whether replenish generates for this combo |

### Access Patterns

1. **List all configs:** `Query PK = "CONFIG"` — returns all combos with their settings.
2. **Get one config:** `GetItem PK = "CONFIG", SK = "{size}#{mode}"`.
3. **Update config:** `PutItem` with full config item (overwrite).
4. **Create config:** `PutItem` with condition `attribute_not_exists(PK)` to prevent accidental overwrites.

### Initial Configs (seed data)

| SK | pipeline | solver | regions | regionVariance | deducible | concurrency | threshold | enabled |
|----|----------|--------|---------|----------------|-----------|-------------|-----------|---------|
| `5#standard` | iterative | propagation | bfs | 0.0 | true | 1 | 3 | true |
| `7#standard` | iterative | propagation | bfs | 0.0 | true | 1 | 3 | true |
| `9#standard` | iterative | propagation | bfs | 0.0 | true | 1 | 3 | true |
| `7#double` | iterative | propagation | bfs | 0.0 | true | 1 | 3 | false |
| `9#double` | iterative | propagation | bfs | 0.0 | true | 1 | 3 | false |

Double Queens configs are `enabled=false` due to KI-007 (generation too slow).

## API Endpoints

### GET /admin/pool (R-051)

Returns all combos with merged config + ready puzzle counts.

**Response:**
```json
{
  "combos": [
    {
      "size": 5,
      "mode": "standard",
      "config": {
        "pipeline": "iterative",
        "solver": "propagation",
        "regions": "bfs",
        "regionVariance": 0.0,
        "deducible": true,
        "concurrency": 1,
        "threshold": 3,
        "enabled": true
      },
      "readyCount": 3
    }
  ]
}
```

**Implementation:** Query all CONFIG items, then for each enabled combo query ready count. Disabled combos return `readyCount: 0` without querying.

### PUT /admin/config/{size}/{mode} (R-052)

Updates an existing combo's config. Returns 404 if the combo doesn't exist.

**Request body:**
```json
{
  "pipeline": "iterative",
  "solver": "propagation",
  "regions": "bfs",
  "regionVariance": 0.0,
  "deducible": true,
  "concurrency": 1,
  "threshold": 3,
  "enabled": true
}
```

**Response:** 200 with the updated config. 400 on validation error. 404 if combo not found.

### POST /admin/config (R-053)

Creates a new combo. Returns 409 if it already exists.

**Request body:**
```json
{
  "size": 7,
  "mode": "double",
  "pipeline": "iterative",
  "solver": "propagation",
  "regions": "bfs",
  "regionVariance": 0.0,
  "deducible": true,
  "concurrency": 1,
  "threshold": 3,
  "enabled": false
}
```

**Response:** 201 with the created config. 400 on validation error. 409 if combo exists.

### POST /admin/replenish (R-054, R-055)

Existing endpoint, refactored. Reads CONFIG items dynamically instead of hardcoded list. Skips disabled combos. Uses per-combo `threshold` and generation params from config.

Optional filter: `?size=7&mode=standard` — only replenish this combo.

**Response:** Same shape as current (triggered/skipped arrays).

## Replenish Refactor (R-054)

Current state (`replenish.go`):
- Hardcoded `PoolThreshold = 3`
- Hardcoded `sizeModeCombos` list
- Hardcoded generation params (iterative/propagation/bfs/0.0/true/1)

Target state:
- Read all CONFIG items via repository
- Filter to `enabled=true` (and optional size/mode query filter)
- Use each config's `threshold` for the pool level check
- Use each config's generation params for the SQS message
- Remove `PoolThreshold` constant and `sizeModeCombos` var

## Frontend: Admin Page (R-056, R-057)

### Route

`/admin` — new route in App.tsx. Uses PageShell with back button to home.

### Navigation

Subtle link in PageShell header, right side, before the dark mode toggle. Text: "Admin" or a gear icon. Visible on all pages.

### Pool Table

Table with one row per combo showing:
- Size + Mode label (e.g., "7x7 Standard")
- Ready count / threshold (e.g., "3 / 3")
- Enabled status (toggle or badge)
- Replenish button (per-combo)
- Edit button (opens modal/expandable)

Global "Replenish All" button at the top.

### Config Editing

Modal or expandable section per combo. Fields:
- Pipeline (select: region-first, iterative, constraint-aware)
- Solver (select: backtrack, propagation)
- Regions (select: bfs, wfc)
- Region Variance (number input, 0.0-1.0)
- Deducible (toggle)
- Concurrency (number input, 1-8)
- Threshold (number input, >= 1)
- Enabled (toggle)

Save button calls PUT /admin/config/{size}/{mode}.

### Create Combo

Button to add a new combo. Opens form with size + mode fields plus all config fields. Calls POST /admin/config.

## LocalStack Seed (R-058)

Update `.localstack/init-aws.sh` to insert CONFIG items after table creation. Use AWS CLI `put-item` commands for the 5 initial configs.

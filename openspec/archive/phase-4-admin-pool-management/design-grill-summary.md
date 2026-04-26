# Design Grill Summary: Admin Pool Management (Phase 4)

## Final Design

Add admin-facing configuration and visibility for the puzzle pool. Config items live in the same `puzzle-pool` DynamoDB table (PK=`CONFIG`, SK=`{size}#{mode}`), storing per-combo generation parameters, pool threshold, and an `enabled` flag. The replenish handler reads config dynamically instead of using hardcoded combos. A new `/admin` page shows pool counts per combo with inline config editing and per-combo + global replenish buttons.

## Decisions

### 1. Config Storage

Same `puzzle-pool` table, not a new table. Config items use PK=`CONFIG`, SK=`{size}#{mode}`. Keeps the single-table design and avoids a new Terraform resource.

### 2. Pool Threshold

Per-combo, stored in the config item (field `threshold`, default 3). Different combos can have different pool depths.

### 3. Combo Discovery

Dynamic. The replenish handler scans CONFIG items instead of iterating a hardcoded list. Adding a new combo means creating a config item -- no code change required.

### 4. Config Schema

Each config item stores: `pipeline`, `solver`, `regions`, `regionVariance`, `deducible`, `concurrency`, `threshold`, `enabled`. The `enabled` boolean allows keeping a combo visible in the admin UI without generating puzzles for it (solves KI-007 Double Queens being too slow).

### 5. API Shape

Three config endpoints plus the existing replenish endpoint:

- `GET /admin/pool` -- returns all combos with merged config + ready counts
- `PUT /admin/config/{size}/{mode}` -- update one combo's config
- `POST /admin/config` -- create a new combo
- `POST /admin/replenish` -- existing, now with optional `?size=X&mode=Y` filter

No DELETE endpoint. Disabling a combo uses `enabled=false`.

### 6. Creating Combos

POST endpoint for creating new combos from the admin UI. Seed script populates initial configs for local dev. Without POST, adding combos would require CLI/seed commands only.

### 7. Pool Status Info

Counts only for now (ready puzzle count per combo). Richer details (status breakdown, generation history) deferred to later phases.

### 8. Counting Strategy

Per-combo queries: iterate CONFIG items, query each PK for ready count, aggregate client-side. Acceptable at current scale (5-10 combos).

### 9. Response Shape

Merged response: `GET /admin/pool` returns config fields and ready count together per combo. Avoids two round trips. The API is built for this app, not as a generic REST API.

### 10. Admin Navigation

Subtle link in PageShell header, next to the dark mode toggle. No sidebar or drawer.

### 11. Config Editing UX

Modal or expandable row for editing a combo's config. Simple approach for now -- can evolve later.

### 12. Replenish UX

Both global "Replenish All" button at the top and per-combo replenish icon on each row. Both call `POST /admin/replenish` -- per-combo passes `?size=X&mode=Y` query params.

## Deferred Items

- Rich pool status details (status breakdown, generation history) -- future phase
- DELETE config endpoint -- `enabled=false` is sufficient
- Difficulty rating integration -- Phase 5+
- Verdict/curation UI -- Phase 5+

## Constraints and Assumptions

- CONFIG items share the `puzzle-pool` table. PK=`CONFIG` is reserved and must never collide with puzzle PKs (which use `{size}#{mode}` format).
- The replenish handler skips combos where `enabled=false`.
- Per-combo query approach is acceptable at current scale (5-10 combos). If combo count grows significantly, a GSI or batch approach would be needed.
- Admin page has no auth gate this phase (same as existing `/admin/replenish`). Auth deferred to identity system.
- Config changes take effect on the next replenish run, not retroactively on existing pool puzzles.

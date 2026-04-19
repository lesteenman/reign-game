# Capability Delta: Frontend Admin UI

This is a **delta spec**. It amends `openspec/changes/phase-4-admin-pool-management/specs/frontend.md`. When Phase 5 lands, the strategy-matrix fields vanish from the admin UI.

Section IDs (FA-XX) are referenced from `tasks.md`.

---

## FA-01: `ConfigData` TypeScript shape

**Supersedes:** Phase 4 `FE-10` (adminService types).

**New shape:**

```ts
export interface ConfigData {
  deducible: boolean;
  threshold: number;
  enabled: boolean;
  maxAttempts?: number; // optional override; 0/undefined means default
}
```

**Removed fields:** `pipeline`, `solver`, `regions`, `regionVariance`, `concurrency`.

**Verification.** `frontend/src/services/adminService.ts` matches. `adminService.test.ts` updated to assert the reduced shape. `npx tsc -b` clean.

---

## FA-02: `AdminPage` `ConfigForm` simplified

**Supersedes:** Phase 4 `FE-11` (admin config form).

**Removed UI elements:**

- `PIPELINE_OPTIONS`, `SOLVER_OPTIONS`, `REGIONS_OPTIONS` constants and their `<select>` elements.
- `regionVariance` number input.
- `concurrency` number input.

**Retained UI elements:**

- `threshold` number input (min 1)
- `deducible` toggle
- `enabled` toggle
- `maxAttempts` optional number input (labelled "Max attempts (0 = default)")

**Closes:**

- **KI-015** — `ConfigForm` now takes a small, uniform set of props. The create-only vs edit-only split becomes trivial; if the form still looks worth splitting after this reduction, that's a Phase 5.5 cleanup, not Phase 5.
- **KI-016** — `PIPELINE_OPTIONS` and friends no longer exist, so the `string[]` vs typed-union mismatch resolves by deletion.

**Verification.** `AdminPage.tsx` tests updated. No `<select>` element references a pipeline/solver/regions option. KI-015 and KI-016 marked closed in ROADMAP.md.

---

## FA-03: Create-combo form simplified

**Supersedes:** Phase 4 `FE-12` (create combo form).

**New fields:** `size`, `mode`, plus all fields from FA-01.

**Validation:**

- `size` in [3, 15]
- `mode` in {"standard", "double"}
- `threshold` >= 1
- `maxAttempts` >= 0 (or unset)

**Verification.** POST `/api/admin/config` with the new payload creates a combo. POST with a duplicate returns 409; validation errors return 400 with a clear message.

---

## FA-04: Admin page retains pool-count display

**Unchanged from Phase 4 `FE-11`:** Pool table rows show `readyCount / threshold`, enabled status, per-row replenish button, edit button. Global replenish button at the top.

**Optional Phase 5 extension (not required):** display `difficulty` histogram per combo, read-only, when the count endpoint exposes it. This is a **non-requirement** for Phase 5 — just a hook for Phase 5.5 or Phase 9's difficulty selector work. Difficulty data lives on `PuzzleRecord` starting with Phase 5 (CS-01) but is not queried by `GET /api/admin/pool` in v1.

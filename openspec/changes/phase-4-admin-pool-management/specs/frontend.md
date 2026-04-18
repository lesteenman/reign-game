# Phase 4: Frontend Specs

## FE-10: Admin API Service

New file `src/services/adminService.ts`.

### Functions

- `fetchPoolStatus(): Promise<PoolStatus>` — GET /admin/pool
- `updateConfig(size, mode, config): Promise<ConfigData>` — PUT /admin/config/{size}/{mode}
- `createConfig(data): Promise<ConfigData>` — POST /admin/config
- `triggerReplenish(size?, mode?): Promise<ReplenishResult>` — POST /admin/replenish with optional query params

### Types

```typescript
interface ConfigData {
  pipeline: string;
  solver: string;
  regions: string;
  regionVariance: number;
  deducible: boolean;
  concurrency: number;
  threshold: number;
  enabled: boolean;
}

interface ComboStatus {
  size: number;
  mode: string;
  config: ConfigData;
  readyCount: number;
}

interface PoolStatus {
  combos: ComboStatus[];
}

interface ReplenishResult {
  triggered: Array<{ size: number; mode: string; count: number }>;
  skipped: Array<{ size: number; mode: string; ready: number }>;
}
```

### Tests (TDD)

- fetchPoolStatus: mock fetch 200, verify parsed response
- updateConfig: mock fetch 200/400/404
- createConfig: mock fetch 201/400/409
- triggerReplenish: with and without filter params

## FE-11: Admin Page Component

New file `src/pages/AdminPage.tsx`.

### Layout

- PageShell with back button to home
- "Replenish All" button at top
- Pool table showing all combos
- "Add Combo" button

### Pool Table Columns

| Column | Content |
|--------|---------|
| Combo | "{size}x{size} {Mode}" (e.g., "7x7 Standard") |
| Pool | "{readyCount} / {threshold}" |
| Enabled | Toggle or green/gray badge |
| Actions | Replenish icon button, Edit icon button |

### Behavior

- On mount: fetch pool status, show loading state
- Replenish All: call triggerReplenish(), refresh pool status, show toast/feedback
- Per-combo replenish: call triggerReplenish(size, mode), refresh that combo's count
- Edit: open modal/expandable with config fields
- Add Combo: open form with size + mode + config fields
- After save/create: refresh pool status

### Tests (TDD)

- Renders pool table with combo data
- Loading state shown initially
- Replenish All button calls API
- Per-combo replenish calls API with correct params
- Edit opens config form
- Error states handled

## FE-12: Config Edit Form

Component within AdminPage (modal or expandable section).

### Fields

- Pipeline: select (region-first, iterative, constraint-aware)
- Solver: select (backtrack, propagation)
- Regions: select (bfs, wfc)
- Region Variance: number input (0.0-1.0, step 0.1)
- Deducible: toggle/checkbox
- Concurrency: number input (1-8)
- Threshold: number input (min 1)
- Enabled: toggle/checkbox

### For Create mode

Additional fields:
- Size: number input (3-15)
- Mode: select (standard, double)

### Behavior

- Pre-fill fields from existing config (edit mode)
- Save calls PUT (edit) or POST (create)
- Validation matches backend rules
- Close on success, show error on failure (400/404/409)

### Tests (TDD)

- Renders all fields with correct values
- Save calls correct API endpoint
- Validation prevents invalid submissions
- 409 on create shows "already exists" message

## FE-13: Admin Navigation Link

Modify `src/components/common/PageShell.tsx`.

### Change

Add a link/button in the header right side, before the dark mode toggle. Options:
- Text: "Admin" (small, subtle)
- Or: gear icon

Links to `/admin` via React Router `<Link>`.

### Tests (TDD)

- Admin link renders in header
- Admin link navigates to /admin

## FE-14: Route Registration

Modify `src/App.tsx`.

### Change

Add route: `<Route path="/admin" element={<AdminPage />} />`

Import AdminPage lazily if desired (React.lazy + Suspense).

### Tests

- /admin route renders AdminPage

# Playwright suites

Two test projects live here, each matching one of the categories in the project glossary (`GLOSSARY.md` → Testing).

| Folder            | Project       | What it talks to                                            |
|-------------------|---------------|-------------------------------------------------------------|
| `integration/`    | `integration` | Frontend with `page.route` mocks — no backend required.     |
| `e2e/`            | `e2e`         | Real backend + real LocalStack, against seeded fixtures.    |

Playwright is the tool in both cases; the category comes from what the test actually exercises.

## Running the integration suite

No setup beyond `npm install`. The existing dev server on `:5180` is reused if running; otherwise Playwright's `webServer` config starts one.

```bash
npm run test:integration
```

## Running the e2e suite

The e2e project drives a **separate Vite on `:5183`** (proxying `/api/*` to the **separate backend on `:5182`** reading **`puzzle-pool-e2e`**). This keeps the dev stack on `:5180`/`:5181` untouched.

```bash
# one-off setup: brings up the e2e backend + frontend and seeds fixtures
task e2e:up

# run the suite
npm run test:e2e

# teardown when done
task e2e:down
```

LocalStack is shared with the dev stack — `task dev:up:localstack` works for both. The e2e isolation lives at the DynamoDB-table and backend-instance boundary.

### Fixtures

Committed under `e2e/fixtures/puzzles/*.json` as DynamoDB-Item JSON. Regenerate after intentional generator changes:

```bash
task e2e:genfixtures
```

Deterministic: the same seed + size + k produces byte-identical output. Re-committing means the tests stay reproducible.

Two fixtures with the same region map + different SKs are committed. That's a workaround for React StrictMode's dev-mode double-mount of `GamePage` — the first mount's cancelled fetch still triggers a server-side `status=served` update, so the second mount needs a fresh fixture to avoid 404. The proper fix (AbortController with a cancel-aware backend, or split serve-and-mark into two endpoints) is out of scope for R-06B.

## Running both

```bash
npm run test:playwright
```

Runs `integration` in parallel (the default), then `e2e` serially (one worker — fixture pool has one puzzle per combo).

## Known caveats

- **First `/api/config/modes` call can be slow.** LocalStack's DynamoDB cold-path occasionally sits at 5–10 s on the first request (KI-022). `task e2e:up` warms the backend; the e2e specs bump `toHaveCount` / `toBeVisible` timeouts to 15 s as belt-and-suspenders.
- **Don't re-run `npm run test:e2e` without re-seeding.** Each run serves the fixture (two rows → `status=served` after play-to-completion). Re-run `task e2e:seed` before another pass, or `task e2e:up` which re-seeds as part of its warmup.

## Current test coverage

Minimum viable for R-06B — full e2e coverage is tracked as R-080 in `ROADMAP.md`.

- **`e2e/play-to-completion.spec.ts`** — drives a 7×7 Standard puzzle to solved. Exercises `/api/config/modes`, `/api/puzzles/next`, cell-click, undo, redo, completion-overlay.
- **`e2e/dynamic-modes.spec.ts`** — asserts `/api/config/modes` filters by `enabled`. The e2e pool pins `9#double` to disabled; the test confirms the button does not render.
- **`integration/grid-interaction.spec.ts`** — the existing mocked-backend suite, unchanged. Covers cell three-tap, drag, undo/redo, solved-state UI.

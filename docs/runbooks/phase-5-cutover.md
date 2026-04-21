# Phase 5 generator cutover (R-069)

End-to-end runbook for replacing the old generator with the Phase 5 rework. Covers local dev (always) and prod (when the prod env exists; currently a single shared environment per CLAUDE.md).

## Before you start

- Phase 5 PRs (R-062 through R-068) merged to `epic/phase-5-generator-rework`, then to `main`.
- `bench/step11-handoff.md` read and understood — Lambda concurrency sizing will come from its yield + throughput tables.
- Stakeholders notified if any consumer is serving live traffic. (Dev only: no notification needed.)
- Local tools: `aws` CLI and `jq` on your PATH. macOS typically needs `brew install jq`; Linux distros have it in their package managers.

## 1. Drain the generator queue

**Local:**

```bash
task dev:down:generator
```

Confirms no new work picked up. In-flight messages naturally drain because the consumer exits.

**Prod** (once prod exists):

Set the generator Lambda's reserved concurrency to 0 in the AWS console (or via Terraform), then wait until the SQS queue's `ApproximateNumberOfMessagesNotVisible` reaches 0.

## 2. Flush the pool

Old-shape puzzles from the pre-R-067 generator must be removed so the new generator's output doesn't share the table with them. CONFIG rows are preserved.

**Local:**

```bash
TABLE_NAME=puzzle-pool \
  AWS_ENDPOINT_URL=http://localhost:4566 \
  AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  CONFIRM=YES \
  ./scripts/flush-pool.sh
```

**Prod:**

```bash
TABLE_NAME=puzzle-pool \
  AWS_PROFILE=<prod-profile> \
  CONFIRM=YES \
  ./scripts/flush-pool.sh
```

The `CONFIRM=YES` gate is deliberate — running the script from a shell history without it refuses to proceed.

## 3. Deploy the new backend + frontend

**Local:**

```bash
task dev:restart:backend
task dev:restart:frontend
# (LocalStack stays up; init-aws.sh is idempotent.)
```

**Prod:**

Merge `epic/phase-5-generator-rework` into `main` and let the CD workflow (`cd.yml`) ship the Lambda + S3/CloudFront deploy.

## 4. Re-seed the configs

Idempotent — safe to run even if configs already exist.

**Local:** handled by `.localstack/init-aws.sh` on container restart. If LocalStack is already up and the CONFIG rows are correct, skip.

**Prod:**

```bash
TABLE_NAME=puzzle-pool \
  AWS_PROFILE=<prod-profile> \
  ./scripts/seed-configs.sh
```

## 5. Re-enable the consumer + trigger replenish

**Local:**

```bash
task dev:up:generator
curl -X POST http://localhost:5181/api/admin/replenish
```

**Prod:** restore the Lambda's reserved concurrency (remove the override, or set back to the configured value). Then hit `POST /api/admin/replenish` once from the admin UI or a curl.

## 6. Verify

Before declaring cutover complete:

1. `curl http://localhost:5181/api/admin/pool` (or prod equivalent): every enabled combo has `readyCount > 0`.
2. At least one Double 9x9 row in the pool with `status=READY`. This is the KI-007 acceptance criterion — the old generator couldn't produce this in the Lambda budget; the new one finishes in milliseconds.
3. `GET /api/puzzles/next?size=9&mode=double` returns a puzzle whose `Regions` is `[[0,0,...],...]` (new shape) and whose `Solution` is `[{r,c},...]` (list of marks).

## 7. Close the KIs

Strike through and dated-close in `ROADMAP.md`:

- KI-007 (Double Queens generation speed) — closed by Phase 5 generator rework.

Other KIs that were originally scoped for R-069 closure but remain open (KI-015, KI-016) stay tracked for R-06A cleanup.

## 8. Update project-structure docs

Skim `PROJECT_STRUCTURE.md` for stale paths. If the Phase 5 merge changed generator subdirectory layout (it shouldn't have materially — `backend/internal/generator/` kept its shape), update the listing.

## Rollback

If the cutover produces broken puzzles:

1. `task dev:down:generator` (or prod: concurrency=0) to stop output.
2. `./scripts/flush-pool.sh` to drain the bad data.
3. `git revert` the `epic → main` merge and redeploy.
4. Re-seed configs + re-enable consumer on the previous version.

Because CONFIG rows survive the flush, no admin-UI reconfiguration is needed after rollback.

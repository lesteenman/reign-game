# GitHub Environments + Deployments + verification gate (acc) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or executing-plans. Steps use `- [ ]` checkboxes.

**Goal:** Run the acc CD deploy under a GitHub `acc` Environment (native Deployment records), gate the deployment status on a post-deploy verification step, and split CI's credentials to `CI_`-prefixed repo secrets/vars.

**Architecture:** Attach the `cd.yml` deploy job to `environment: { name: acc, url: … }` (records come free + the deploy secrets resolve from the env). Add a final verification step running a committed Playwright/fetch script against the live site. Rewire `ci.yml` to the `CI_*` set.

**Tech Stack:** GitHub Actions, Terraform (acc state), Playwright (frontend dep), AWS OIDC.

**Issue:** #241 · **DoR:** issue #241 comment · **Risk class:** hold-for-merge + `security-review-final`.

---

## Pre-merge prerequisites (NOT code — supervisor)
1. **OIDC trust policy** allows `repo:lesteenman/reign-game:environment:acc` sub (external role; verify/fix before merge). Hard blocker — secrets live in the `acc` env so `environment: acc` is mandatory.
2. **`acc` Environment + `CI_` secrets/vars created** (done — 16 entries verified).
3. **Merge #241 before #248** and before any other main merge — the bare repo-level secrets were deleted, so old-workflow runs fail until #241 swaps the references.

## File structure
- Modify `.github/workflows/cd.yml` — add `environment:`; add Playwright cache+install+verification steps.
- Create `frontend/scripts/verify-deploy.mjs` — headers + health + live-wire browser check.
- Modify `.github/workflows/ci.yml` — bare `secrets.*`/`vars.*` → `CI_*` (+ fix now-stale comments).
- Modify `infra/CLAUDE.md` — document env/secret model, `CI_` convention, verification gate.

---

### Task 1: Verification script (`frontend/scripts/verify-deploy.mjs`)

**Files:** Create `frontend/scripts/verify-deploy.mjs`

- [ ] **Step 1: Write the script.** Node 24 global `fetch` for headers/health; the frontend's Playwright (`chromium`) for the browser check. Takes the base URL as argv[2] (default `https://reign.acc.steenman.me`).
  - Assert the 6 security headers present on `/` (HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Content-Security-Policy, Permissions-Policy).
  - Assert `GET /api/health` → 200.
  - Browser: `goto /` then `/daily`; collect console errors; record every `**/api/**` response; **fail** if any console error, if zero `/api/*` responses were seen, or if any `/api/*` response status ≥ 400 (the #225-class catch — a stale bundle without `x-api-key` would 403).
  - Exit non-zero with a clear summary on any failure; exit 0 on all-pass.
- [ ] **Step 2: Run locally against live acc** (RED/GREEN — no secrets needed, it only probes the public site):
  Run: `cd frontend && npm ci && npx playwright install chromium && node scripts/verify-deploy.mjs https://reign.acc.steenman.me`
  Expected: prints header/health/wire results, "PASS", exit 0. If it fails, fix the script's assumptions (e.g. which route triggers the `/api` call) until green.
- [ ] **Step 3: Commit.** `git add frontend/scripts/verify-deploy.mjs && git commit -m "feat(ci): post-deploy verification script for the acc deployment gate (#241)"`

### Task 2: Attach `cd.yml` to the `acc` environment + wire verification

**Files:** Modify `.github/workflows/cd.yml`

- [ ] **Step 1: Add the environment** to the `deploy` job (after `runs-on: ubuntu-latest`):
  ```yaml
      environment:
        name: acc
        url: https://reign.acc.steenman.me
  ```
  No reference changes needed — `secrets.AWS_ROLE_ARN`, `secrets.CLERK_*`, `secrets.TF_VAR_TERRAFORM_STATE_BUCKET`, and the `vars.*` now resolve from the `acc` environment.
- [ ] **Step 2: Add Playwright cache + chromium install + verification** as the final steps (after "Invalidate CloudFront cache"), mirroring ci.yml's cache pattern (lines 118-131):
  ```yaml
      - name: Cache Playwright browsers
        uses: actions/cache@27d5ce7f107fe9357f9df03efb73ab90386fccae # v5.0.5
        with:
          path: ~/.cache/ms-playwright
          key: playwright-${{ runner.os }}-${{ hashFiles('frontend/package-lock.json') }}
      - name: Install Playwright chromium
        working-directory: frontend
        run: npx playwright install --with-deps chromium
      - name: Post-deploy verification
        working-directory: frontend
        run: node scripts/verify-deploy.mjs https://reign.acc.steenman.me
  ```
  Because this is the deploy job's last step, a failure fails the job → the native Deployment auto-marks `failure` (no script needed for records).
- [ ] **Step 3: actionlint** `cd.yml` (if available) / `python -c "import yaml,sys; yaml.safe_load(open('.github/workflows/cd.yml'))"` to confirm valid YAML.
- [ ] **Step 4: Commit.** `git commit -m "feat(cd): run deploy under acc environment + post-deploy verification gate (#241)"`

### Task 3: Rewire `ci.yml` to `CI_*`

**Files:** Modify `.github/workflows/ci.yml`

- [ ] **Step 1: Swap references** (these exact `${{ … }}` expressions; comments referencing the bare names get updated in step 2):
  - `secrets.CLERK_PUBLISHABLE_KEY` → `secrets.CI_CLERK_PUBLISHABLE_KEY` (lines 99, 175, 362)
  - `secrets.CLERK_SECRET_KEY` → `secrets.CI_CLERK_SECRET_KEY` (lines 235, 256, 363)
  - `secrets.AWS_ROLE_ARN` → `secrets.CI_AWS_ROLE_ARN` (line 314)
  - `secrets.TF_VAR_TERRAFORM_STATE_BUCKET` → `secrets.CI_TF_VAR_TERRAFORM_STATE_BUCKET` (line 334)
  - `vars.AWS_REGION` → `vars.CI_AWS_REGION` (lines 315, 336, 368)
  - `vars.TF_VAR_TERRAFORM_STATE_PREFIX` → `vars.CI_TF_VAR_TERRAFORM_STATE_PREFIX` (line 335)
  - `vars.DOMAIN_ALIASES` → `vars.CI_DOMAIN_ALIASES` (line 364)
  - `vars.ACM_CERTIFICATE_ARN` → `vars.CI_ACM_CERTIFICATE_ARN` (line 365)
  - **KEEP:** `secrets.GITHUB_TOKEN`, `secrets.E2E_CLERK_TEST_*`, `secrets.GITLEAKS_LICENSE`.
- [ ] **Step 2: Fix now-stale comments** — the env-mapping comments (≈ lines 95-98, 171-174) say the secret is "shared with the CD pipeline / the Path A name". After the split CI uses its own `CI_`-prefixed copy; update those phrases minimally to say so. Surgical: only the sentences the rename falsifies.
- [ ] **Step 3: Validate** `ci.yml` YAML + confirm no remaining bare `secrets.CLERK`/`secrets.AWS_ROLE_ARN`/`secrets.TF_VAR_TERRAFORM_STATE_BUCKET`/`vars.AWS_REGION`/`vars.TF_VAR_TERRAFORM_STATE_PREFIX`/`vars.DOMAIN_ALIASES`/`vars.ACM_CERTIFICATE_ARN` references remain:
  Run: `grep -nE 'secrets\.(CLERK_|AWS_ROLE_ARN|TF_VAR_TERRAFORM_STATE_BUCKET)|vars\.(AWS_REGION|TF_VAR_TERRAFORM_STATE_PREFIX|DOMAIN_ALIASES|ACM_CERTIFICATE_ARN)' .github/workflows/ci.yml`
  Expected: only `CI_`-prefixed matches.
- [ ] **Step 4: Commit.** `git commit -m "refactor(ci): read credentials from CI_-prefixed repo secrets/vars (#241)"`

### Task 4: Document in `infra/CLAUDE.md`

**Files:** Modify `infra/CLAUDE.md`

- [ ] **Step 1:** Add a short subsection under the CI/CD section: the `acc` Environment holds the clean deploy set; CI reads `CI_`-prefixed repo secrets/vars (modelled separately — values coincide today by accident); the deploy job's `environment:` gives native Deployment records; the post-deploy verification step gates the deployment status; the OIDC trust policy must allow the `environment:acc` sub.
- [ ] **Step 2: Commit.** `git commit -m "docs(infra): document acc Environment + CI_ split + verification gate (#241)"`

---

## Gates before PR (Change Workflow 6-8)
- **Integration verification (step 6):** Task 1 step 2 runs the real verification against live acc.
- **Code review (step 7):** `requesting-code-review` over the diff.
- **Security (step 8):** `security-review-final` (touches workflows + secrets handling).

## Post-merge cleanup (follow-up, not this PR)
Delete the now-orphaned bare repo-level secrets/vars (`AWS_ROLE_ARN`, `CLERK_*`, `TF_VAR_TERRAFORM_STATE_BUCKET`, `AWS_REGION`, `TF_VAR_TERRAFORM_STATE_PREFIX`, `DOMAIN_ALIASES`, `ACM_CERTIFICATE_ARN`) once CD + CI are green on the new scheme.

## Self-review
- Spec coverage: acc env ✓ (T2), native records ✓ (T2 via environment:), verification gate ✓ (T1+T2), CI_ split ✓ (T3), docs ✓ (T4). All 5 acceptance criteria mapped.
- The OIDC-sub prerequisite is surfaced (not code) — flagged as a hard pre-merge blocker.

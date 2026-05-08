# Custom Domain Setup

How to attach a custom domain (e.g. `reign.acc.steenman.me`) to the frontend CloudFront distribution.

## Prerequisites

- Domain registered at TransIP (or any DNS host that supports CNAME).
- Access to the `accounts/reign-game` Terraform project (separate repo) — that's where the ACM cert lives, because cross-account IAM/cert resources are managed there, not in this repo.
- Access to the GitHub repo's Settings → Secrets and variables → Actions.
- Access to the Clerk dashboard for whichever Clerk environment this deployment uses.

## Why two repos

The cert lives in `accounts/reign-game` (us-east-1, account-level resource) and gets referenced by ARN here. Splitting the cert out of this repo's state keeps the per-account Terraform self-contained and lets us point any number of CloudFront distributions at the same cert later.

## Order of operations

1. **Provision the ACM cert** in the other project (`accounts/reign-game`). It must be in `us-east-1` — CloudFront only reads ACM from there. DNS validation. The apply outputs the validation CNAME record AND the cert ARN.

2. **Create the validation CNAME at TransIP.** ACM gives a record like `_<random>.reign.acc.steenman.me CNAME _<random>.acm-validations.aws.`. TransIP DNS panel → add CNAME, TTL 300. Drop the trailing dot off the value if TransIP rejects it (some panels add it automatically).

3. **Wait for ACM to issue.** Status `PENDING_VALIDATION` → `ISSUED`. Usually 5–30 min. Check via AWS Console (Certificate Manager, us-east-1) or `aws acm describe-certificate --certificate-arn <arn> --region us-east-1`. **Do not proceed until ISSUED** — the CD apply will fail otherwise.

4. **Set GitHub repo variables** (Settings → Secrets and variables → Actions → Variables tab — *not* Secrets; these are not sensitive):
   - `ACM_CERTIFICATE_ARN` — full ARN from step 1, e.g. `arn:aws:acm:us-east-1:236320489345:certificate/...`
   - `DOMAIN_ALIASES` — JSON list, e.g. `["reign.acc.steenman.me"]`

5. **Trigger CD.** Push any commit to `main` (or re-run the latest CD workflow). The apply now adds the alias and switches `viewer_certificate` to ACM. Distribution deploys take ~5–15 minutes globally.

6. **Create the user-facing CNAME at TransIP:** `reign.acc.steenman.me CNAME dXXX.cloudfront.net.` The CloudFront target is the value of `terraform output cloudfront_url` (or the existing `*.cloudfront.net` you've been using). TTL 300 while testing, raise to 3600 once stable.

7. **Add the origin to Clerk.** Dashboard → relevant environment → Allowed Origins → add `https://reign.acc.steenman.me`. Without this, sign-in throws a CORS / origin-not-allowed error in the browser console.

8. **Verify end-to-end.** Open `https://reign.acc.steenman.me` in a browser:
   - Padlock is green (cert matches).
   - The landing page renders.
   - DevTools → Network: `/api/health` returns 200.
   - Sign-in flow works (if Clerk is configured).
   - Per devops-engineer.md lesson 7: "Open the deployed URL after CD succeeds and verify the user-visible feature actually works." Don't trust CI-green alone.

## Rollback

To revert to the bare CloudFront URL: clear `DOMAIN_ALIASES` (set to `[]`) and `ACM_CERTIFICATE_ARN` (empty string) in the GitHub repo variables, then re-run CD. The conditional in `infra/modules/frontend/main.tf` falls back to the default `*.cloudfront.net` cert. The cert in the other project can stay (it's cheap and re-attachable later).

## Adding a second alias later

To add `reign.steenman.me` for prod alongside the acceptance domain:

1. In `accounts/reign-game`, either request a separate cert OR add the new SAN to the existing cert (re-validate the new SAN via DNS).
2. Update `DOMAIN_ALIASES` to `["reign.acc.steenman.me", "reign.steenman.me"]`.
3. Update `ACM_CERTIFICATE_ARN` if the cert ARN changed.
4. Re-run CD; add the second TransIP CNAME; add the second origin to Clerk.

The same single distribution serves both — CloudFront routes by Host header.

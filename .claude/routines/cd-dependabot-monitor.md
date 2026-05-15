You are the Reign CD + Dependabot monitor. Run silently if all is healthy. Stop after one pass.

Repo: lesteenman/reign-game

## Authentication

Use `curl` with the token inlined in each request. The fine-grained PAT is scoped to lesteenman/reign-game with Actions:read, Metadata:read, Dependabot alerts:read, Issues:write. Do NOT use the `gh` CLI — it is not available in this environment.

The actual PAT is embedded in the routine prompt configured in the Claude Code web interface (not stored here — secrets must not be committed to the repo). Replace `<TOKEN>` in every curl command below with the PAT from the routine prompt.

All GitHub API calls use:
```
curl -s \
  -H "Authorization: token <TOKEN>" \
  -H "Accept: application/vnd.github+json" \
  "<URL>"
```

## Step 1 — Recent CD failures on `main` (last 24h)

Run:
```bash
curl -s \
  -H "Authorization: token <TOKEN>" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/lesteenman/reign-game/actions/workflows/cd.yml/runs?branch=main&per_page=20" \
| python3 -c "
import sys, json
from datetime import datetime, timezone, timedelta
data = json.load(sys.stdin)
runs = data.get('workflow_runs', [])
cutoff = datetime.now(timezone.utc) - timedelta(hours=24)
bad = []
for r in runs:
    created = datetime.fromisoformat(r['created_at'].replace('Z','+00:00'))
    if created >= cutoff and r['conclusion'] in ['failure','cancelled','timed_out','startup_failure','action_required']:
        bad.append({
            'id': r['id'],
            'sha': r['head_sha'][:7],
            'title': r['display_title'],
            'url': r['html_url'],
            'conclusion': r['conclusion'],
        })
print(json.dumps(bad))
"
```

Filter for `conclusion` in `[failure, cancelled, timed_out, startup_failure, action_required]` AND `created_at` within the last 24 hours.

For each failure, capture: `id`, short `sha` (first 7 chars), `title`, `url`.

For the FIRST failure ONLY, also fetch failed logs:
```bash
curl -s \
  -H "Authorization: token <TOKEN>" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/lesteenman/reign-game/actions/runs/<id>/jobs" \
| python3 -c "
import sys, json
data = json.load(sys.stdin)
for job in data.get('jobs', []):
    if job.get('conclusion') in ['failure','cancelled']:
        print(f'Job: {job[\"name\"]} — {job[\"conclusion\"]}')
        for step in job.get('steps', []):
            if step.get('conclusion') in ['failure','cancelled']:
                print(f'  Step: {step[\"name\"]} — {step[\"conclusion\"]}')
"
```

Note: The GitHub API does not return raw log text via JSON — the jobs endpoint gives job/step names and conclusions. That is sufficient for the issue body. Don't attempt to download the zip log archive.

## Step 2 — Open Dependabot alerts (critical or high severity)

Run:
```bash
curl -s \
  -H "Authorization: token <TOKEN>" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/lesteenman/reign-game/dependabot/alerts?state=open" \
| python3 -c "
import sys, json
data = json.load(sys.stdin)
if isinstance(data, dict) and data.get('message'):
    print(json.dumps({'error': data['message']}))
else:
    alerts = [
        {
            'number': a['number'],
            'severity': a['security_advisory']['severity'],
            'package': a['dependency']['package']['name'],
            'summary': a['security_advisory']['summary'],
            'url': a['html_url']
        }
        for a in data
        if a['security_advisory']['severity'] in ('critical', 'high')
    ]
    print(json.dumps(alerts))
"
```

If the response contains `{"error": "..."}`: the PAT doesn't have Dependabot scope. Skip Step 2 silently and note the scope gap in the issue body if you reach Step 4.

## Step 3 — Decide whether to alert

If BOTH lists are empty: print `"all green at $(date -u +%FT%TZ)"` to your own log and exit silently. Do NOT create any GitHub issue or take any other action.

If ANY list is non-empty: proceed to Step 4.

## Step 4 — Open a GitHub issue with the findings

Use the `mcp__github__issue_write` MCP tool (NOT `gh issue create` — `gh` is not available):

```
method: create
owner: lesteenman
repo: reign-game
title: "[Monitor] <YYYY-MM-DD HH:MM UTC>: <N> CD failure(s) + <M> Dependabot alert(s)"
  (Omit either half if count is 0. E.g.: "[Monitor] 2026-05-16 07:00 UTC: 1 CD failure")
labels: ["area:devops", "type:bug", "priority:p0", "status:blocks-prod"]
body: (markdown — see template below)
```

Body template:
```markdown
## CD failures (last 24h)

- **<short-sha>** — <title> (`<conclusion>`)
  <url>

  Failed jobs/steps:
  - Job: <job name> → Step: <step name>

(repeat per failure; only include failed-job detail for the first run)

## Dependabot alerts (critical + high)

- **<severity>** — <package>: <summary>
  <url>

## Notes

Surfaced by the **Reign CD + Dependabot monitor** routine. Close this issue once the underlying problem is fixed. If the Dependabot alerts API returned a permission error, this report only covers CD.
```

After successful creation, print the issue URL.

## Constraints

- Do NOT retry any API call more than once.
- Do NOT comment on existing issues — always create a fresh issue.
- Do NOT modify CI/CD config, code, or any other state.
- Do NOT close any issues.
- Stop after one pass. No follow-up actions.

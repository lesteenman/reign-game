# Automated GitLab Code Review Workflow

This guide describes how to set up automated code reviews on GitLab merge requests using Claude Code
and the `gitlab-code-review` skill.

## Overview

The automated workflow triggers a Claude Code review whenever a merge request is created or updated.
A review agent posts findings as MR comments. A separate agent (the implementer) picks up those
comments and acts on them.

```
┌──────────────┐    ┌─────────────────┐    ┌──────────────────┐    ┌────────────────┐
│  MR Created  │───▶│  Review Agent   │───▶│  MR Comment      │───▶│  Impl Agent    │
│  or Updated  │    │  (Claude Code)  │    │  Posted          │    │  Picks Up &    │
│              │    │                 │    │                  │    │  Fixes         │
└──────────────┘    └─────────────────┘    └──────────────────┘    └────────────────┘
```

## Option 1: GitLab CI/CD Pipeline

Add a code review job to your `.gitlab-ci.yml` that runs Claude Code on MR events.

### Prerequisites

- Claude Code CLI (`claude`) installed on the runner
- `ANTHROPIC_API_KEY` set as a CI/CD variable (masked)
- `GITLAB_TOKEN` set as a CI/CD variable (masked) — for `glab` authentication
- Runner with network access to gitlab.com and the Anthropic API

### .gitlab-ci.yml

```yaml
stages:
  - review

code-review:
  stage: review
  image: node:22  # or your preferred base image with claude CLI installed
  rules:
    # Only run on merge request events
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  variables:
    GITLAB_TOKEN: $GITLAB_TOKEN
    ANTHROPIC_API_KEY: $ANTHROPIC_API_KEY
  before_script:
    # Install claude CLI (adjust to your installation method)
    - npm install -g @anthropic-ai/claude-code
    # Install glab CLI
    - curl -sL "https://gitlab.com/gitlab-org/cli/-/releases/permalink/latest/downloads/glab_$(uname -s)_$(uname -m).tar.gz" | tar xz
    - mv bin/glab /usr/local/bin/
    # Authenticate glab
    - echo "$GITLAB_TOKEN" | glab auth login --hostname gitlab.com --stdin
  script:
    - |
      claude -p "Review merge request !${CI_MERGE_REQUEST_IID} using the gitlab-code-review skill. \
        Post your findings as a comment on the MR." \
        --allowedTools 'Bash(glab *)' 'Read' 'Glob' 'Grep' 'Agent'
  allow_failure: true  # Don't block the pipeline on review failures
  timeout: 10m
```

### Notes on CI setup

- `allow_failure: true` ensures the review doesn't block merging. Remove this if you want reviews to
  be mandatory.
- Set `timeout` to avoid runaway costs. 10 minutes is generous for most MRs.
- The `rules` section ensures this only runs on MR pipelines, not branch pushes.
- If your project uses a custom runner, ensure it has outbound HTTPS access.

---

## Option 2: GitLab Webhook + Server

For more control (e.g., filtering by labels, running on specific events), set up a webhook listener.

### Architecture

```
GitLab Webhook ──▶ Your Server ──▶ claude -p "Review MR !<iid> ..."
  (MR events)      (Express/Flask)    (spawns Claude Code process)
```

### Webhook Configuration

1. Go to your project's **Settings > Webhooks**
2. URL: `https://your-server.example.com/webhook/code-review`
3. Trigger: **Merge request events**
4. Secret token: set and verify in your server

### Server Pseudocode

```python
@app.post("/webhook/code-review")
def handle_mr_webhook(request):
    event = request.json

    # Only review on open/update actions
    if event["object_attributes"]["action"] not in ["open", "update"]:
        return {"status": "skipped"}

    # Skip drafts
    if event["object_attributes"]["work_in_progress"]:
        return {"status": "skipped"}

    mr_iid = event["object_attributes"]["iid"]
    project_path = event["project"]["path_with_namespace"]

    # Spawn Claude Code in background
    subprocess.Popen([
        "claude", "-p",
        f"Review merge request !{mr_iid} in {project_path} using the gitlab-code-review skill. "
        f"Post your findings as a comment on the MR.",
        "--allowedTools", "Bash(glab *)", "Read", "Glob", "Grep", "Agent"
    ])

    return {"status": "queued"}
```

---

## Option 3: Scheduled Review via `glab` + Cron

For simpler setups, run a periodic check for unreviewed MRs.

### Script: `review-open-mrs.sh`

```bash
#!/bin/bash
set -euo pipefail

# List open MRs that don't have a "Code review" comment yet
MR_IIDS=$(glab mr list --state opened --output json | \
  jq -r '.[].iid')

for MR_IID in $MR_IIDS; do
  # Check if already reviewed
  HAS_REVIEW=$(glab mr view "$MR_IID" --comments --output json | \
    jq '[.notes[]? | select(.body | startswith("### Code review"))] | length')

  if [ "$HAS_REVIEW" -eq 0 ]; then
    echo "Reviewing MR !$MR_IID..."
    claude -p "Review merge request !$MR_IID using the gitlab-code-review skill. Post findings on the MR." \
      --allowedTools 'Bash(glab *)' 'Read' 'Glob' 'Grep' 'Agent'
  fi
done
```

### Cron Entry

```cron
# Run every 15 minutes during work hours (Mon-Fri, 8-18)
*/15 8-18 * * 1-5  cd /path/to/repo && ./review-open-mrs.sh
```

---

## The Implementation Agent (Picking Up Review Comments)

After the review agent posts findings, a separate agent reads the comments and acts on them. This
can be triggered automatically or manually.

### Automatic Pickup (CI Job)

Add a second CI job that triggers after the review job, or on MR note events:

```yaml
fix-review-findings:
  stage: review
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  needs: ["code-review"]
  script:
    - |
      # Get the latest review comment
      REVIEW_COMMENT=$(glab mr view "$CI_MERGE_REQUEST_IID" --comments --output json | \
        jq -r '[.notes[] | select(.body | startswith("### Code review"))] | last | .body')

      if [ -z "$REVIEW_COMMENT" ] || echo "$REVIEW_COMMENT" | grep -q "No issues found"; then
        echo "No review findings to address."
        exit 0
      fi

      claude -p "The code review agent left these findings on MR !${CI_MERGE_REQUEST_IID}:

      $REVIEW_COMMENT

      Read the review, verify each issue against the codebase, and fix the real issues. \
      Push a new commit to the MR branch with the fixes." \
        --allowedTools 'Bash(glab *)' 'Bash(git *)' 'Read' 'Edit' 'Glob' 'Grep' 'Agent'
  allow_failure: true
  timeout: 15m
```

### Manual Pickup

From your terminal on the MR branch:

```bash
# Get review comments
glab mr view <MR-ID> --comments

# Ask Claude to address them
claude -p "Read the code review comments on MR !<MR-ID> and fix the issues found."
```

---

## Security Considerations

- **API keys**: Store `ANTHROPIC_API_KEY` and `GITLAB_TOKEN` as masked CI/CD variables. Never commit
  them to the repo.
- **Token permissions**: The GitLab token needs `api` scope for reading MRs and posting comments.
  Use a project access token (not a personal token) for CI.
- **Review scope**: The review agent only reads code and posts comments. It does not push code, merge
  MRs, or modify pipelines.
- **Cost control**: Set timeouts on CI jobs. Consider limiting reviews to non-draft MRs and skipping
  trivial changes (dependency bumps, generated files).

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `glab` auth fails in CI | Verify `GITLAB_TOKEN` is set and has `api` scope |
| Review takes too long | Reduce the number of parallel agents or add `--max-turns` |
| Too many false positives | Raise the confidence threshold from 80 to 90 |
| Comments not appearing | Check the token has permission to post notes on the project |
| Duplicate reviews | The skill checks for existing review comments before posting |

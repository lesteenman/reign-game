# Dev container

Future Claude Code sessions and IDE work happen inside this container. The host
stays clean; a compromised dependency is contained to the container + named
volumes, not the host filesystem.

## What's inside

- Claude Code (via the official Anthropic devcontainer feature)
- Go 1.26.3, Node 24, Terraform — versions pinned to match
  `.github/workflows/ci.yml`
- `task` v3.50.0, `golangci-lint` v2.11.4, `gitleaks` 8.30.1,
  `govulncheck` v1.3.0
- Docker CLI with the host socket mounted

The `.devcontainer/docker-compose.yml` extends the root `docker-compose.yml` —
LocalStack starts automatically alongside the dev container. No duplicate
LocalStack instance.

## Opening it

### IntelliJ IDEA / JetBrains Gateway

1. Gateway → "Dev Containers" → "New Dev Container from VCS / Local".
2. Point it at this repo. Gateway reads `.devcontainer/devcontainer.json`.
3. First build takes a few minutes (image pull + features + `post-create.sh`).
4. When it's ready, open a project terminal — you're inside the container.

### devcontainers CLI (host shell)

```bash
~/.devcontainers/bin/devcontainer up --workspace-folder .
~/.devcontainers/bin/devcontainer exec --workspace-folder . bash
```

## Running Claude in the dev container

The day-to-day flow once the container exists:

```bash
# 1. From the host, in the repo root, make sure the container is running.
~/.devcontainers/bin/devcontainer up --workspace-folder .

# 2. Get a shell inside it.
~/.devcontainers/bin/devcontainer exec --workspace-folder . bash

# 3. Inside the container — start Claude.
claude
```

What you should see:

- The first time, `claude` prints a browser auth URL. Open it on the host
  (`cmd+click` works in most terminals), approve, paste the code back. The
  token is written into `/home/vscode/.claude/` which lives in the
  per-project named volume `claude-code-config-<devcontainerId>`.
- On every subsequent run — host reboots, container restarts, image rebuilds
  for feature bumps — `claude` starts already authenticated. Sessions and
  settings persist in the same named volume.

The first time you log in, Claude may ask for trust permissions on the
workspace; the workspace path inside the container is
`/workspaces/reign-game`.

To remove the saved auth (e.g., switching accounts):

```bash
docker volume rm reign-game_claude-code-config-<devcontainerId>
```

(Find the exact name with `docker volume ls | grep claude-code-config`.)

### Doing this from IntelliJ Gateway

When Gateway opens the project, it gives you an integrated terminal that's
already inside the container. Step 1 and 2 above happen automatically. Just
run `claude` in that terminal.

## MCP servers

Three MCP servers are wired up via `.mcp.json` at the repo root and are
loaded by Claude Code automatically when it starts in this project:

| Server | Provides | Auth |
|---|---|---|
| `terraform` | Terraform Registry lookups (providers, modules, resources) — pulls `hashicorp/terraform-mcp-server` | None for registry browsing |
| `aws` | AWS API access via the [Agent Toolkit's hosted MCP](https://docs.aws.amazon.com/agent-toolkit/) — `uvx mcp-proxy-for-aws@latest` | Local AWS SDK creds (SSO) |
| `github` | GitHub API (issues, PRs, comments, contents) — pulls `ghcr.io/github/github-mcp-server` | Personal access token |

### One-time setup on the host

Set up these two host-level prerequisites once, before launching the dev
container. The dev container forwards them via `${localEnv:...}` in
`devcontainer.json`'s `containerEnv`.

**1. AWS SSO login** — required for the `aws` MCP server.

```bash
# On the host (macOS), run once per session expiry (typically every ~8 hours):
aws sso login --profile <your-profile>

# Add to your shell rc (~/.zshrc / ~/.bash_profile):
export AWS_PROFILE=<your-profile>
```

The dev container mounts `~/.aws/config` and `~/.aws/sso/cache/` read-only,
so once you've logged in on the host, the in-container SDK can use the
session. Read-only means SSO refresh fails — when the token expires, run
`aws sso login` on the host again. (The `~/.aws/credentials` static-keys
file is intentionally not mounted.)

**2. GitHub PAT** — required for the `github` MCP server.

Create a fine-grained PAT at <https://github.com/settings/personal-access-tokens>
with these repository permissions for `lesteenman/reign-game`:

- Contents: read/write
- Pull requests: read/write
- Issues: read/write
- Actions: read (so the agent can poll CI status)
- Metadata: read (mandatory)

Then export the token in your shell rc:

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=github_pat_…
```

After exporting either of these, restart the dev container so the new env
flows in:

```bash
~/.devcontainers/bin/devcontainer up --workspace-folder . --remove-existing-container
```

### How AWS MCP avoids the LocalStack creds

The dev container sets `AWS_ACCESS_KEY_ID=test` / `AWS_SECRET_ACCESS_KEY=test`
/ `AWS_ENDPOINT_URL=http://localstack:4566` so the backend code can talk to
LocalStack out of the box. Those would shadow the real SSO creds for the
AWS MCP server, so `.mcp.json` wraps the proxy in `env -u AWS_ACCESS_KEY_ID
-u AWS_SECRET_ACCESS_KEY -u AWS_ENDPOINT_URL` — the proxy starts in a clean
slate and falls through to the SSO config in `~/.aws/config`. No backend
behavior changes.

### Verifying the MCP servers

Inside the container, after `claude` is running, ask it `what MCP tools do
you have?` — you should see Terraform/AWS/GitHub tool prefixes. Or from a
separate shell:

```bash
claude mcp list
```

If a server is failing to start, `claude mcp list` shows the error.

## Skills + agents

| Source | Inside the container | Writeable? |
|---|---|---|
| Host global skills (`~/.claude/skills`) | `~/.claude/skills` (read-only bind) | No — host is protected |
| Repo-scoped skills/agents (`.claude/`) | `/workspaces/reign-game/.claude/` | Yes — committed with the repo |
| Per-project Claude state (auth token, settings, history) | `~/.claude/*` (named volume) | Yes — isolated per project |

To add a project-only skill or agent, edit `.claude/` at the repo root and
commit it. To change a host-global skill, do that on the host — the bind mount
is read-only.

## Running the app

LocalStack auto-starts via the dev-compose stack. From inside the container,
the backend reaches LocalStack at `http://localstack:4566` (set via
`AWS_ENDPOINT_URL`). The host (your Mac) reaches forwarded ports at
`http://localhost:5180` etc.

### What works inside the container

- `cd backend && go run ./cmd/api` — the same binary the Taskfile starts for
  both the API (on `:5181`) and the generator worker (no port). Generator role
  is selected via env, the way `task dev:up:generator` does it.
- `cd frontend && npm run dev` — frontend on `:5180`.
- `go test`, `npm test`, `golangci-lint run`, `gitleaks detect`,
  `terraform fmt`, etc.

### What does NOT work inside the container

`task dev:up`, `task dev:up:backend`, `task dev:up:generator`, and
`task dev:up:localstack` all invoke `docker compose ...` against the host
docker daemon (via the mounted socket). The compose CLI inside the container
emits container paths (e.g. `/workspaces/reign-game/.localstack`); host docker
rejects them with "mounts denied: path is not shared from the host". This is
the well-known path-translation limit of docker-outside-of-docker.

You don't need those tasks here anyway — the dev-compose stack already brings
LocalStack up. Just run the app processes directly with `go run` / `npm run dev`.

### Running tasks from the host

If you want `task dev:up` and friends, run them on the host shell as before.
LocalStack is already up via the dev-compose stack (project `reign-game`,
service `localstack`), so `task dev:up:localstack` will report it's running and
no-op. The other targets compile + run host-side Go/Node processes — same as
they did before the dev container existed.

## Verifying the setup

Inside the container:

```bash
go version            # go1.26.3
node --version        # v24.x
task --version        # 3.50.0
terraform -version    # 1.x
golangci-lint version # v2.11.4
gitleaks version      # 8.30.1
claude --version

curl http://localstack:4566/_localstack/health   # JSON response

cd backend  && go test -short ./...
cd frontend && npm test

ls ~/.claude/skills        # host skills, e.g. clean-code, design-grill, ...
touch ~/.claude/skills/probe   # should fail (read-only)
```

## Caveats

- **Folder name is hardcoded.** `docker-compose.yml` mounts `.` (the compose
  project directory = repo root) to `/workspaces/reign-game`. If you clone the
  repo into a different directory name, update both that target path and
  `workspaceFolder` in `devcontainer.json`.
- **`LOCALSTACK_TOKEN`.** The root compose reads it from `.env` (gitignored).
  Compose's project directory resolves to the repo root, so the existing `.env`
  works without copying.
- **Docker socket is mounted.** A compromised container can do anything to host
  docker (start/stop/inspect any container, mount any host path). This is the
  trade-off for keeping `task dev:up` and Taskfile's `docker compose exec`
  flows working unchanged. If isolation matters more than Taskfile parity later,
  remove the `/var/run/docker.sock` line from `.devcontainer/docker-compose.yml`
  and the `docker-outside-of-docker` feature from `devcontainer.json`.
- **Host `~/.claude` is NOT mounted.** Only `~/.claude/skills` is bind-mounted
  read-only. Host credentials and other Claude state stay on the host.
- **Egress is unrestricted.** A compromised dependency installed during
  `npm ci` or `go mod download` has unrestricted outbound. Anthropic's
  reference container ships an opt-in `init-firewall.sh`; consider adopting
  it if defense-in-depth matters for your threat model.

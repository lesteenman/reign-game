#!/usr/bin/env bash
set -euo pipefail

# Project tools that aren't covered by devcontainer features.
# Installed once at container create time.
#
# All versions pinned. Per CLAUDE.md lesson 8 ("verify dependency versions at
# the registry") + lesson 10 ("lockstep service config") — any time CI bumps a
# tool, bump the matching pin here.

TASK_VERSION=v3.50.0
GOLANGCI_LINT_VERSION=v2.11.4    # must match .github/workflows/ci.yml + .githooks/pre-push
GITLEAKS_VERSION=8.30.1
GOVULNCHECK_VERSION=v1.3.0
UV_VERSION=0.11.12               # used by `uvx` to launch the AWS MCP proxy
AWSCLI_VERSION=2.34.45           # `aws sso login`, `aws s3 ...`, etc.

echo "=== Installing project tooling ==="

# Named volumes mount at root-owned mount points the first time the container
# is created. Hand them to the runtime user so subsequent npm/go/claude writes
# don't need sudo and don't fail with EACCES.
#
# /home/vscode/.cache is intentionally listed even though no named volume
# targets `.cache` directly. The `playwright-cache` volume mounts to
# `.cache/ms-playwright`, and Docker creates the parent `.cache/` as root
# when materialising the mount path. Without chowning the parent, the very
# first `go install` below fails creating `~/.cache/go-build` and the whole
# script aborts — leaving the container with no `task`, `gitleaks`,
# `govulncheck`, or `aws` on PATH.
sudo chown -R vscode:vscode \
  /home/vscode/.cache \
  /home/vscode/.npm \
  /home/vscode/go \
  /home/vscode/.aws/sso/cache \
  /workspaces/reign-game/frontend/node_modules \
  || true
# /home/vscode/.claude is the Claude Code config volume; chown only the top
# level — recursing would hit the read-only skills bind underneath.
sudo chown vscode:vscode /home/vscode/.claude

# Persist ~/.claude.json across container recreates. The named volume covers
# ~/.claude/ but NOT the sibling ~/.claude.json file at $HOME, which is on
# the overlay filesystem and therefore wiped every recreate — making Claude
# Code re-run its setup wizard each time even though credentials in the
# volume persist. Symlink the file into the volume so writes flow through.
CLAUDE_HOME_CONFIG=/home/vscode/.claude.json
CLAUDE_VOL_CONFIG=/home/vscode/.claude/.claude.json
if [ ! -L "$CLAUDE_HOME_CONFIG" ]; then
  if [ -f "$CLAUDE_HOME_CONFIG" ]; then
    if [ -f "$CLAUDE_VOL_CONFIG" ]; then
      # Volume already has a config from a prior session — archive the
      # fresh $HOME stub (Claude Code creates one when it can't find the
      # symlink target).
      mv "$CLAUDE_HOME_CONFIG" "${CLAUDE_VOL_CONFIG}.recovered-$(date +%s)"
    else
      mv "$CLAUDE_HOME_CONFIG" "$CLAUDE_VOL_CONFIG"
    fi
  fi
  ln -s "$CLAUDE_VOL_CONFIG" "$CLAUDE_HOME_CONFIG"
fi

# go-task. Installed via `go install` so the version is pinned and reproducible
# (the upstream `taskfile.dev/install.sh` resolves "latest" at run time).
echo "--- Installing task ${TASK_VERSION} ---"
go install "github.com/go-task/task/v3/cmd/task@${TASK_VERSION}"

# golangci-lint. The installer script is pulled from the same tag as the binary
# version so a compromised `master` can't ship malicious install logic.
echo "--- Installing golangci-lint ${GOLANGCI_LINT_VERSION} ---"
curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCI_LINT_VERSION}/install.sh" \
  | sh -s -- -b "$(go env GOPATH)/bin" "${GOLANGCI_LINT_VERSION}"

# gitleaks (used by pre-commit + pre-push). Installed from the official release
# tarball — `go install` fails because the module declares its path as
# github.com/zricethezav/gitleaks/v8 even though releases ship from
# github.com/gitleaks/gitleaks.
echo "--- Installing gitleaks ${GITLEAKS_VERSION} ---"
ARCH=$(uname -m); case "$ARCH" in
  x86_64) GL_ARCH=x64 ;;
  aarch64|arm64) GL_ARCH=arm64 ;;
  *) echo "Unsupported arch $ARCH for gitleaks" >&2; exit 1 ;;
esac
curl -fsSL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_${GL_ARCH}.tar.gz" \
  -o /tmp/gitleaks.tgz
sudo tar -xz -C /usr/local/bin -f /tmp/gitleaks.tgz gitleaks
rm /tmp/gitleaks.tgz

# govulncheck (CI security gate)
echo "--- Installing govulncheck ${GOVULNCHECK_VERSION} ---"
go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"

# uv. Used by `uvx` to launch the AWS MCP proxy declared in .mcp.json.
# Installs to ~/.local/bin (already on PATH for the vscode user).
echo "--- Installing uv ${UV_VERSION} ---"
curl -LsSf "https://astral.sh/uv/${UV_VERSION}/install.sh" | sh

# AWS CLI v2 — `aws sso login` and ad-hoc human use. The base image doesn't
# ship `unzip`, which the official installer needs.
echo "--- Installing aws-cli ${AWSCLI_VERSION} ---"
case "$ARCH" in
  x86_64) AWS_ARCH=x86_64 ;;
  aarch64|arm64) AWS_ARCH=aarch64 ;;
esac
sudo apt-get update -qq && sudo apt-get install -y -qq --no-install-recommends unzip
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${AWS_ARCH}-${AWSCLI_VERSION}.zip" \
  -o /tmp/awscliv2.zip
( cd /tmp && unzip -q -o awscliv2.zip && sudo ./aws/install --update )
rm -rf /tmp/awscliv2.zip /tmp/aws

# Note: `awslocal` is intentionally not installed here. Every project usage is
# `docker compose exec localstack awslocal ...` — it runs inside the LocalStack
# container (which has it pre-installed), not on the dev container's PATH.

# Project hooks (CLAUDE.md: "After cloning the repo, configure git to run the project's hooks")
echo "--- Configuring git hooks path ---"
git config core.hooksPath .githooks

# Dependency download — pre-warm caches so first build is fast.
echo "--- Downloading Go modules ---"
( cd backend && go mod download )

echo "--- Installing frontend dependencies ---"
# `npm ci` clears node_modules content before installing, so no manual rm.
# The container-local `frontend-node-modules` volume keeps Linux native
# bindings separate from the host's darwin bindings.
( cd frontend && npm ci )

# Playwright browser + system libs. `install-deps` apt-installs the shared
# objects chromium needs (libnss3, libatk, etc.); `install chromium` drops
# the matching browser binary into ~/.cache/ms-playwright (mounted as a named
# volume so it survives container recreates — see docker-compose.yml).
# Browser version is pinned by the @playwright/test version in
# frontend/package.json. Chromium-only — playwright.config.ts uses only chromium.
#
# sudo strips PATH (secure_path in sudoers) so `npx` / `node` aren't found.
# `sudo env "PATH=$PATH" ...` forwards the vscode user's PATH so the playwright
# binary can locate `node` via its `#!/usr/bin/env node` shebang.
echo "--- Installing Playwright chromium + system deps ---"
( cd frontend && sudo env "PATH=$PATH" ./node_modules/.bin/playwright install-deps chromium )
( cd frontend && ./node_modules/.bin/playwright install chromium )

echo "=== Setup complete ==="

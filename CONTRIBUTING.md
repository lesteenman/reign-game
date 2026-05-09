# Contributing to Reign

## Prerequisites

You can either install the toolchain on your host (instructions below) **or**
use the dev container, which ships everything pre-pinned to match CI. See
`.devcontainer/README.md` for the dev container path. The two paths are
maintained in parallel.

### Required (host install)

- **Go 1.26+** — [install](https://go.dev/dl/)
- **Node.js 24+** — [install](https://nodejs.org/)
- **Terraform 1.5+** — [install](https://developer.hashicorp.com/terraform/install)
- **Docker** — for LocalStack (local DynamoDB)
- **golangci-lint** — `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4` (v2 module path; CI is pinned to v2.11.4 in `.github/workflows/ci.yml`)
- **gitleaks** — secret scanning, required before every commit. [install](https://github.com/gitleaks/gitleaks#installing)

### Claude Code Setup

This project uses [Claude Code](https://claude.ai/code) with custom agents and skills for development workflows. If you use Claude Code:

1. **Install Claude Code** — follow the [official docs](https://docs.anthropic.com/en/docs/claude-code)

2. **Install required plugins** — these skills are used by the frontend-dev, ui-ux-designer, and design-flow agents:

   ```bash
   # Frontend design guidance
   claude plugins install frontend-design@claude-plugins-official

   # UI/UX patterns and design system generation
   claude plugins install ui-ux-pro-max@ui-ux-pro-max-skill
   ```

   The marketplace for `ui-ux-pro-max` is hosted at `github:nextlevelbuilder/ui-ux-pro-max-skill`. If the install prompts for a marketplace source, use that.

3. **Verify setup** — the project-local settings (`.claude/settings.json`) and all agent definitions (`.claude/agents/`) are committed to the repo and will be picked up automatically.

Without these plugins, agents that reference `skills/frontend-design/SKILL.md` or `skills/ui-ux-pro-max/SKILL.md` will fail during visual design phases. Non-visual work (backend, infrastructure, tests) is unaffected.

## Development Workflow

### Branching

All work happens on feature branches. Never commit directly to `main`.

```bash
git checkout -b feature/your-feature-name
```

### Commit Conventions

- Commit after every artifact delivery (specs, wireframes, code)
- Run `gitleaks detect --source .` before every commit
- Keep commits focused — one logical change per commit

### Testing

TDD is mandatory for both frontend and backend. Write a failing test first, then make it pass.

```bash
# Backend
cd backend && go test ./... -v

# Frontend
cd frontend && npm run test

# E2E
cd frontend && npx playwright test

# Lint
cd backend && golangci-lint run
```

### Security

Before every commit:

```bash
gitleaks detect --source .
```

Never commit API keys, tokens, passwords, or credentials. Use environment variables or AWS SSM Parameter Store for secrets.

### Pull Requests

- Create a PR against `main`
- PRs trigger CI (lint + test for both frontend and backend)
- Include a brief description of what changed and why
- If the change involved design decisions, add a "Key Decisions" section explaining intentional choices

## Project Layout

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for the full directory layout and API endpoints.

## Domain Language

See [GLOSSARY.md](GLOSSARY.md) for shared vocabulary. Use these terms consistently in code, specs, and discussions. If you introduce a new domain concept, add it to the glossary.

## License

Core engine and app shell: MIT (see LICENSE when added).
Curated puzzle database and premium features: proprietary.

# SHAI (Shell AI)

SHAI is a lightweight terminal copilot that converts natural language queries into shell commands, validates them through safety guardrails, and optionally executes them after confirmation. The project is implemented in Go and follows Clean Architecture plus SOLID principles, separating domain entities, application services, and infrastructure adapters.

## Key Features (Phase 1)

- Natural language CLI entry point (`shai query "describe task"`) and shell `#` prefix integration scaffolding.
- Multi-provider AI abstraction with initial Claude (Anthropic) HTTP client and offline heuristic fallback.
- Context collector that gathers working directory, files, installed tools, git/k8s metadata, and env snippets.
- Config loader for `~/.shai/config.yaml`, environment overrides, and default bootstrap content.
- Guardrail engine with regex-based danger patterns, risk levels, and confirmation policies.
- Local command executor with clipboard copy, preview-only, and confirmation prompts.
- Persistent history log and filesystem-backed response cache for faster repeated queries.
- YAML guardrail schema supports protected paths, confirmation messaging tiers, and command whitelist.
- History persists to SQLite (`~/.shai/history/history.db`) with CLI tooling to list/search/export, while cache adds TTL+LRU eviction.

## Architecture Overview

Layer | Responsibility | Packages
----- | -------------- | --------
Domain | Pure entities (config, context, models, guardrails) | `internal/domain`
Application | Use cases orchestrating ports | `internal/application/query`
Ports | Interfaces between layers | `internal/ports`
Infrastructure | Adapters for config, AI providers, context, security, executor, CLI | `internal/infrastructure/*`
App wiring | Dependency container | `internal/app`
CLI | Cobra-based entry point | `cmd/shai`, `internal/infrastructure/cli`

Each dependency points inward (infrastructure depends on ports/domain, never the opposite) to keep substitutions easy.

## Getting Started

```bash
git clone https://github.com/doeshing/shai-go.git
cd shai-go
go run ./cmd/shai --help

# Example: preview a command
go run ./cmd/shai query "list git status" --preview-only
```

Once satisfied, build an executable:

```bash
go build -o shai ./cmd/shai
./shai query "check docker containers"
```

Set `SHAI_DEBUG=true` for verbose logging. Provide `ANTHROPIC_API_KEY` to enable real Claude calls; otherwise SHAI falls back to heuristic suggestions.

## Configuration

On first run SHAI writes `~/.shai/config.yaml` containing:

- `preferences`: default model, preview/auto-execute toggles, timeout.
- `models`: definitions for Claude/OpenAI/Ollama endpoints (name, endpoint, env vars, prompts).
- `context`: file listing limits, git/k8s/env inclusion modes.
- `security`: guardrail toggle plus `rules_file` (defaults to `~/.shai/guardrail.yaml`).
- `execution`: shell selection and confirmation behavior.

You can override the config path via `SHAI_CONFIG=/custom/path.yaml`.

Guardrail rules follow the YAML schema described in the project specification. Missing files are auto-populated with safe defaults (rm -rf, dd, curl | sudo, etc.).

## CLI Usage

```
shai query "list docker containers"
  --model claude-sonnet-4
  --preview-only
  --auto-execute
  --copy
  --with-git-status
  --with-env
  --with-k8s-info
  --debug

shai install [--shell zsh]
shai uninstall [--shell bash]
shai config show
shai config set preferences.default_model gpt-4
shai config edit
shai config validate
shai config get --key preferences.default_model
shai config reset
shai doctor
shai history list --limit 10
shai history search --query docker
shai history export history.jsonl
shai cache list
shai cache size
shai cache clear
```

Plain `shai "..."` proxies to the `query` command.

Confirmation tiers map to guardrail actions:

- `allow` -> auto execution (respecting `--auto-execute` or config).
- `preview_only` -> show command only.
- `simple_confirm` / `confirm` -> prompt `[y/N]`.
- `explicit_confirm` -> require typing `yes`.
- `block` -> refuse to run.

## Shell Integration Roadmap

The repo ships integration scripts under `assets/shell/` (placeholder today). Upcoming CLI commands:

1. `shai install` – detect shell, drop scripts into `~/.shai/shell/{zsh,bash}.sh`, append sourcing line.
2. `shai uninstall` – remove hooks but keep backup scripts.
3. `shai reload` – refresh active shell sessions.

## Milestones

- **Phase 1 (MVP)**: CLI, Claude provider, context collector, basic guardrails, zsh integration scripts.
- **Phase 2**: Guardrail YAML enhancements, Ollama/OpenAI providers, bash integration, config subcommands.
- **Phase 3**: Performance optimizations, history/cache subsystems, auto-update, full documentation set.

Issues, discussions, and contributions are welcome via GitHub once the repository is published. All code is released under the MIT License.

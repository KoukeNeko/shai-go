# SHAI (Shell AI)

> A production-ready terminal copilot that safely converts natural language into shell commands.

SHAI is an AI-powered CLI assistant built with Go, implementing Hexagonal Architecture for maximum testability and
maintainability. It supports multiple AI providers, features comprehensive security guardrails, and provides intelligent
context awareness.

## Features

### 🤖 AI-Powered Command Generation

- **Multi-Provider Support**: Anthropic Claude, OpenAI, Ollama via unified HTTP provider
- **Context-Aware**: Auto-collects git status, Kubernetes info, Docker state, available tools
- **Template-Driven Prompts**: Customize prompts using Go templates
- **Smart Caching**: SHA256-based response cache with TTL and LRU eviction

### 🛡️ Security First

- **5-Tier Risk Assessment**: safe, low, medium, high, critical
- **Pattern Matching**: Blocks dangerous operations (`rm -rf /`, `dd`, fork bombs)
- **Protected Paths**: Guards `/etc`, `/usr`, `$HOME`, `.ssh`
- **Whitelist Support**: Bypass checks for read-only commands
- **Auto-Created Config**: `~/.shai/guardrail.yaml` created on first run

### ⚡ Performance & UX

- **Response Caching**: Instant results for repeated queries
- **SQLite History**: Full-text search, 90-day retention
- **Hot Reload**: Update config without restarting shell
- **Clipboard Integration**: Copy commands with `--copy` flag
- **Streaming Output**: Real-time AI reasoning with `--stream`

### 🏗️ Clean Architecture

- **Hexagonal Design**: Complete port/adapter separation
- **SOLID Principles**: Single responsibility throughout
- **100% Testable**: Business logic isolated from infrastructure
- **Uber Go Style Guide**: Idiomatic, maintainable code

---

## Quick Start

### Installation

```bash
# Clone and build
git clone https://github.com/doeshing/shai-go.git
cd shai-go
go build -o shai ./cmd/shai

# Move to PATH
sudo mv shai /usr/local/bin/

# Verify
shai version
```

### Setup

```bash
# Set API key (choose one)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
# OR use Ollama locally (no key required)

# First run creates ~/.shai/config.yaml and ~/.shai/guardrail.yaml
shai "show current directory contents"
```

### Basic Usage

```bash
# Generate and execute commands
shai "list all docker containers with memory usage"

# Preview without executing
shai "delete all .log files" --preview-only

# Auto-execute safe commands
shai "show git status" --auto-execute

# Copy to clipboard
shai "compress images" --copy

# Include additional context
shai "deploy to k8s" --with-git-status --with-k8s-info

# Use specific model
shai "complex task" --model gpt-4

# Debug mode
shai "troubleshoot" --debug
```

---

## Available Commands

| Command        | Description                                |
|----------------|--------------------------------------------|
| `shai [query]` | Generate command from natural language     |
| `shai health`  | Run system diagnostics                     |
| `shai reload`  | Reload configuration without shell restart |
| `shai version` | Show version information                   |

### Query Flags

```bash
-m, --model <name>       Override AI model
-p, --preview-only       Show command without executing
-a, --auto-execute       Execute without confirmation (respects guardrails)
-c, --copy               Copy to clipboard instead of executing
--with-git-status        Include git repository status
--with-env               Include environment variables
--with-k8s-info          Include Kubernetes context
--debug                  Enable verbose logging
--stream                 Stream AI reasoning in real-time
--timeout <duration>     Override timeout (default: 60s)
```

### Health Check Example

```bash
$ shai health
[OK] Config file - loaded 1
[OK] Guardrail - rules loaded
[OK] Context collector - detected tools: 10
[OK] Git status - branch main, modified 3
[WARN] API keys - ANTHROPIC_API_KEY missing
[OK] Guardrail file - /Users/you/.shai/guardrail.yaml
```

---

## Configuration

### File Structure

SHAI creates two configuration files on first run:

**`~/.shai/config.yaml`** - Main configuration:

```yaml
config_format_version: "1"

preferences:
  default_model: claude-sonnet-4
  auto_execute_safe: false
  preview_mode: always  # always | never
  timeout: 30

models:
  - name: claude-sonnet-4
    endpoint: https://api.anthropic.com/v1/messages
    auth_env_var: ANTHROPIC_API_KEY
    model_id: claude-3-5-sonnet-20240620
    max_tokens: 1024

context:
  max_files: 20
  include_git: auto      # auto | always | never
  include_k8s: auto
  include_env: false

security:
  enabled: true
  rules_file: ~/.shai/guardrail.yaml

execution:
  shell: auto            # auto | bash | zsh | fish
  confirm_before_execute: true

cache:
  ttl: 1h
  max_entries: 100

history:
  retention_days: 90
```

**`~/.shai/guardrail.yaml`** - Security rules (auto-created with comprehensive defaults):

```yaml
rules:
  danger_patterns:
    - pattern: 'rm\s+-rf\s+/'
      level: critical
      action: block
      message: "Attempting to delete root filesystem"

  protected_paths:
    - path: "/etc"
      operations: [ "rm", "mv" ]
      level: high
      action: explicit_confirm

  whitelist:
    - "ls"
    - "git status"
    - "docker ps"

  confirmation_levels:
    critical:
      action: block
      message: "⛔ This action is blocked by security policy."
    high:
      action: explicit_confirm
      message: "⚠️  Type 'yes' to execute this high-risk operation."
```

### Managing Configuration

```bash
# Edit config
$EDITOR ~/.shai/config.yaml

# Reload after changes (no shell restart needed)
shai reload

# Override config path
SHAI_CONFIG=/custom/path.yaml shai "..."

# Debug mode
SHAI_DEBUG=1 shai "..."
```

---

## Security Guardrails

### Default Protected Operations

| Pattern          | Risk     | Action        | Reason                   |
|------------------|----------|---------------|--------------------------|
| `rm -rf /`       | Critical | Block         | Root filesystem deletion |
| `dd if=`         | Critical | Block         | Raw disk operations      |
| `:(){ :\|:& };:` | Critical | Block         | Fork bomb                |
| `curl \| sudo`   | High     | Type "yes"    | Piping to sudo           |
| `chmod 777`      | Medium   | Confirm [y/N] | Overly permissive        |

### Risk Levels

- **Critical** → Blocked entirely
- **High** → Require typing "yes"
- **Medium** → Prompt [y/N]
- **Low** → Simple confirmation
- **Safe** → Execute immediately (with auto-execute)

### Customize Rules

Edit `~/.shai/guardrail.yaml` to add custom patterns:

```yaml
rules:
  danger_patterns:
    - pattern: 'npm\s+install.*--global'
      level: medium
      action: simple_confirm
      reason: "Installing global npm package"
```

---

## Shell Integration

SHAI includes shell integration scripts for inline command generation using the `#` prefix.

### Manual Installation

**For zsh** - Add to `~/.zshrc`:

```bash
source ~/.shai/shell/zsh.sh
```

**For bash** - Add to `~/.bashrc`:

```bash
source ~/.shai/shell/bash.sh
```

Then reload: `source ~/.zshrc` or `source ~/.bashrc`

### Usage

```bash
# Type a comment starting with # and press Enter
$ # list all docker containers with status
→ docker ps --format "table {{.Names}}\t{{.Status}}"
Execute command? [y/N]: y
```

### Custom Binary Path

```bash
# If shai is not in PATH
export SHAI_BIN="/custom/path/to/shai"
```

---

## AI Provider Configuration

### Adding Custom Providers

SHAI supports any OpenAI-compatible API via configuration:

```yaml
models:
  - name: custom-llm
    endpoint: https://api.example.com/v1/chat
    auth_env_var: CUSTOM_API_KEY
    model_id: custom-model-v1
    max_tokens: 2048
    prompt:
      - role: system
        content: |
          You are a shell command generator. Respond ONLY with the command.

          Context:
          - Directory: {{.WorkingDir}}
          - Shell: {{.Shell}}
          - OS: {{.OS}}
          - Tools: {{.AvailableTools}}
          {{- if .GitStatus}}
          - Git: {{.GitStatus}}
          {{- end}}
      - role: user
        content: "{{.Prompt}}"
```

### Template Variables

| Variable              | Description        | Example                  |
|-----------------------|--------------------|--------------------------|
| `{{.Prompt}}`         | User's query       | "list docker containers" |
| `{{.WorkingDir}}`     | Current directory  | "/home/user/project"     |
| `{{.Shell}}`          | Active shell       | "zsh"                    |
| `{{.OS}}`             | Operating system   | "darwin"                 |
| `{{.Files}}`          | File listing       | "main.go\nREADME.md"     |
| `{{.AvailableTools}}` | CLI tools          | "docker, kubectl, git"   |
| `{{.GitStatus}}`      | Git status         | "main, 3 modified"       |
| `{{.K8sContext}}`     | Kubernetes context | "production"             |
| `{{.K8sNamespace}}`   | K8s namespace      | "default"                |

---

## Architecture

SHAI implements **Hexagonal Architecture** with strict dependency inversion:

```
┌─────────────────────────┐
│     CLI (Cobra)         │  User interaction
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│  Services               │  Use case orchestration
│  • QueryService         │
│  • HealthService        │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│   Ports (Interfaces)    │  12 interface definitions
│  • Provider             │
│  • SecurityService      │
│  • ContextCollector     │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│ Infrastructure          │  Concrete implementations
│  • AI providers         │
│  • Guardrail            │
│  • SQLite store         │
│  • File cache           │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│   Domain (Entities)     │  Pure business logic
│  • Config               │
│  • RiskAssessment       │
│  • QueryRequest         │
└─────────────────────────┘
```

### Project Structure

```
shai-go/
├── cmd/shai/              # Application entry point
├── internal/
│   ├── domain/           # Pure business entities (9 files)
│   ├── ports/            # Interface definitions (12 ports)
│   ├── services/         # Use cases (3 services)
│   ├── infrastructure/   # Adapters (13 implementations)
│   ├── app/              # Dependency injection container
│   └── pkg/              # Shared utilities
└── assets/
    ├── defaults/         # Embedded default configs
    │   ├── config.yaml
    │   └── guardrail.yaml
    └── shell/            # Shell integration scripts
        ├── bash.sh
        └── zsh.sh
```

### Design Principles

- **Dependency Inversion**: Infrastructure depends on domain, never the opposite
- **Single Responsibility**: Each component has one clear purpose
- **Open/Closed**: Extend via configuration, not code modification
- **Interface Segregation**: Small, focused interfaces
- **Testability**: Services use dependency injection with interface mocks

---

## Development

### Building

```bash
# Standard build
go build -o shai ./cmd/shai

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o shai-linux ./cmd/shai
GOOS=darwin GOARCH=arm64 go build -o shai-mac ./cmd/shai
GOOS=windows GOARCH=amd64 go build -o shai.exe ./cmd/shai
```

### Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# With race detection
go test -race ./...

# Verbose
go test -v ./internal/services/...
```

### Code Quality

```bash
# Format
go fmt ./...

# Vet
go vet ./...

# Lint (requires golangci-lint)
golangci-lint run
```

---

## Project Statistics

- **Go Version**: 1.25.3
- **Total Lines**: ~5,255 lines of Go
- **Files**: 39 .go files
- **Dependencies**: 5 direct dependencies (Cobra, YAML, SQLite, UUID, Humanize)
- **Architecture**: Hexagonal (12 ports, 13 adapters)

---

## Requirements

- **Go**: 1.21+ (tested with 1.25.3)
- **AI Provider**: At least one (Anthropic, OpenAI, or Ollama)
- **Platform**: macOS, Linux, Windows

---

## FAQ

**Q: Which AI provider should I use?**
A: Anthropic Claude Sonnet 4 offers the best balance. Use Ollama for offline/local usage.

**Q: Is my API key secure?**
A: Yes. Keys are read from environment variables and never logged or cached.

**Q: Can I use SHAI offline?**
A: Yes, with Ollama locally. Cached responses also work offline.

**Q: What if a dangerous command gets through?**
A: Always review commands before execution. Report false negatives via GitHub Issues.

**Q: How do I customize prompts?**
A: Edit the `prompt` section in your model definition in `~/.shai/config.yaml`.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go)
- Inspired by GitHub Copilot CLI and Amazon CodeWhisperer

## Support

- **Issues**: [GitHub Issues](https://github.com/doeshing/shai-go/issues)
- **Discussions**: [GitHub Discussions](https://github.com/doeshing/shai-go/discussions)

---

**⚠️ Important**: SHAI is an AI-powered tool. Always review generated commands before execution, especially for
destructive operations or sensitive data.

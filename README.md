# SHAI (Shell AI)

> Production-ready terminal copilot that converts natural language into safe, executable shell commands.

SHAI is an AI-powered CLI assistant written in Go that implements clean hexagonal architecture. It features
multi-provider AI support, comprehensive security guardrails, intelligent context awareness, and seamless shell
integration.

---

## Features

### AI-Powered Command Generation

- **Multi-Provider Support**: Anthropic Claude, OpenAI, Ollama, or any OpenAI-compatible API
- **Configuration-Driven**: Add new AI providers via YAML without code changes
- **Context-Aware**: Auto-collects git status, Kubernetes info, Docker state, and available tools
- **Template-Based Prompts**: Customize prompts using Go templates with context variables
- **Streaming Support**: View real-time AI reasoning with `--stream` flag

### Security Guardrails

SHAI implements a **5-tier risk assessment system** to protect against dangerous operations:

| Risk Level   | Action                | Example Commands                          |
|--------------|-----------------------|-------------------------------------------|
| **Critical** | Block entirely        | `rm -rf /`, `dd if=/dev/zero`, fork bombs |
| **High**     | Type "yes" to confirm | `sudo curl \| bash`, `chmod -R 777`       |
| **Medium**   | Confirm [y/N]         | `chown -R`, `npm install -g`              |
| **Low**      | Simple confirmation   | Harmless operations                       |
| **Safe**     | Execute immediately   | `ls`, `git status`, `docker ps`           |

**Security Features**:

- Regex-based danger pattern detection
- Protected path rules (`/etc`, `/usr`, `$HOME`, `.ssh`)
- Whitelist for read-only commands
- Dry-run suggestions with undo hints
- Configurable rules via `~/.shai/guardrail.yaml`

### Performance & UX

- **Zero External SDKs**: All AI communication via standard HTTP client
- **Clipboard Integration**: Copy commands with `--copy` flag
- **Hot Reload**: Update configuration without restarting shell
- **Detailed Diagnostics**: `shai health` checks environment, API keys, and configuration
- **Verbose Mode**: Optional context display (directory, tools, model selection)

### Clean Architecture

- **Hexagonal Design**: Complete separation of domain, services, and infrastructure
- **SOLID Principles**: Single responsibility throughout the codebase
- **12 Interface Ports**: Full dependency inversion for 100% testability
- **13 Infrastructure Adapters**: Concrete implementations of all interfaces
- **Idiomatic Go**: Strict adherence to Uber Go Style Guide and `gofmt`

---

## Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/doeshing/shai-go.git
cd shai-go

# Build binary
go build -o shai ./cmd/shai

# Install to PATH
sudo mv shai /usr/local/bin/

# Verify installation
shai version
```

### Setup

```bash
# Set API key (choose one provider)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
# OR use Ollama locally (no key required)

# First run creates configuration files
shai "show current directory contents"
# Creates:
#   ~/.shai/config.yaml
#   ~/.shai/guardrail.yaml
```

### Basic Usage

```bash
# Generate command with confirmation prompt
shai "list all docker containers with memory usage"

# Auto-execute safe commands (skip confirmation)
shai "show git status" --auto-execute

# Copy to clipboard instead of executing
shai "compress images in current directory" --copy

# Include additional context
shai "deploy to kubernetes" --with-git-status --with-k8s-info

# Override AI model
shai "complex query" --model gpt-4

# Stream AI reasoning in real-time
shai "analyze logs" --stream

# Debug mode (verbose output)
shai "troubleshoot docker" --debug
```

---

## Shell Integration

SHAI provides inline command generation using the `#` prefix in your shell.

### Installation

```bash
# Auto-detect shell and install integration
shai install

# Or specify shell explicitly
shai install --shell zsh
shai install --shell bash
```

This command will:

1. Copy shell scripts to `~/.shai/shell/`
2. Add integration to your RC file (`~/.zshrc` or `~/.bashrc`)
3. Create timestamped backup of RC file
4. Prevent duplicate installations

Activate: `source ~/.zshrc` or restart terminal

### Usage

```bash
# Type a comment starting with # and press Enter
$ # list all docker containers with status
→ docker ps --format "table {{.Names}}\t{{.Status}}"
Execute command? [y/N]: y
```

### Uninstallation

```bash
# Remove shell integration (keeps configuration)
shai uninstall

# Complete removal including all SHAI data
shai uninstall --purge

# Without --purge:
#   - Removes integration from RC file
#   - Deletes shell scripts from ~/.shai/shell/
#   - Preserves config.yaml, guardrail.yaml
#
# With --purge:
#   - Everything above PLUS entire ~/.shai/ directory
#   - Removes all configuration, history, cache
#
# Always creates backup before modifying RC file
```

---

## CLI Commands

| Command              | Description                                       |
|----------------------|---------------------------------------------------|
| `shai [query]`       | Generate command from natural language            |
| `shai query [query]` | Alias for above                                   |
| `shai health`        | Run environment diagnostics                       |
| `shai reload`        | Reload configuration without shell restart        |
| `shai version`       | Display version information                       |
| `shai install`       | Install shell integration (auto-detects zsh/bash) |
| `shai uninstall`     | Remove shell integration                          |

### Query Flags

```bash
-m, --model <name>       Override AI model selection
-a, --auto-execute       Execute safe commands without confirmation
-c, --copy               Copy command to clipboard (skip execution)
--with-git-status        Include git repository status in context
--with-env               Include environment variables in context
--with-k8s-info          Include Kubernetes context and namespace
--debug                  Enable verbose logging
--stream                 Stream AI reasoning in real-time
--timeout <duration>     Override execution timeout (default: 60s)
```

### Health Check Example

```bash
$ shai health
[OK] Config file - loaded 2 models
[OK] Guardrail - rules loaded successfully
[OK] Context collector - detected 10 tools
[OK] Git status - branch main, 3 modified files
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
  verbose: false         # Show detailed context (directory, tools, model)
  timeout: 30
  fallback_models: [ ]

models:
  - name: claude-sonnet-4
    endpoint: https://api.anthropic.com/v1/messages
    auth_env_var: ANTHROPIC_API_KEY
    model_id: claude-3-5-sonnet-20240620
    max_tokens: 1024
    api_format:
      auth_header_name: x-api-key
      auth_header_prefix: ""
      system_message_mode: separate
      content_wrapper: anthropic
      response_json_path: content[0].text
      extra_headers:
        anthropic-version: "2023-06-01"
    prompt:
      - role: system
        content: |
          You are SHAI, a cautious shell assistant.
          Current environment:
          - Directory: {{.WorkingDir}}
          - Shell: {{.Shell}}
          - OS: {{.OS}}
          {{if .AvailableTools}}- Tools: {{.AvailableTools}}{{end}}
          {{if .GitStatus}}- Git: {{.GitStatus}}{{end}}
      - role: user
        content: "{{.Prompt}}"

context:
  include_files: true
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
```

**`~/.shai/guardrail.yaml`** - Security rules:

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

### Configuration Management

```bash
# Edit configuration
$EDITOR ~/.shai/config.yaml

# Reload after changes (no shell restart needed)
shai reload

# Override config path
SHAI_CONFIG=/custom/path.yaml shai "query"

# Debug mode
SHAI_DEBUG=1 shai "query"
```

---

## AI Provider Configuration

SHAI supports any AI provider through configuration-driven API format specification. No code changes needed.

### Anthropic Claude

```yaml
models:
  - name: claude-sonnet-4
    endpoint: https://api.anthropic.com/v1/messages
    auth_env_var: ANTHROPIC_API_KEY
    model_id: claude-3-5-sonnet-20240620
    max_tokens: 1024
    api_format:
      auth_header_name: x-api-key           # Use x-api-key instead of Authorization
      auth_header_prefix: ""                # No "Bearer " prefix
      system_message_mode: separate         # System in separate field
      content_wrapper: anthropic            # Wrap content in type/text structure
      response_json_path: content[0].text   # Extract from content array
      extra_headers:
        anthropic-version: "2023-06-01"     # Required API version
    prompt:
      - role: system
        content: "You are a shell command generator..."
      - role: user
        content: "{{.Prompt}}"
```

### OpenAI-Compatible Providers

For OpenAI, Ollama, and most compatible APIs (default format):

```yaml
models:
  - name: gpt-4
    endpoint: https://api.openai.com/v1/chat/completions
    auth_env_var: OPENAI_API_KEY
    model_id: gpt-4-turbo
    max_tokens: 2048
    # api_format is optional - defaults to OpenAI format
    prompt:
      - role: system
        content: "You are a shell command generator..."
      - role: user
        content: "{{.Prompt}}"
```

### Ollama (Local)

```yaml
models:
  - name: codellama
    endpoint: http://localhost:11434/v1/chat/completions
    model_id: codellama:7b
    max_tokens: 512
    # No auth_env_var needed for local Ollama
    prompt:
      - role: system
        content: "Convert user requests into shell commands."
      - role: user
        content: "{{.Prompt}}"
```

### Custom Provider

```yaml
models:
  - name: custom-llm
    endpoint: https://api.example.com/v1/chat
    auth_env_var: CUSTOM_API_KEY
    model_id: custom-model-v1
    max_tokens: 2048
    api_format:
      auth_header_name: X-Custom-Auth       # Custom auth header
      auth_header_prefix: "Token "          # Custom prefix
      system_message_mode: inline           # inline | separate
      content_wrapper: openai               # openai | anthropic
      response_json_path: data.message      # Custom JSON path
      extra_headers:
        X-Custom-Version: "1.0"
    prompt:
      - role: system
        content: "System prompt..."
      - role: user
        content: "{{.Prompt}}"
```

### API Format Options

| Field                 | Description                   | Default                      | Examples                     |
|-----------------------|-------------------------------|------------------------------|------------------------------|
| `auth_header_name`    | HTTP header for API key       | `Authorization`              | `x-api-key`, `X-Custom-Auth` |
| `auth_header_prefix`  | Prefix before API key value   | `Bearer `                    | `""`, `Token `, `ApiKey `    |
| `system_message_mode` | How to send system messages   | `inline`                     | `inline`, `separate`         |
| `content_wrapper`     | Message content format        | `openai`                     | `openai`, `anthropic`        |
| `response_json_path`  | JSON path to extract response | `choices[0].message.content` | `content[0].text`            |
| `extra_headers`       | Additional HTTP headers (map) | `{}`                         | Version headers, metadata    |

### System Message Modes

**`inline`** (OpenAI format): System messages included in `messages` array

```json
{
  "messages": [
    {
      "role": "system",
      "content": "..."
    },
    {
      "role": "user",
      "content": "..."
    }
  ]
}
```

**`separate`** (Anthropic format): System message in dedicated field

```json
{
  "system": "...",
  "messages": [
    {
      "role": "user",
      "content": "..."
    }
  ]
}
```

### Content Wrapper Formats

**`openai`**: Direct string content

```json
{
  "role": "user",
  "content": "list files"
}
```

**`anthropic`**: Wrapped in type/text structure

```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "list files"
    }
  ]
}
```

### Template Variables

| Variable              | Description                         | Example                  |
|-----------------------|-------------------------------------|--------------------------|
| `{{.Prompt}}`         | User's natural language query       | "list docker containers" |
| `{{.WorkingDir}}`     | Current directory path              | "/home/user/project"     |
| `{{.Shell}}`          | Active shell                        | "zsh"                    |
| `{{.OS}}`             | Operating system                    | "darwin"                 |
| `{{.Files}}`          | File listing from current directory | "main.go\nREADME.md"     |
| `{{.AvailableTools}}` | Detected CLI tools                  | "docker, kubectl, git"   |
| `{{.GitStatus}}`      | Git repository status               | "main, 3 modified"       |
| `{{.K8sContext}}`     | Kubernetes context                  | "production"             |
| `{{.K8sNamespace}}`   | Kubernetes namespace                | "default"                |

---

## Architecture

SHAI implements **Hexagonal Architecture** (Ports and Adapters) with strict dependency inversion:

```
┌─────────────────────────┐
│     CLI Layer           │  User interaction (Cobra)
│   • Root command        │
│   • Query/Health/Install│
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│  Services Layer         │  Use case orchestration
│   • QueryService        │
│   • HealthService       │
│   • ConfigService       │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│   Ports (Interfaces)    │  12 dependency inversion points
│   • Provider            │
│   • SecurityService     │
│   • ContextCollector    │
│   • ConfigProvider      │
│   • CommandExecutor     │
│   • Clipboard           │
│   • Logger              │
│   • ShellIntegrator     │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│ Infrastructure Layer    │  13 concrete implementations
│   • AI HTTP provider    │
│   • Guardrail           │
│   • File config loader  │
│   • Context collector   │
│   • Local executor      │
│   • CLI adapters        │
└───────────┬─────────────┘
            │
┌───────────▼─────────────┐
│   Domain Layer          │  Pure business entities
│   • Config              │
│   • RiskAssessment      │
│   • ContextSnapshot     │
│   • ModelDefinition     │
│   • QueryRequest        │
└─────────────────────────┘
```

### Project Structure

```
shai-go/
├── cmd/shai/              # Application entry point
│   └── main.go            # Main function with context setup
│
├── internal/
│   ├── domain/            # Pure business entities (9 files, ~400 LOC)
│   │   ├── config.go      # Config, Preferences, ModelDefinition
│   │   ├── security.go    # RiskAssessment, GuardrailRules
│   │   ├── model.go       # ModelDefinition, APIFormat
│   │   ├── query.go       # QueryRequest, QueryResponse
│   │   ├── context.go     # ContextSnapshot
│   │   ├── shell.go       # Shell enums, InstallResult
│   │   ├── health.go      # HealthCheck structures
│   │   ├── constants.go   # Shared constants
│   │   └── config_behavior.go  # Config helper methods
│   │
│   ├── ports/             # Interface definitions (12 ports)
│   │   └── ports.go       # Provider, SecurityService, etc.
│   │
│   ├── services/          # Use case orchestration (3 services)
│   │   ├── query.go       # Core command generation pipeline
│   │   ├── query_test.go  # Query service tests
│   │   └── health.go      # Environment diagnostics
│   │
│   ├── infrastructure/    # Concrete implementations (13+ adapters)
│   │   ├── ai/
│   │   │   └── ai.go      # Configuration-driven HTTP provider
│   │   ├── cli/
│   │   │   ├── root.go    # Cobra root command setup
│   │   │   ├── commands/  # Install/uninstall commands
│   │   │   ├── prompter.go     # Interactive confirmation
│   │   │   ├── renderer.go     # Output formatting
│   │   │   ├── clipboard.go    # Cross-platform clipboard
│   │   │   ├── spinner.go      # Loading animation
│   │   │   └── stream_writer.go # Streaming output
│   │   ├── config.go      # FileLoader (ConfigProvider)
│   │   ├── security.go    # Guardrail (SecurityService)
│   │   ├── context.go     # BasicCollector (ContextCollector)
│   │   ├── executor.go    # LocalExecutor (CommandExecutor)
│   │   └── shell.go       # ShellIntegration installer
│   │
│   ├── app/               # Dependency injection container
│   │   └── container.go   # BuildContainer() wires services
│   │
│   └── pkg/               # Shared utilities
│       ├── logger/        # Structured logging
│       └── filesystem/    # Filesystem utilities
│
└── assets/                # Embedded resources
    ├── defaults/
    │   ├── config.yaml    # Default configuration template
    │   └── guardrail.yaml # Default security rules
    ├── shell/
    │   ├── bash.sh        # Bash integration script
    │   └── zsh.sh         # Zsh integration script
    └── embed.go           # Go embed declarations
```

### Design Principles

- **Dependency Inversion**: Infrastructure depends on domain, never the opposite
- **Single Responsibility**: Each component has exactly one reason to change
- **Interface Segregation**: Small, focused interfaces (12 ports)
- **Open/Closed**: Extensibility via configuration, not code modification
- **100% Testable**: Services use dependency injection with mockable interfaces

### Key Components

**Domain Layer** (Pure Business Logic):

- No external dependencies
- Contains all business rules
- Defines data structures and behavior
- Enforces configuration consistency

**Ports Layer** (Interfaces):

- Defines contracts between layers
- Enables dependency inversion
- Makes entire codebase testable
- 12 focused interfaces

**Services Layer** (Use Cases):

- Orchestrates business workflows
- QueryService: Command generation pipeline
- HealthService: Environment diagnostics
- ConfigService: Configuration management

**Infrastructure Layer** (Adapters):

- AI Provider: HTTP client with template-based prompts
- Guardrail: Pattern-based security evaluation
- Context Collector: Git, K8s, Docker detection
- Config Loader: YAML parsing with defaults
- Shell Integration: RC file management

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

# Verbose output
go test -v ./internal/services/...

# Specific package
go test ./internal/infrastructure/
```

### Code Quality

```bash
# Format code
go fmt ./...

# Vet for suspicious constructs
go vet ./...

# Lint (requires golangci-lint)
golangci-lint run
```

---

## Project Statistics

- **Language**: Go 1.25.3
- **Total Files**: 39 Go files
- **Lines of Code**: ~5,000 LOC
- **Test Coverage**: 17 test functions across 4 test files
- **Dependencies**: 2 direct dependencies (Cobra, YAML)
- **Architecture**: Hexagonal (12 ports, 13 adapters)
- **External SDKs**: Zero (all AI communication via standard HTTP)

---

## Requirements

- **Go**: 1.21+ (tested with 1.25.3)
- **AI Provider**: At least one of:
    - Anthropic API key (`ANTHROPIC_API_KEY`)
    - OpenAI API key (`OPENAI_API_KEY`)
    - Ollama running locally (no key required)
- **Platform**: macOS, Linux, Windows

---

## Security Notes

**SHAI is an AI-powered tool. Always review generated commands before execution.**

Security Best Practices:

- Review all commands, especially those involving `sudo`, `rm`, or system paths
- Keep guardrail rules updated (`~/.shai/guardrail.yaml`)
- Use `--debug` flag to inspect context being sent to AI
- API keys are read from environment variables and never logged
- Never disable security guardrails in production environments
- Report security issues via GitHub Issues

---

## FAQ

**Q: Which AI provider should I use?**
A: Anthropic Claude Sonnet 4 offers the best balance of accuracy and cost. Use Ollama for offline/local usage.

**Q: Is my API key secure?**
A: Yes. Keys are read from environment variables and never logged, cached, or transmitted outside of AI provider API
calls.

**Q: Can I use SHAI offline?**
A: Yes, with Ollama running locally. No internet connection required.

**Q: What if a dangerous command gets through guardrails?**
A: Always review commands before execution. Report false negatives via GitHub Issues to improve guardrail rules.

**Q: How do I customize AI prompts?**
A: Edit the `prompt` section in your model definition in `~/.shai/config.yaml`. Use template variables for dynamic
context.

**Q: Does SHAI support Windows?**
A: Yes, SHAI builds and runs on Windows. Shell integration requires bash or zsh (Git Bash, WSL, Cygwin).

**Q: How do I add a new AI provider?**
A: Add a model definition to `~/.shai/config.yaml` with appropriate `api_format` settings. No code changes needed.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- YAML parsing via [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml)
- Inspired by GitHub Copilot CLI and Amazon CodeWhisperer

## Support

- **Issues**: [GitHub Issues](https://github.com/doeshing/shai-go/issues)
- **Discussions**: [GitHub Discussions](https://github.com/doeshing/shai-go/discussions)
- **Documentation**: Coming soon at https://docs.shai.dev

---

**⚠️ Important**: SHAI is an AI-powered tool. Always review generated commands before execution, especially for
destructive operations or sensitive environments.

// Package ports defines the interfaces (ports) for the hexagonal architecture.
//
// This package establishes the contract between the application core and external
// adapters (infrastructure). Following the Ports and Adapters (Hexagonal) pattern,
// these interfaces allow the application to remain independent of specific
// implementations like databases, HTTP clients, or CLI frameworks.
//
// Key architectural concepts:
//   - Ports: Interfaces defined here (e.g., Provider, ConfigProvider)
//   - Adapters: Concrete implementations in the infrastructure layer
//   - Dependency inversion: Application depends on abstractions, not implementations
package ports

import (
	"context"

	"github.com/doeshing/shai-go/internal/domain"
)

// ConfigProvider loads the latest configuration from persistent storage.
// Implementations typically read from ~/.shai/config.yaml.
type ConfigProvider interface {
	Load(context.Context) (domain.Config, error)
}

// ContextCollector gathers environmental context (git, k8s, files, etc.) to enrich AI prompts.
// This provides the AI with situational awareness about the user's current working environment.
type ContextCollector interface {
	Collect(context.Context, domain.Config, domain.QueryRequest) (domain.ContextSnapshot, error)
}

// ProviderFactory builds AI provider instances based on model definitions.
// It abstracts the creation of different provider types (Anthropic, OpenAI, Ollama).
type ProviderFactory interface {
	ForModel(domain.ModelDefinition) (Provider, error)
}

// Provider defines the core AI generation capability for producing shell commands.
// Each provider implementation wraps a specific AI service API.
type Provider interface {
	Name() string
	Model() domain.ModelDefinition
	Generate(context.Context, ProviderRequest) (ProviderResponse, error)
}

// ProviderRequest contains all data needed to generate an AI response.
// This includes the user's prompt, environmental context, and generation parameters.
type ProviderRequest struct {
	Prompt       string
	Context      domain.ContextSnapshot
	Model        domain.ModelDefinition
	Debug        bool
	Stream       bool
	StreamWriter domain.StreamWriter
}

// ProviderResponse contains the AI's generated command and explanatory text.
// The Command field holds the executable shell command, while Reply provides context.
type ProviderResponse struct {
	Command   string
	Reply     string
	Reasoning string
}

// SecurityService evaluates commands against security rules to prevent dangerous operations.
// This implements the guardrail system that warns users about potentially harmful commands.
type SecurityService interface {
	Evaluate(command string) (domain.RiskAssessment, error)
}

// CommandExecutor runs shell commands in the configured shell environment.
type CommandExecutor interface {
	Execute(ctx context.Context, command string) (domain.ExecutionResult, error)
}

// ConfirmationPrompter handles interactive user confirmations for risky operations.
// Used by the guardrail system to get user approval before executing dangerous commands.
type ConfirmationPrompter interface {
	Confirm(action domain.GuardrailAction, risk domain.RiskLevel, command string, reasons []string) (bool, error)
	Enabled() bool
}

// Clipboard provides cross-platform clipboard integration for copying commands.
// Allows users to copy generated commands without manually selecting text.
type Clipboard interface {
	Copy(text string) error
	Enabled() bool
}

// ShellIntegrator manages shell integration hooks (bash, zsh, fish).
// Handles installation and removal of shell aliases and functions for seamless CLI usage.
type ShellIntegrator interface {
	Install(shell string, force bool) (domain.ShellInstallResult, error)
	Uninstall(shell string) (domain.ShellInstallResult, error)
	Status(shell string) domain.ShellStatus
	DetectShell() string
}

// Logger provides structured logging abstraction for the application layer.
// Implementations can route to different backends (stdout, files, external services).
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, err error, fields map[string]interface{})
}

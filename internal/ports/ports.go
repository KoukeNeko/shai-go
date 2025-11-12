package ports

import (
	"context"

	"github.com/doeshing/shai-go/internal/domain"
)

// ConfigProvider loads the latest configuration snapshot.
type ConfigProvider interface {
	Load(context.Context) (domain.Config, error)
}

// ContextCollector gathers environment context for prompts and logs.
type ContextCollector interface {
	Collect(context.Context, domain.Config, domain.QueryRequest) (domain.ContextSnapshot, error)
}

// ProviderRegistry resolves AI providers by model name.
// ProviderFactory builds provider instances for the configured models.
type ProviderFactory interface {
	ForModel(domain.ModelDefinition) (Provider, error)
}

// Provider defines the minimal AI generation capability needed by the use case.
type Provider interface {
	Name() string
	Model() domain.ModelDefinition
	Generate(context.Context, ProviderRequest) (ProviderResponse, error)
}

// ProviderRequest is fed into the AI provider.
type ProviderRequest struct {
	Prompt       string
	Context      domain.ContextSnapshot
	Model        domain.ModelDefinition
	Debug        bool
	Stream       bool
	StreamWriter domain.StreamWriter
}

// ProviderResponse returns a suggested command and reasoning.
type ProviderResponse struct {
	Command   string
	Reply     string
	Reasoning string
}

// SecurityService evaluates generated commands before execution.
type SecurityService interface {
	Evaluate(command string) (domain.RiskAssessment, error)
}

// CommandExecutor runs shell commands and streams output back.
type CommandExecutor interface {
	Execute(ctx context.Context, command string, previewOnly bool) (domain.ExecutionResult, error)
}

// ConfirmationPrompter handles interactive confirmations.
type ConfirmationPrompter interface {
	Confirm(action domain.GuardrailAction, risk domain.RiskLevel, command string, reasons []string) (bool, error)
	Enabled() bool
}

// Clipboard provides optional copy-to-clipboard functionality.
type Clipboard interface {
	Copy(text string) error
	Enabled() bool
}

// ShellIntegrator manages shell hook installation.
type ShellIntegrator interface {
	Install(shell string, force bool) (domain.ShellInstallResult, error)
	Uninstall(shell string) (domain.ShellInstallResult, error)
	Status(shell string) domain.ShellStatus
	DetectShell() string
}

// HistoryStore persists query history.
type HistoryStore interface {
	Save(record domain.HistoryRecord) error
}

// HistoryRepository extends HistoryStore with query utilities.
type HistoryRepository interface {
	HistoryStore
	Records(limit int, search string) ([]domain.HistoryRecord, error)
	Clear() error
	ExportJSON(path string) error
	Path() string
}

// CacheStore stores provider responses keyed by context hash.
type CacheStore interface {
	Get(key string) (domain.CacheEntry, bool, error)
	Set(entry domain.CacheEntry) error
}

// CacheRepository extends CacheStore with management utilities.
type CacheRepository interface {
	CacheStore
	Entries() ([]domain.CacheEntry, error)
	Clear() error
	Dir() string
}

// Logger is a minimal logging facade for application layer.
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, err error, fields map[string]interface{})
}

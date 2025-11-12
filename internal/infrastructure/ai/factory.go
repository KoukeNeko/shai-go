package ai

import (
	"net/http"
	"strings"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Factory implements ProviderFactory by detecting provider kind via endpoint.
type Factory struct {
	httpClient *http.Client
}

// NewFactory creates a new AI provider factory.
func NewFactory() *Factory {
	return &Factory{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ForModel returns a provider implementation for the given model definition.
func (f *Factory) ForModel(model domain.ModelDefinition) (ports.Provider, error) {
	switch inferProvider(model.Endpoint, model.Name) {
	case domain.ProviderKindAnthropic:
		return newAnthropicProvider(model, f.httpClient), nil
	case domain.ProviderKindOpenAI:
		return newOpenAIProvider(model, f.httpClient), nil
	case domain.ProviderKindOllama:
		return newOllamaProvider(model, f.httpClient), nil
	default:
		return newHeuristicProvider(model), nil
	}
}

func inferProvider(endpoint string, name string) domain.ProviderKind {
	nameLower := strings.ToLower(name)
	switch {
	case strings.Contains(endpoint, "anthropic.com"):
		return domain.ProviderKindAnthropic
	case strings.Contains(endpoint, "openai.com"):
		return domain.ProviderKindOpenAI
	case strings.Contains(nameLower, "ollama"):
		return domain.ProviderKindOllama
	case strings.Contains(endpoint, "11434"):
		return domain.ProviderKindOllama
	case strings.Contains(endpoint, "localhost"):
		return domain.ProviderKindOllama
	default:
		return domain.ProviderKindUnknown
	}
}

var _ ports.ProviderFactory = (*Factory)(nil)

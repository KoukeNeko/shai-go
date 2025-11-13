package ai

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

type Factory struct {
	httpClient *http.Client
}

func NewFactory() *Factory {
	return &Factory{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (f *Factory) ForModel(model domain.ModelDefinition) (ports.Provider, error) {
	providerKind := inferProviderKind(model.Endpoint, model.Name)

	switch providerKind {
	case domain.ProviderKindAnthropic:
		return newHTTPProvider("anthropic", model, f.httpClient, anthropicAdapter()), nil
	case domain.ProviderKindOpenAI:
		return newHTTPProvider("openai", model, f.httpClient, openaiAdapter()), nil
	case domain.ProviderKindOllama:
		return newHTTPProvider("ollama", model, f.httpClient, ollamaAdapter()), nil
	case domain.ProviderKindUnknown:
		return newHeuristicProvider(model), nil
	default:
		return nil, fmt.Errorf("unsupported provider kind: %s", providerKind)
	}
}

func inferProviderKind(endpoint string, name string) domain.ProviderKind {
	nameLower := strings.ToLower(name)

	switch {
	case strings.Contains(endpoint, "anthropic.com"):
		return domain.ProviderKindAnthropic
	case strings.Contains(endpoint, "openai.com"):
		return domain.ProviderKindOpenAI
	case strings.Contains(nameLower, "ollama"), strings.Contains(endpoint, "11434"), strings.Contains(endpoint, "localhost"):
		return domain.ProviderKindOllama
	default:
		return domain.ProviderKindUnknown
	}
}

var _ ports.ProviderFactory = (*Factory)(nil)

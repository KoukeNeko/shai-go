// Package ai provides AI provider factory for creating provider instances.
//
// The factory creates generic HTTP providers configured entirely through
// the model's APIFormat settings. No provider-specific logic is needed.
package ai

import (
	"net/http"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

const httpClientTimeout = 60 * time.Second

// Factory creates AI provider instances based on model definitions.
// It maintains a single HTTP client shared across all providers.
type Factory struct {
	httpClient *http.Client
}

// NewFactory creates a new provider factory with a configured HTTP client.
func NewFactory() *Factory {
	return &Factory{
		httpClient: &http.Client{Timeout: httpClientTimeout},
	}
}

// ForModel creates a generic HTTP provider for any model definition.
// All provider-specific behavior is controlled through the model's APIFormat configuration.
func (f *Factory) ForModel(model domain.ModelDefinition) (ports.Provider, error) {
	return newHTTPProvider(model, f.httpClient), nil
}

var _ ports.ProviderFactory = (*Factory)(nil)

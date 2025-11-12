package app

import (
	"context"

	"github.com/doeshing/shai-go/internal/application/query"
	"github.com/doeshing/shai-go/internal/infrastructure/ai"
	"github.com/doeshing/shai-go/internal/infrastructure/config"
	contextcollector "github.com/doeshing/shai-go/internal/infrastructure/context"
	"github.com/doeshing/shai-go/internal/infrastructure/executor"
	"github.com/doeshing/shai-go/internal/infrastructure/security"
	"github.com/doeshing/shai-go/internal/pkg/logger"
)

// Container wires up application services with infrastructure adapters.
type Container struct {
	QueryService *query.Service
}

// BuildContainer constructs the dependency graph.
func BuildContainer(ctx context.Context, verbose bool) (*Container, error) {
	cfgLoader := config.NewFileLoader("")
	cfg, err := cfgLoader.Load(ctx)
	if err != nil {
		return nil, err
	}

	guardrail, err := security.NewGuardrail(cfg.Security.RulesFile)
	if err != nil {
		guardrail, err = security.NewGuardrail("")
		if err != nil {
			return nil, err
		}
	}

	service := &query.Service{
		ConfigProvider:   cfgLoader,
		ContextCollector: contextcollector.NewBasicCollector(),
		ProviderFactory:  ai.NewFactory(),
		SecurityService:  guardrail,
		Executor:         executor.NewLocalExecutor(""),
		Logger:           logger.NewStd(verbose),
	}

	return &Container{
		QueryService: service,
	}, nil
}

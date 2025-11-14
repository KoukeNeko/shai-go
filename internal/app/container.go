package app

import (
	"context"

	"github.com/doeshing/shai-go/internal/infrastructure"
	"github.com/doeshing/shai-go/internal/infrastructure/ai"
	"github.com/doeshing/shai-go/internal/pkg/logger"
	"github.com/doeshing/shai-go/internal/ports"
	"github.com/doeshing/shai-go/internal/services"
)

// Container wires up application services with infrastructure adapters.
type Container struct {
	QueryService    *services.QueryService
	ConfigProvider  ports.ConfigProvider
	ConfigLoader    *infrastructure.FileLoader
	ShellIntegrator ports.ShellIntegrator
	HealthService   *services.HealthService
}

// BuildContainer constructs the dependency graph.
func BuildContainer(ctx context.Context, verbose bool) (*Container, error) {
	cfgLoader := infrastructure.NewFileLoader("")
	cfg, err := cfgLoader.Load(ctx)
	if err != nil {
		return nil, err
	}

	log := logger.NewStd(verbose)
	collector := infrastructure.NewBasicCollector()

	guardrail, err := infrastructure.NewGuardrail(cfg.Security.RulesFile)
	if err != nil {
		guardrail, err = infrastructure.NewGuardrail("")
		if err != nil {
			return nil, err
		}
	}

	shellInstaller := infrastructure.NewInstaller(log)

	queryService := &services.QueryService{
		ConfigProvider:   cfgLoader,
		ContextCollector: collector,
		ProviderFactory:  ai.NewFactory(),
		SecurityService:  guardrail,
		Executor:         infrastructure.NewLocalExecutor(""),
		Logger:           log,
	}

	healthService := &services.HealthService{
		ConfigProvider:   cfgLoader,
		ShellIntegrator:  shellInstaller,
		SecurityService:  guardrail,
		ContextCollector: collector,
	}

	return &Container{
		QueryService:    queryService,
		ConfigProvider:  cfgLoader,
		ConfigLoader:    cfgLoader,
		ShellIntegrator: shellInstaller,
		HealthService:   healthService,
	}, nil
}

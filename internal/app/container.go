package app

import (
	"context"

	"github.com/doeshing/shai-go/internal/application/doctor"
	"github.com/doeshing/shai-go/internal/application/query"
	"github.com/doeshing/shai-go/internal/infrastructure/ai"
	"github.com/doeshing/shai-go/internal/infrastructure/cache"
	"github.com/doeshing/shai-go/internal/infrastructure/config"
	contextcollector "github.com/doeshing/shai-go/internal/infrastructure/context"
	"github.com/doeshing/shai-go/internal/infrastructure/executor"
	"github.com/doeshing/shai-go/internal/infrastructure/history"
	"github.com/doeshing/shai-go/internal/infrastructure/security"
	"github.com/doeshing/shai-go/internal/infrastructure/shell"
	"github.com/doeshing/shai-go/internal/pkg/logger"
	"github.com/doeshing/shai-go/internal/ports"
)

// Container wires up application services with infrastructure adapters.
type Container struct {
	QueryService    *query.Service
	ConfigProvider  ports.ConfigProvider
	ConfigLoader    *config.FileLoader
	ShellIntegrator ports.ShellIntegrator
	DoctorService   *doctor.Service
	HistoryStore    ports.HistoryRepository
	CacheStore      ports.CacheRepository
}

// BuildContainer constructs the dependency graph.
func BuildContainer(ctx context.Context, verbose bool) (*Container, error) {
	cfgLoader := config.NewFileLoader("")
	cfg, err := cfgLoader.Load(ctx)
	if err != nil {
		return nil, err
	}

	log := logger.NewStd(verbose)
	collector := contextcollector.NewBasicCollector()
	historyStore := history.NewSQLiteStore()
	cacheStore := cache.NewFileCache()

	guardrail, err := security.NewGuardrail(cfg.Security.RulesFile)
	if err != nil {
		guardrail, err = security.NewGuardrail("")
		if err != nil {
			return nil, err
		}
	}

	shellInstaller := shell.NewInstaller(log)

	queryService := &query.Service{
		ConfigProvider:   cfgLoader,
		ContextCollector: collector,
		ProviderFactory:  ai.NewFactory(),
		SecurityService:  guardrail,
		Executor:         executor.NewLocalExecutor(""),
		Logger:           log,
		HistoryStore:     historyStore,
		CacheStore:       cacheStore,
	}

	doctorService := &doctor.Service{
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
		DoctorService:   doctorService,
		HistoryStore:    historyStore,
		CacheStore:      cacheStore,
	}, nil
}

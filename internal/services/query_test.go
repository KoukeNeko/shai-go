package services

import (
	"context"
	"testing"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/pkg/logger"
	"github.com/doeshing/shai-go/internal/ports"
)

func TestServiceRunExecutesCommandWhenAllowed(t *testing.T) {
	cfg := domain.Config{
		Preferences: domain.Preferences{DefaultModel: "claude"},
		Models: []domain.ModelDefinition{
			{Name: "claude", ModelID: "claude", Endpoint: "anthropic"},
		},
	}

	executor := &stubExecutor{
		result: domain.ExecutionResult{Ran: true, Stdout: "ok"},
	}

	svc := &QueryService{
		ConfigProvider:   stubConfigProvider{cfg: cfg},
		ContextCollector: stubContextCollector{snapshot: domain.ContextSnapshot{WorkingDir: "/tmp"}},
		ProviderFactory:  stubProviderFactory{provider: stubProvider{}},
		SecurityService:  stubSecurity{risk: domain.RiskAssessment{Action: domain.ActionAllow}},
		Executor:         executor,
		Logger:           logger.NewStd(false),
	}

	resp, err := svc.Run(domain.QueryRequest{
		Context:     context.Background(),
		Prompt:      "list files",
		AutoExecute: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if resp.ExecutionResult == nil || !resp.ExecutionResult.Ran {
		t.Fatalf("expected command to execute, got %+v", resp.ExecutionResult)
	}
	if !executor.called {
		t.Fatal("executor was not called")
	}
}

func TestServiceRunBlocksWhenGuardrailBlocks(t *testing.T) {
	cfg := domain.Config{
		Preferences: domain.Preferences{DefaultModel: "claude"},
		Models:      []domain.ModelDefinition{{Name: "claude", ModelID: "claude", Endpoint: "anthropic"}},
	}

	svc := &QueryService{
		ConfigProvider:   stubConfigProvider{cfg: cfg},
		ContextCollector: stubContextCollector{snapshot: domain.ContextSnapshot{WorkingDir: "/tmp"}},
		ProviderFactory:  stubProviderFactory{provider: stubProvider{}},
		SecurityService:  stubSecurity{risk: domain.RiskAssessment{Action: domain.ActionBlock}},
		Executor:         &stubExecutor{},
		Logger:           logger.NewStd(false),
	}

	_, err := svc.Run(domain.QueryRequest{
		Context: context.Background(),
		Prompt:  "dangerous",
	})
	if err == nil {
		t.Fatal("expected error due to guardrail block")
	}
}

type stubConfigProvider struct {
	cfg domain.Config
	err error
}

func (s stubConfigProvider) Load(context.Context) (domain.Config, error) {
	return s.cfg, s.err
}

type stubContextCollector struct {
	snapshot domain.ContextSnapshot
	err      error
}

func (s stubContextCollector) Collect(context.Context, domain.Config, domain.QueryRequest) (domain.ContextSnapshot, error) {
	return s.snapshot, s.err
}

type stubProviderFactory struct {
	provider ports.Provider
	err      error
}

func (s stubProviderFactory) ForModel(domain.ModelDefinition) (ports.Provider, error) {
	if s.provider == nil {
		return stubProvider{}, nil
	}
	return s.provider, s.err
}

type stubProvider struct{}

func (stubProvider) Name() string                  { return "stub" }
func (stubProvider) Model() domain.ModelDefinition { return domain.ModelDefinition{} }
func (stubProvider) Generate(context.Context, ports.ProviderRequest) (ports.ProviderResponse, error) {
	return ports.ProviderResponse{Command: "ls"}, nil
}

type stubSecurity struct {
	risk domain.RiskAssessment
	err  error
}

func (s stubSecurity) Evaluate(string) (domain.RiskAssessment, error) {
	return s.risk, s.err
}

type stubExecutor struct {
	result domain.ExecutionResult
	err    error
	called bool
}

func (s *stubExecutor) Execute(context.Context, string, bool) (domain.ExecutionResult, error) {
	s.called = true
	return s.result, s.err
}

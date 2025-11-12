package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Service runs environment diagnostics.
type Service struct {
	ConfigProvider   ports.ConfigProvider
	ShellIntegrator  ports.ShellIntegrator
	SecurityService  ports.SecurityService
	ContextCollector ports.ContextCollector
}

// Run executes checks and returns a report.
func (s *Service) Run(ctx context.Context) (domain.HealthReport, error) {
	var checks []domain.HealthCheck

	cfg, err := s.ConfigProvider.Load(ctx)
	if err != nil {
		checks = append(checks, fail("Config file", fmt.Sprintf("load failed: %v", err)))
		return domain.HealthReport{Checks: checks}, err
	}
	checks = append(checks, ok("Config file", fmt.Sprintf("loaded %s", cfg.ConfigFormatVersion)))

	if s.SecurityService != nil {
		if _, err := s.SecurityService.Evaluate("ls"); err != nil {
			checks = append(checks, fail("Guardrail", err.Error()))
		} else {
			checks = append(checks, ok("Guardrail", "rules loaded"))
		}
	} else {
		checks = append(checks, warn("Guardrail", "security service not initialized"))
	}

	if s.ContextCollector != nil {
		if snapshot, err := s.ContextCollector.Collect(ctx, cfg, domain.QueryRequest{}); err == nil {
			checks = append(checks, ok("Context collector", fmt.Sprintf("detected tools: %d", len(snapshot.AvailableTools))))
		} else {
			checks = append(checks, warn("Context collector", err.Error()))
		}
	}

	if s.ShellIntegrator != nil {
		status := s.ShellIntegrator.Status("")
		if status.ScriptExists && status.LinePresent {
			checks = append(checks, ok("Shell integration", fmt.Sprintf("%s ready", status.Shell)))
		} else if status.Error != "" {
			checks = append(checks, warn("Shell integration", status.Error))
		} else {
			checks = append(checks, warn("Shell integration", "not installed"))
		}
	}

	checks = append(checks, apiCheck(cfg.Models))

	return domain.HealthReport{Checks: checks}, nil
}

func apiCheck(models []domain.ModelDefinition) domain.HealthCheck {
	for _, model := range models {
		switch detectProvider(model.Endpoint) {
		case domain.ProviderKindAnthropic:
			if envMissing(model.AuthEnvVar, "ANTHROPIC_API_KEY") {
				return warn("API keys", "ANTHROPIC_API_KEY missing")
			}
		case domain.ProviderKindOpenAI:
			if envMissing(model.AuthEnvVar, "OPENAI_API_KEY") {
				return warn("API keys", "OPENAI_API_KEY missing")
			}
		}
	}
	return ok("API keys", "detected for configured providers")
}

func detectProvider(endpoint string) domain.ProviderKind {
	switch {
	case strings.Contains(endpoint, "anthropic.com"):
		return domain.ProviderKindAnthropic
	case strings.Contains(endpoint, "openai.com"):
		return domain.ProviderKindOpenAI
	default:
		return domain.ProviderKindUnknown
	}
}

func envMissing(primary, fallback string) bool {
	if primary != "" && os.Getenv(primary) != "" {
		return false
	}
	if fallback != "" && os.Getenv(fallback) != "" {
		return false
	}
	return true
}

func ok(name, details string) domain.HealthCheck {
	return domain.HealthCheck{Name: name, Status: domain.HealthOK, Details: details}
}

func warn(name, details string) domain.HealthCheck {
	return domain.HealthCheck{Name: name, Status: domain.HealthWarn, Details: details}
}

func fail(name, details string) domain.HealthCheck {
	return domain.HealthCheck{Name: name, Status: domain.HealthError, Details: details}
}

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// DoctorService runs environment diagnostics.
type DoctorService struct {
	ConfigProvider   ports.ConfigProvider
	ShellIntegrator  ports.ShellIntegrator
	SecurityService  ports.SecurityService
	ContextCollector ports.ContextCollector
}

// Run executes checks and returns a report.
func (s *DoctorService) Run(ctx context.Context) (domain.HealthReport, error) {
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
		if snapshot, err := s.ContextCollector.Collect(ctx, cfg, domain.QueryRequest{WithEnv: true, WithK8sInfo: true}); err == nil {
			checks = append(checks, ok("Context collector", fmt.Sprintf("detected tools: %d", len(snapshot.AvailableTools))))
			checks = append(checks, contextDiagnostics(snapshot, cfg)...)
		} else {
			checks = append(checks, warn("Context collector", err.Error()))
		}
	}

	if s.ShellIntegrator != nil {
		checks = append(checks, shellDiagnostics(s.ShellIntegrator, domain.ShellZsh))
		checks = append(checks, shellDiagnostics(s.ShellIntegrator, domain.ShellBash))
	}

	checks = append(checks, apiCheck(cfg.Models))
	checks = append(checks, guardrailFileCheck(cfg.Security.RulesFile))

	return domain.HealthReport{Checks: checks}, nil
}

func contextDiagnostics(snapshot domain.ContextSnapshot, cfg domain.Config) []domain.HealthCheck {
	var checks []domain.HealthCheck
	if snapshot.Git != nil {
		checks = append(checks, ok("Git status", fmt.Sprintf("branch %s, modified %d", snapshot.Git.Branch, snapshot.Git.ModifiedCount)))
	} else if shouldCheck(cfg.Context.IncludeGit) {
		checks = append(checks, warn("Git status", "no git repo detected"))
	}
	if snapshot.Kubernetes != nil && snapshot.Kubernetes.Context != "" {
		checks = append(checks, ok("Kubernetes", fmt.Sprintf("context %s namespace %s", snapshot.Kubernetes.Context, snapshot.Kubernetes.Namespace)))
	} else if shouldCheck(cfg.Context.IncludeK8s) {
		checks = append(checks, warn("Kubernetes", "kubectl context not detected"))
	}
	if snapshot.Docker != nil && snapshot.Docker.Running {
		checks = append(checks, ok("Docker", snapshot.Docker.Info))
	}
	return checks
}

func shellDiagnostics(installer ports.ShellIntegrator, shell domain.ShellName) domain.HealthCheck {
	status := installer.Status(string(shell))
	name := fmt.Sprintf("Shell %s", shell)
	if status.Error != "" {
		return warn(name, status.Error)
	}
	if status.ScriptExists && status.LinePresent {
		return ok(name, fmt.Sprintf("hook active (%s)", status.RCFile))
	}
	return warn(name, "integration not installed")
}

func apiCheck(models []domain.ModelDefinition) domain.HealthCheck {
	for _, model := range models {
		if isAnthropicEndpoint(model.Endpoint) {
			if envMissing(model.AuthEnvVar, "ANTHROPIC_API_KEY") {
				return warn("API keys", "ANTHROPIC_API_KEY missing")
			}
		} else if isOpenAIEndpoint(model.Endpoint) {
			if envMissing(model.AuthEnvVar, "OPENAI_API_KEY") {
				return warn("API keys", "OPENAI_API_KEY missing")
			}
		}
	}
	return ok("API keys", "detected for configured providers")
}

func guardrailFileCheck(path string) domain.HealthCheck {
	if path == "" {
		return warn("Guardrail file", "security.rules_file not configured")
	}
	expanded := expandPath(path)
	if _, err := os.Stat(expanded); err != nil {
		return warn("Guardrail file", fmt.Sprintf("missing at %s", expanded))
	}
	return ok("Guardrail file", expanded)
}

func isAnthropicEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "anthropic.com")
}

func isOpenAIEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "openai.com")
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

func shouldCheck(setting string) bool {
	switch strings.ToLower(setting) {
	case "always":
		return true
	case "never":
		return false
	default:
		return true
	}
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
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

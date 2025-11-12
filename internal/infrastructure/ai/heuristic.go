package ai

import (
	"context"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

type heuristicProvider struct {
	model domain.ModelDefinition
}

func newHeuristicProvider(model domain.ModelDefinition) ports.Provider {
	return &heuristicProvider{model: model}
}

func (p *heuristicProvider) Name() string {
	return "heuristic"
}

func (p *heuristicProvider) Model() domain.ModelDefinition {
	return p.model
}

func (p *heuristicProvider) Generate(_ context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	command := guessCommand(req.Prompt, req.Context)
	return ports.ProviderResponse{
		Command:   command,
		Reply:     "Heuristic provider suggestion (offline fallback)",
		Reasoning: "Generated locally due to missing AI credentials",
	}, nil
}

func guessCommand(prompt string, ctx domain.ContextSnapshot) string {
	prompt = strings.ToLower(prompt)
	switch {
	case strings.Contains(prompt, "docker"):
		return "docker ps"
	case strings.Contains(prompt, "git status"):
		return "git status"
	case strings.Contains(prompt, "list") && strings.Contains(prompt, "file"):
		return "ls -la"
	case strings.Contains(prompt, "kubernetes") || strings.Contains(prompt, "pod"):
		return "kubectl get pods"
	default:
		return "echo \"No AI provider configured\""
	}
}

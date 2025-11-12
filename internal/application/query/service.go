package query

import (
	"context"
	"errors"
	"fmt"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Service orchestrates the query lifecycle end-to-end.
type Service struct {
	ConfigProvider   ports.ConfigProvider
	ContextCollector ports.ContextCollector
	ProviderFactory  ports.ProviderFactory
	SecurityService  ports.SecurityService
	Executor         ports.CommandExecutor
	Prompter         ports.ConfirmationPrompter
	Clipboard        ports.Clipboard
	Logger           ports.Logger
}

// Run processes a single natural-language query.
func (s *Service) Run(req domain.QueryRequest) (domain.QueryResponse, error) {
	if s.ConfigProvider == nil || s.ContextCollector == nil || s.ProviderFactory == nil ||
		s.SecurityService == nil || s.Executor == nil || s.Logger == nil {
		return domain.QueryResponse{}, errors.New("query.Service dependencies not satisfied")
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := s.ConfigProvider.Load(ctx)
	if err != nil {
		return domain.QueryResponse{}, fmt.Errorf("load config: %w", err)
	}

	ctxSnapshot, err := s.ContextCollector.Collect(ctx, cfg, req)
	if err != nil {
		return domain.QueryResponse{}, fmt.Errorf("collect context: %w", err)
	}

	modelDef, err := pickModel(cfg, req.ModelOverride)
	if err != nil {
		return domain.QueryResponse{}, err
	}

	provider, err := s.ProviderFactory.ForModel(modelDef)
	if err != nil {
		return domain.QueryResponse{}, fmt.Errorf("provider init: %w", err)
	}

	s.Logger.Info("calling provider", map[string]interface{}{
		"provider": provider.Name(),
		"model":    modelDef.ModelID,
	})

	aiResp, err := provider.Generate(ctx, ports.ProviderRequest{
		Prompt:  req.Prompt,
		Context: ctxSnapshot,
		Model:   modelDef,
		Debug:   req.Debug,
	})
	if err != nil {
		return domain.QueryResponse{}, fmt.Errorf("provider generate: %w", err)
	}

	risk, err := s.SecurityService.Evaluate(aiResp.Command)
	if err != nil {
		return domain.QueryResponse{}, fmt.Errorf("security evaluate: %w", err)
	}

	resp := domain.QueryResponse{
		Command:            aiResp.Command,
		NaturalLanguage:    req.Prompt,
		Reasoning:          aiResp.Reasoning,
		RiskAssessment:     risk,
		ContextInformation: ctxSnapshot,
	}

	if req.CopyToClipboard && s.Clipboard != nil && s.Clipboard.Enabled() {
		if err := s.Clipboard.Copy(aiResp.Command); err != nil {
			s.Logger.Warn("clipboard copy failed", map[string]interface{}{"error": err.Error()})
		}
	}

	shouldExecute, err := s.decideExecution(req, cfg, risk, aiResp.Command)
	if err != nil {
		return resp, err
	}

	if !shouldExecute {
		return resp, nil
	}

	execResult, err := s.Executor.Execute(ctx, aiResp.Command, false)
	resp.ExecutionResult = &execResult
	if err != nil {
		return resp, err
	}
	resp.ExecutionPlanned = true
	return resp, nil
}

func (s *Service) decideExecution(
	req domain.QueryRequest,
	cfg domain.Config,
	risk domain.RiskAssessment,
	command string,
) (bool, error) {
	if req.PreviewOnly {
		return false, nil
	}

	switch risk.Action {
	case domain.ActionBlock:
		return false, fmt.Errorf("command blocked by guardrail: %s", command)
	case domain.ActionPreviewOnly:
		return false, nil
	case domain.ActionAllow:
		return req.AutoExecute || cfg.Preferences.AutoExecuteSafe, nil
	case domain.ActionSimpleConfirm, domain.ActionConfirm:
		if s.Prompter == nil || !s.Prompter.Enabled() {
			return false, nil
		}
		return s.Prompter.Confirm(risk.Action, risk.Level, command, risk.Reasons)
	case domain.ActionExplicitConfirm:
		if s.Prompter == nil || !s.Prompter.Enabled() {
			return false, nil
		}
		return s.Prompter.Confirm(risk.Action, risk.Level, command, risk.Reasons)
	default:
		return false, nil
	}
}

func pickModel(cfg domain.Config, override string) (domain.ModelDefinition, error) {
	name := override
	if name == "" {
		name = cfg.Preferences.DefaultModel
	}
	if name == "" && len(cfg.Models) > 0 {
		return cfg.Models[0], nil
	}
	for _, model := range cfg.Models {
		if model.Name == name {
			return model, nil
		}
	}
	return domain.ModelDefinition{}, fmt.Errorf("model %s not configured", name)
}

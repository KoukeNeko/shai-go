package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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
	HistoryStore     ports.HistoryStore
	CacheStore       ports.CacheStore
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

	aiResp, fromCache, err := s.generateCommand(ctx, modelDef, req, ctxSnapshot)
	if err != nil {
		return domain.QueryResponse{}, err
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
		FromCache:          fromCache,
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
		s.persistHistory(req, modelDef, resp, nil)
		return resp, nil
	}

	execResult, err := s.Executor.Execute(ctx, aiResp.Command, false)
	resp.ExecutionResult = &execResult
	if err != nil {
		s.persistHistory(req, modelDef, resp, &execResult)
		return resp, err
	}
	resp.ExecutionPlanned = true
	s.persistHistory(req, modelDef, resp, &execResult)
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

func (s *Service) generateCommand(ctx context.Context, modelDef domain.ModelDefinition, req domain.QueryRequest, snapshot domain.ContextSnapshot) (ports.ProviderResponse, bool, error) {
	cacheKey := ""
	if s.CacheStore != nil {
		cacheKey = cacheKeyFor(modelDef, req, snapshot)
		if entry, ok, err := s.CacheStore.Get(cacheKey); err == nil && ok {
			return ports.ProviderResponse{
				Command:   entry.Command,
				Reply:     entry.Reply,
				Reasoning: entry.Reasoning,
			}, true, nil
		}
	}

	provider, err := s.ProviderFactory.ForModel(modelDef)
	if err != nil {
		return ports.ProviderResponse{}, false, fmt.Errorf("provider init: %w", err)
	}

	s.Logger.Info("calling provider", map[string]interface{}{
		"provider": provider.Name(),
		"model":    modelDef.ModelID,
	})

	aiResp, err := provider.Generate(ctx, ports.ProviderRequest{
		Prompt:  req.Prompt,
		Context: snapshot,
		Model:   modelDef,
		Debug:   req.Debug,
	})
	if err != nil {
		return ports.ProviderResponse{}, false, fmt.Errorf("provider generate: %w", err)
	}

	if s.CacheStore != nil && cacheKey != "" {
		_ = s.CacheStore.Set(domain.CacheEntry{
			Key:       cacheKey,
			Command:   aiResp.Command,
			Reply:     aiResp.Reply,
			Reasoning: aiResp.Reasoning,
			Model:     modelDef.Name,
			CreatedAt: timeNow(),
		})
	}

	return aiResp, false, nil
}

func (s *Service) persistHistory(req domain.QueryRequest, model domain.ModelDefinition, resp domain.QueryResponse, exec *domain.ExecutionResult) {
	if s.HistoryStore == nil {
		return
	}
	record := domain.HistoryRecord{
		Timestamp: timeNow(),
		Prompt:    req.Prompt,
		Command:   resp.Command,
		Model:     model.Name,
		RiskLevel: resp.RiskAssessment.Level,
	}
	if exec != nil {
		record.Executed = exec.Ran
		record.Success = exec.Err == nil
		record.ExitCode = exec.ExitCode
		record.ExecutionTimeMS = exec.DurationMS
	}
	if err := s.HistoryStore.Save(record); err != nil {
		s.Logger.Warn("failed to persist history", map[string]interface{}{"error": err.Error()})
	}
}

func cacheKeyFor(model domain.ModelDefinition, req domain.QueryRequest, snapshot domain.ContextSnapshot) string {
	data := strings.Join([]string{
		model.Name,
		req.Prompt,
		snapshot.WorkingDir,
		strings.Join(snapshot.AvailableTools, ","),
	}, "|")
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

var timeNow = func() time.Time { return time.Now() }

package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Service orchestrates the query lifecycle end-to-end.
type QueryService struct {
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
func (s *QueryService) Run(req domain.QueryRequest) (domain.QueryResponse, error) {
	if s.ConfigProvider == nil || s.ContextCollector == nil || s.ProviderFactory == nil ||
		s.SecurityService == nil || s.Executor == nil || s.Logger == nil {
		return domain.QueryResponse{}, errors.New("services.QueryService dependencies not satisfied")
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

	aiResp, fromCache, modelUsed, err := s.generateCommand(ctx, cfg, modelDef, req, ctxSnapshot)
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
		ModelUsed:          modelUsed,
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

	execResult, err := s.Executor.Execute(ctx, aiResp.Command)
	resp.ExecutionResult = &execResult
	if err != nil {
		s.persistHistory(req, modelDef, resp, &execResult)
		return resp, err
	}
	resp.ExecutionPlanned = true
	s.persistHistory(req, modelDef, resp, &execResult)
	return resp, nil
}

func (s *QueryService) decideExecution(
	req domain.QueryRequest,
	cfg domain.Config,
	risk domain.RiskAssessment,
	command string,
) (bool, error) {
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
	if model, ok := findModel(cfg, name); ok {
		return model, nil
	}
	return domain.ModelDefinition{}, fmt.Errorf("model %s not configured", name)
}

func (s *QueryService) generateCommand(ctx context.Context, cfg domain.Config, primary domain.ModelDefinition, req domain.QueryRequest, snapshot domain.ContextSnapshot) (ports.ProviderResponse, bool, string, error) {
	candidates := s.buildCandidateModels(cfg, primary)
	if len(candidates) == 0 {
		return ports.ProviderResponse{}, false, "", fmt.Errorf("no providers available")
	}

	type result struct {
		resp      ports.ProviderResponse
		fromCache bool
		modelName string
		err       error
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(candidates))
	var wg sync.WaitGroup

	for _, model := range candidates {
		wg.Add(1)
		go func(model domain.ModelDefinition) {
			defer wg.Done()
			resp, fromCache, err := s.generateWithModel(ctx, model, req, snapshot)
			results <- result{resp: resp, fromCache: fromCache, modelName: model.Name, err: err}
		}(model)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	errs := make([]error, 0, len(candidates))
	var success *result
	for res := range results {
		if res.err == nil && success == nil {
			success = &res
			cancel()
			continue
		}
		if res.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", res.modelName, res.err))
		}
	}

	if success != nil {
		if req.Stream && req.StreamWriter != nil {
			req.StreamWriter.WriteChunk(success.resp.Reasoning)
			req.StreamWriter.Done()
		}
		return success.resp, success.fromCache, success.modelName, nil
	}

	if len(errs) == 0 {
		return ports.ProviderResponse{}, false, "", fmt.Errorf("no provider succeeded")
	}
	return ports.ProviderResponse{}, false, "", errors.Join(errs...)
}

func (s *QueryService) generateWithModel(ctx context.Context, model domain.ModelDefinition, req domain.QueryRequest, snapshot domain.ContextSnapshot) (ports.ProviderResponse, bool, error) {
	cacheKey := ""
	if s.CacheStore != nil {
		cacheKey = cacheKeyFor(model, req, snapshot)
		if entry, ok, err := s.CacheStore.Get(cacheKey); err == nil && ok {
			resp := ports.ProviderResponse{
				Command:   entry.Command,
				Reply:     entry.Reply,
				Reasoning: entry.Reasoning,
			}
			if req.Stream && req.StreamWriter != nil {
				req.StreamWriter.WriteChunk(resp.Reasoning)
				req.StreamWriter.Done()
			}
			return resp, true, nil
		}
	}

	provider, err := s.ProviderFactory.ForModel(model)
	if err != nil {
		return ports.ProviderResponse{}, false, fmt.Errorf("provider init: %w", err)
	}

	s.Logger.Info("calling provider", map[string]interface{}{
		"provider": provider.Name(),
		"model":    model.ModelID,
	})

	aiResp, err := provider.Generate(ctx, ports.ProviderRequest{
		Prompt:       req.Prompt,
		Context:      snapshot,
		Model:        model,
		Debug:        req.Debug,
		Stream:       req.Stream,
		StreamWriter: req.StreamWriter,
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
			Model:     model.Name,
			CreatedAt: timeNow(),
		})
	}

	return aiResp, false, nil
}

func (s *QueryService) buildCandidateModels(cfg domain.Config, primary domain.ModelDefinition) []domain.ModelDefinition {
	candidates := make([]domain.ModelDefinition, 0, 1+len(cfg.Preferences.FallbackModels))
	candidates = append(candidates, primary)
	seen := map[string]bool{primary.Name: true}
	for _, name := range cfg.Preferences.FallbackModels {
		if seen[name] {
			continue
		}
		if model, ok := findModel(cfg, name); ok {
			candidates = append(candidates, model)
			seen[name] = true
		}
	}
	return candidates
}

func (s *QueryService) persistHistory(req domain.QueryRequest, model domain.ModelDefinition, resp domain.QueryResponse, exec *domain.ExecutionResult) {
	if s.HistoryStore == nil {
		return
	}
	record := domain.HistoryRecord{
		Timestamp: timeNow(),
		Prompt:    req.Prompt,
		Command:   resp.Command,
		Model:     firstNonEmpty(resp.ModelUsed, model.Name),
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Compile-time interface compliance check
var _ domain.QueryService = (*QueryService)(nil)

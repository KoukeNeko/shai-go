package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
)

// Validate ensures config structure is consistent.
func Validate(cfg domain.Config) error {
	if cfg.ConfigFormatVersion == "" {
		cfg.ConfigFormatVersion = "1"
	}
	if len(cfg.Models) == 0 {
		return errors.New("at least one model must be configured")
	}
	if cfg.Preferences.DefaultModel == "" {
		cfg.Preferences.DefaultModel = cfg.Models[0].Name
	}
	if _, ok := findModel(cfg, cfg.Preferences.DefaultModel); !ok {
		return fmt.Errorf("default model %s not found in models list", cfg.Preferences.DefaultModel)
	}
	for _, name := range cfg.Preferences.FallbackModels {
		if _, ok := findModel(cfg, name); !ok {
			return fmt.Errorf("fallback model %s not found", name)
		}
	}
	if err := validateContext(cfg.Context); err != nil {
		return err
	}
	if err := validateSecurity(cfg.Security); err != nil {
		return err
	}
	if err := validateCache(cfg.Cache); err != nil {
		return err
	}
	if err := validateHistory(cfg.History); err != nil {
		return err
	}
	return nil
}

func validateContext(ctx domain.ContextSettings) error {
	switch strings.ToLower(ctx.IncludeGit) {
	case "", "auto":
		ctx.IncludeGit = "auto"
	case "always", "never":
	default:
		return fmt.Errorf("context.include_git must be auto|always|never, got %s", ctx.IncludeGit)
	}
	switch strings.ToLower(ctx.IncludeK8s) {
	case "", "auto":
		ctx.IncludeK8s = "auto"
	case "always", "never":
	default:
		return fmt.Errorf("context.include_k8s must be auto|always|never, got %s", ctx.IncludeK8s)
	}
	if ctx.MaxFiles <= 0 {
		return fmt.Errorf("context.max_files must be > 0")
	}
	return nil
}

func validateSecurity(sec domain.SecuritySettings) error {
	if sec.RulesFile == "" {
		return fmt.Errorf("security.rules_file must be set")
	}
	return nil
}

func validateCache(cache domain.CacheSettings) error {
	if cache.TTL == "" {
		cache.TTL = "1h"
	}
	if _, err := time.ParseDuration(cache.TTL); err != nil {
		return fmt.Errorf("cache.ttl invalid: %w", err)
	}
	if cache.MaxEntries <= 0 {
		return fmt.Errorf("cache.max_entries must be > 0")
	}
	return nil
}

func validateHistory(history domain.HistorySettings) error {
	if history.RetentionDays < 0 {
		return fmt.Errorf("history.retention_days must be >= 0")
	}
	return nil
}

func findModel(cfg domain.Config, name string) (domain.ModelDefinition, bool) {
	for _, model := range cfg.Models {
		if model.Name == name {
			return model, true
		}
	}
	return domain.ModelDefinition{}, false
}

package config

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// FileLoader loads YAML configuration from ~/.shai/config.yaml (overridable via SHAI_CONFIG).
type FileLoader struct {
	overridePath string
}

// NewFileLoader builds a new loader.
func NewFileLoader(path string) *FileLoader {
	return &FileLoader{overridePath: path}
}

// Load implements ports.ConfigProvider.
func (l *FileLoader) Load(context.Context) (domain.Config, error) {
	path := l.resolvePath()
	if err := ensureConfigDir(path); err != nil {
		return domain.Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg := defaultConfig()
			if err := writeDefault(path, cfg); err != nil {
				return domain.Config{}, err
			}
			return cfg, nil
		}
		return domain.Config{}, err
	}

	var cfg domain.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.Config{}, err
	}

	return hydrateDefaults(cfg), nil
}

func (l *FileLoader) resolvePath() string {
	if l.overridePath != "" {
		return l.overridePath
	}
	if custom := os.Getenv("SHAI_CONFIG"); custom != "" {
		return expandPath(custom)
	}
	return filepath.Join(userHomeDir(), ".shai", "config.yaml")
}

func ensureConfigDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0o755)
}

func writeDefault(path string, cfg domain.Config) error {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func defaultConfig() domain.Config {
	return domain.Config{
		ConfigFormatVersion: "1",
		Preferences: domain.Preferences{
			DefaultModel:    "claude-sonnet-4",
			AutoExecuteSafe: false,
			PreviewMode:     "always",
			TimeoutSeconds:  30,
		},
		Context: domain.ContextSettings{
			IncludeFiles: true,
			MaxFiles:     20,
			IncludeGit:   "auto",
			IncludeK8s:   "auto",
			IncludeEnv:   false,
		},
		Security: domain.SecuritySettings{
			Enabled:   true,
			RulesFile: filepath.Join(userHomeDir(), ".shai", "guardrail.yaml"),
		},
		Execution: domain.ExecutionSettings{
			Shell:                "auto",
			ConfirmBeforeExecute: true,
		},
		Models: []domain.ModelDefinition{
			{
				Name:       "claude-sonnet-4",
				Endpoint:   "https://api.anthropic.com/v1/messages",
				AuthEnvVar: "ANTHROPIC_API_KEY",
				ModelID:    "claude-3-5-sonnet-20240620",
				MaxTokens:  1024,
			},
		},
	}
}

func hydrateDefaults(cfg domain.Config) domain.Config {
	if cfg.Preferences.DefaultModel == "" && len(cfg.Models) > 0 {
		cfg.Preferences.DefaultModel = cfg.Models[0].Name
	}
	if cfg.Preferences.TimeoutSeconds == 0 {
		cfg.Preferences.TimeoutSeconds = 30
	}
	if cfg.Context.MaxFiles == 0 {
		cfg.Context.MaxFiles = 20
	}
	return cfg
}

func expandPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if len(path) > 1 && path[:2] == "~/" {
		return filepath.Join(userHomeDir(), path[2:])
	}
	return filepath.Clean(path)
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.ConfigProvider = (*FileLoader)(nil)

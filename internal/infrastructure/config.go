package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/assets"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/pkg/filesystem"
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
		return domain.Config{}, fmt.Errorf("ensure config dir: %w", err)
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
	return filepath.Join(filesystem.UserHomeDir(), ".shai", "config.yaml")
}

func ensureConfigDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, domain.DirectoryPermissions)
}

func writeDefault(path string, cfg domain.Config) error {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, domain.SecureFilePermissions)
}

// Path returns the resolved config file path.
func (l *FileLoader) Path() string {
	return l.resolvePath()
}

// Save writes the given config back to disk.
func (l *FileLoader) Save(cfg domain.Config) error {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := ensureConfigDir(l.resolvePath()); err != nil {
		return err
	}
	return os.WriteFile(l.resolvePath(), raw, domain.SecureFilePermissions)
}

// Reset overwrites the config with defaults and returns the default snapshot.
func (l *FileLoader) Reset() (domain.Config, error) {
	cfg := defaultConfig()
	if err := l.Save(cfg); err != nil {
		return domain.Config{}, err
	}
	return cfg, nil
}

// Backup copies the current config file to a timestamped backup.
func (l *FileLoader) Backup() (string, error) {
	path := l.resolvePath()
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102T150405"))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, data, domain.SecureFilePermissions); err != nil {
		return "", err
	}
	return backup, nil
}

func defaultConfig() domain.Config {
	var cfg domain.Config
	if err := yaml.Unmarshal(assets.DefaultConfigYAML, &cfg); err != nil {
		// Fallback to minimal config if embedded YAML is corrupted
		// This should never happen in production, but provides safety
		return domain.Config{
			ConfigFormatVersion: "1",
			Preferences: domain.Preferences{
				DefaultModel:    "claude-sonnet-4",
				AutoExecuteSafe: false,
				Verbose:         false,
				TimeoutSeconds:  30,
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

	// Expand user home directory in security rules file path
	if cfg.Security.RulesFile == "~/.shai/guardrail.yaml" {
		cfg.Security.RulesFile = filepath.Join(filesystem.UserHomeDir(), ".shai", "guardrail.yaml")
	}

	return cfg
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
	for i := range cfg.Models {
		if len(cfg.Models[i].Prompt) == 0 {
			cfg.Models[i].Prompt = defaultPromptMessages()
		}
	}
	if cfg.Cache.TTL == "" {
		cfg.Cache.TTL = "1h"
	}
	if cfg.Cache.MaxEntries <= 0 {
		cfg.Cache.MaxEntries = 100
	}
	if cfg.History.RetentionDays < 0 {
		cfg.History.RetentionDays = 0
	}
	return cfg
}


func expandPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if len(path) > 1 && path[:2] == "~/" {
		return filepath.Join(filesystem.UserHomeDir(), path[2:])
	}
	return filepath.Clean(path)
}

var _ ports.ConfigProvider = (*FileLoader)(nil)

func defaultPromptMessages() []domain.PromptMessage {
	return []domain.PromptMessage{
		{
			Role: "system",
			Content: `You are SHAI, a cautious shell assistant.
Always output a single shell command (with optional short explanation).
Current environment:
- Directory: {{.WorkingDir}}
- Shell: {{.Shell}}
- OS: {{.OS}}
{{if .AvailableTools}}- Tools: {{.AvailableTools}}{{end}}
{{if .GitStatus}}- Git: {{.GitStatus}}{{end}}
{{if .K8sNamespace}}- Kubernetes: {{.K8sContext}}/{{.K8sNamespace}}{{end}}`,
		},
		{
			Role:    "user",
			Content: "{{.Prompt}}",
		},
	}
}

// DefaultConfig exposes the bootstrap configuration template.
func DefaultConfig() domain.Config {
	return defaultConfig()
}

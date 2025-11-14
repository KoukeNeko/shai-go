// Package domain defines core business entities and value objects for SHAI.
//
// This file contains configuration structures that map directly to the YAML
// configuration file (~/.shai/config.yaml). These structures define how users
// customize SHAI's behavior, security, caching, and AI provider settings.
package domain

// Config represents the complete SHAI configuration loaded from ~/.shai/config.yaml.
// It encompasses all user preferences, model definitions, and behavioral settings.
type Config struct {
	ConfigFormatVersion string            `yaml:"config_format_version"`
	Preferences         Preferences       `yaml:"preferences"`
	Models              []ModelDefinition `yaml:"models"`
	Context             ContextSettings   `yaml:"context"`
	Security            SecuritySettings  `yaml:"security"`
	Execution           ExecutionSettings `yaml:"execution"`
	Cache               CacheSettings     `yaml:"cache"`
	History             HistorySettings   `yaml:"history"`
}

// Preferences contains user-level behavioral settings and toggles.
// These settings control the default model, execution behavior, and fallback strategies.
type Preferences struct {
	DefaultModel    string   `yaml:"default_model"`
	AutoExecuteSafe bool     `yaml:"auto_execute_safe"`
	PreviewMode     string   `yaml:"preview_mode"`
	Verbose         bool     `yaml:"verbose"`
	TimeoutSeconds  int      `yaml:"timeout"`
	FallbackModels  []string `yaml:"fallback_models"`
}

// ContextSettings configures what environmental context is collected and sent to AI.
// This controls whether git status, kubernetes info, files, and environment variables
// are included in prompts to provide better contextual awareness.
type ContextSettings struct {
	IncludeFiles bool   `yaml:"include_files"`
	MaxFiles     int    `yaml:"max_files"`
	IncludeGit   string `yaml:"include_git"`
	IncludeK8s   string `yaml:"include_k8s"`
	IncludeEnv   bool   `yaml:"include_env"`
}

// SecuritySettings defines security guardrail behavior to prevent dangerous commands.
// When enabled, commands are checked against rules before execution.
type SecuritySettings struct {
	Enabled   bool   `yaml:"enabled"`
	RulesFile string `yaml:"rules_file"`
}

// ExecutionSettings controls how generated commands are executed.
// This includes shell selection and whether user confirmation is required.
type ExecutionSettings struct {
	Shell                string `yaml:"shell"`
	ConfirmBeforeExecute bool   `yaml:"confirm_before_execute"`
}

// CacheSettings configures response caching to reduce API calls and improve performance.
// Cached responses are stored locally with a time-to-live and maximum entry limit.
type CacheSettings struct {
	TTL        string `yaml:"ttl"`
	MaxEntries int    `yaml:"max_entries"`
}

// HistorySettings controls command history persistence and retention.
// History allows users to review and reuse previous AI-generated commands.
type HistorySettings struct {
	RetentionDays int `yaml:"retention_days"`
}

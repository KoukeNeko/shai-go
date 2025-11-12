package domain

// Config mirrors ~/.shai/config.yaml.
type Config struct {
	ConfigFormatVersion string            `yaml:"config_format_version"`
	Preferences         Preferences       `yaml:"preferences"`
	Models              []ModelDefinition `yaml:"models"`
	Context             ContextSettings   `yaml:"context"`
	Security            SecuritySettings  `yaml:"security"`
	Execution           ExecutionSettings `yaml:"execution"`
}

// Preferences captures user level toggles.
type Preferences struct {
	DefaultModel    string `yaml:"default_model"`
	AutoExecuteSafe bool   `yaml:"auto_execute_safe"`
	PreviewMode     string `yaml:"preview_mode"`
	TimeoutSeconds  int    `yaml:"timeout"`
}

// ContextSettings configures context collection.
type ContextSettings struct {
	IncludeFiles bool   `yaml:"include_files"`
	MaxFiles     int    `yaml:"max_files"`
	IncludeGit   string `yaml:"include_git"`
	IncludeK8s   string `yaml:"include_k8s"`
	IncludeEnv   bool   `yaml:"include_env"`
}

// SecuritySettings defines guardrail behavior.
type SecuritySettings struct {
	Enabled   bool   `yaml:"enabled"`
	RulesFile string `yaml:"rules_file"`
}

// ExecutionSettings controls how commands run.
type ExecutionSettings struct {
	Shell                string `yaml:"shell"`
	ConfirmBeforeExecute bool   `yaml:"confirm_before_execute"`
}

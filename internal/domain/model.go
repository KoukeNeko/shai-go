package domain

// ModelDefinition describes an AI provider option declared in configuration.
type ModelDefinition struct {
	Name       string          `yaml:"name"`
	Endpoint   string          `yaml:"endpoint"`
	AuthEnvVar string          `yaml:"auth_env_var"`
	OrgEnvVar  string          `yaml:"org_env_var"`
	ModelID    string          `yaml:"model_id"`
	MaxTokens  int             `yaml:"max_tokens"`
	Prompt     []PromptMessage `yaml:"prompt"`
}

// PromptMessage follows the role/content pair required by most chat APIs.
type PromptMessage struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

// ProviderKind derives protocol semantics from endpoint metadata.
type ProviderKind string

const (
	ProviderKindAnthropic ProviderKind = "anthropic"
	ProviderKindOpenAI    ProviderKind = "openai"
	ProviderKindOllama    ProviderKind = "ollama"
	ProviderKindUnknown   ProviderKind = "unknown"
)

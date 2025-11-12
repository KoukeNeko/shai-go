package domain

// RiskLevel enumerates guardrail outcomes.
type RiskLevel string

const (
	RiskSafe     RiskLevel = "safe"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// GuardrailAction describes how the executor should react to a risk level.
type GuardrailAction string

const (
	ActionAllow           GuardrailAction = "allow"
	ActionPreviewOnly     GuardrailAction = "preview_only"
	ActionSimpleConfirm   GuardrailAction = "simple_confirm"
	ActionConfirm         GuardrailAction = "confirm"
	ActionExplicitConfirm GuardrailAction = "explicit_confirm"
	ActionBlock           GuardrailAction = "block"
)

// RiskAssessment aggregates security evaluation data.
type RiskAssessment struct {
	Level          RiskLevel
	Action         GuardrailAction
	Reasons        []string
	ProtectedPaths []string
	MatchedRules   []string
	PreviewEntries []string
}

// GuardrailRules is the in-memory representation of YAML guardrail configuration.
type GuardrailRules struct {
	DangerPatterns []DangerPattern
	ProtectedPaths []ProtectedPath
	Preview        PreviewRules
	Confirmation   map[string]ConfirmationLevel
	Whitelist      []string
}

// DangerPattern is a regex-based rule loaded from YAML.
type DangerPattern struct {
	Pattern string `yaml:"pattern"`
	Level   string `yaml:"level"`
	Message string `yaml:"message"`
	Action  string `yaml:"action"`
}

// ProtectedPath describes operations guarded for a given filesystem path.
type ProtectedPath struct {
	Path       string   `yaml:"path"`
	Operations []string `yaml:"operations"`
	Level      string   `yaml:"level"`
	Action     string   `yaml:"action"`
}

// PreviewRules controls preview listings.
type PreviewRules struct {
	MaxFiles int `yaml:"max_files"`
}

// ConfirmationLevel customizes messaging per risk level.
type ConfirmationLevel struct {
	Action  string `yaml:"action"`
	Message string `yaml:"message"`
}

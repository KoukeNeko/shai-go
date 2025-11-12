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
}

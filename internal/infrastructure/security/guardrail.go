package security

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Guardrail implements the SecurityService port.
type Guardrail struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	re   *regexp.Regexp
	rule DangerPattern
}

// DangerPattern describes a regex-based guardrail rule.
type DangerPattern struct {
	Pattern string `yaml:"pattern"`
	Level   string `yaml:"level"`
	Message string `yaml:"message"`
	Action  string `yaml:"action"`
}

// RulesFile is the YAML schema root.
type RulesFile struct {
	Rules struct {
		DangerPatterns []DangerPattern `yaml:"danger_patterns"`
	} `yaml:"rules"`
}

// NewGuardrail loads guardrail rules from disk (or defaults when missing).
func NewGuardrail(path string) (*Guardrail, error) {
	rules, err := loadRules(path)
	if err != nil {
		return nil, err
	}

	var compiled []compiledPattern
	for _, pattern := range rules.Rules.DangerPatterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, compiledPattern{
			re:   re,
			rule: pattern,
		})
	}

	return &Guardrail{patterns: compiled}, nil
}

// Evaluate implements ports.SecurityService.
func (g *Guardrail) Evaluate(command string) (domain.RiskAssessment, error) {
	if g == nil {
		return domain.RiskAssessment{}, errors.New("guardrail nil")
	}
	assessment := domain.RiskAssessment{
		Level:  domain.RiskSafe,
		Action: domain.ActionAllow,
	}
	highest := domain.RiskSafe
	for _, pattern := range g.patterns {
		if pattern.re.MatchString(command) {
			ruleLevel := parseRiskLevel(pattern.rule.Level)
			if moreSevere(ruleLevel, highest) {
				highest = ruleLevel
				assessment.Level = ruleLevel
				assessment.Action = parseAction(pattern.rule.Action, ruleLevel)
			}
			assessment.Reasons = append(assessment.Reasons, pattern.rule.Message)
			assessment.MatchedRules = append(assessment.MatchedRules, pattern.rule.Pattern)
		}
	}
	return assessment, nil
}

func loadRules(path string) (RulesFile, error) {
	var rules RulesFile
	path = expandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		// fall back to defaults
		rules.Rules.DangerPatterns = defaultPatterns()
		return rules, nil
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return RulesFile{}, err
	}
	if len(rules.Rules.DangerPatterns) == 0 {
		rules.Rules.DangerPatterns = defaultPatterns()
	}
	return rules, nil
}

func parseRiskLevel(value string) domain.RiskLevel {
	switch strings.ToLower(value) {
	case "low":
		return domain.RiskLow
	case "medium":
		return domain.RiskMedium
	case "high":
		return domain.RiskHigh
	case "critical":
		return domain.RiskCritical
	default:
		return domain.RiskSafe
	}
}

func parseAction(value string, fallback domain.RiskLevel) domain.GuardrailAction {
	switch strings.ToLower(value) {
	case "preview_only":
		return domain.ActionPreviewOnly
	case "simple_confirm":
		return domain.ActionSimpleConfirm
	case "confirm":
		return domain.ActionConfirm
	case "explicit_confirm":
		return domain.ActionExplicitConfirm
	case "block":
		return domain.ActionBlock
	default:
		if fallback == domain.RiskSafe {
			return domain.ActionAllow
		}
		return domain.ActionConfirm
	}
}

func moreSevere(next domain.RiskLevel, current domain.RiskLevel) bool {
	order := map[domain.RiskLevel]int{
		domain.RiskSafe:     0,
		domain.RiskLow:      1,
		domain.RiskMedium:   2,
		domain.RiskHigh:     3,
		domain.RiskCritical: 4,
	}
	return order[next] > order[current]
}

func expandPath(path string) string {
	if path == "" {
		return filepath.Join(userHomeDir(), ".shai", "guardrail.yaml")
	}
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(userHomeDir(), path[2:])
	}
	return filepath.Join(userHomeDir(), path)
}

func defaultPatterns() []DangerPattern {
	return []DangerPattern{
		{Pattern: `rm\s+-rf\s+/`, Level: "critical", Message: "Deleting root directory", Action: "block"},
		{Pattern: `rm\s+-rf\s+\*`, Level: "critical", Message: "Recursive delete everything", Action: "explicit_confirm"},
		{Pattern: `dd\s+if=`, Level: "critical", Message: "Raw disk writing", Action: "block"},
		{Pattern: `mkfs\.`, Level: "critical", Message: "Formatting filesystem", Action: "block"},
		{Pattern: `> /dev/(sd[a-z]|nvme)`, Level: "critical", Message: "Writing to block device", Action: "block"},
		{Pattern: `chmod\s+777`, Level: "medium", Message: "Overly permissive chmod", Action: "simple_confirm"},
		{Pattern: `curl.*\|\s*sudo`, Level: "high", Message: "Piping remote script to sudo", Action: "confirm"},
		{Pattern: `rm\s+-rf\s+\$HOME`, Level: "high", Message: "Deleting home directory", Action: "explicit_confirm"},
		{Pattern: `:(){ :\|:& };:`, Level: "critical", Message: "Fork bomb", Action: "block"},
	}
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.SecurityService = (*Guardrail)(nil)

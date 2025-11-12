package security

import (
	"errors"
	"fmt"
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
	patterns     []compiledPattern
	pathRules    []domain.ProtectedPath
	previewLimit int
}

type compiledPattern struct {
	re   *regexp.Regexp
	rule domain.DangerPattern
}

// RulesFile is the YAML schema root.
type RulesFile struct {
	Rules struct {
		DangerPatterns []domain.DangerPattern `yaml:"danger_patterns"`
		ProtectedPaths []domain.ProtectedPath `yaml:"protected_paths"`
		Preview        domain.PreviewRules    `yaml:"preview"`
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

	previewLimit := rules.Rules.Preview.MaxFiles
	if previewLimit == 0 {
		previewLimit = 10
	}

	return &Guardrail{
		patterns:     compiled,
		pathRules:    rules.Rules.ProtectedPaths,
		previewLimit: previewLimit,
	}, nil
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

	pathAssessment := g.evaluateProtectedPaths(command)
	if moreSevere(pathAssessment.Level, highest) {
		assessment.Level = pathAssessment.Level
		assessment.Action = pathAssessment.Action
		highest = pathAssessment.Level
	}
	assessment.Reasons = append(assessment.Reasons, pathAssessment.Reasons...)
	assessment.ProtectedPaths = append(assessment.ProtectedPaths, pathAssessment.ProtectedPaths...)
	assessment.PreviewEntries = append(assessment.PreviewEntries, pathAssessment.PreviewEntries...)

	return assessment, nil
}

func loadRules(path string) (RulesFile, error) {
	var rules RulesFile
	path = expandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		// fall back to defaults
		rules.Rules.DangerPatterns = defaultPatterns()
		rules.Rules.ProtectedPaths = defaultProtectedPaths()
		rules.Rules.Preview = domain.PreviewRules{MaxFiles: 10}
		return rules, nil
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return RulesFile{}, err
	}
	if len(rules.Rules.DangerPatterns) == 0 {
		rules.Rules.DangerPatterns = defaultPatterns()
	}
	if len(rules.Rules.ProtectedPaths) == 0 {
		rules.Rules.ProtectedPaths = defaultProtectedPaths()
	}
	if rules.Rules.Preview.MaxFiles == 0 {
		rules.Rules.Preview.MaxFiles = 10
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

func defaultPatterns() []domain.DangerPattern {
	return []domain.DangerPattern{
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

func defaultProtectedPaths() []domain.ProtectedPath {
	return []domain.ProtectedPath{
		{Path: "/", Operations: []string{"rm", "mv", "chmod", "chown"}, Level: "critical", Action: "block"},
		{Path: "/etc", Operations: []string{"rm", "mv"}, Level: "high", Action: "explicit_confirm"},
		{Path: "/usr", Operations: []string{"rm"}, Level: "high", Action: "explicit_confirm"},
		{Path: "$HOME", Operations: []string{"rm -rf"}, Level: "high", Action: "explicit_confirm"},
	}
}

func (g *Guardrail) evaluateProtectedPaths(command string) domain.RiskAssessment {
	result := domain.RiskAssessment{
		Level:  domain.RiskSafe,
		Action: domain.ActionAllow,
	}
	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		return result
	}
	for _, rule := range g.pathRules {
		if matchesPathRule(tokens, rule) {
			level := parseRiskLevel(rule.Level)
			if moreSevere(level, result.Level) {
				result.Level = level
				result.Action = parseAction(rule.Action, level)
			}
			result.Reasons = append(result.Reasons, fmt.Sprintf("Operation on protected path %s", rule.Path))
			result.ProtectedPaths = append(result.ProtectedPaths, rule.Path)
			preview := previewPath(rule.Path, g.previewLimit)
			result.PreviewEntries = append(result.PreviewEntries, preview...)
		}
	}
	return result
}

func matchesPathRule(tokens []string, rule domain.ProtectedPath) bool {
	if len(rule.Operations) == 0 {
		return false
	}
	command := strings.Join(tokens, " ")
	path := rule.Path
	if path == "$HOME" {
		path = os.Getenv("HOME")
	}
	if path == "" {
		return false
	}
	for _, op := range rule.Operations {
		if strings.Contains(command, op) && strings.Contains(command, path) {
			return true
		}
		if len(tokens) > 0 && tokens[0] == op && strings.Contains(command, path) {
			return true
		}
	}
	return false
}

func previewPath(path string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	list := []string{}
	resolved := path
	if strings.HasPrefix(path, "$HOME") {
		resolved = strings.Replace(path, "$HOME", os.Getenv("HOME"), 1)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return nil
		}
		for i, entry := range entries {
			if i >= limit {
				break
			}
			list = append(list, filepath.Join(path, entry.Name()))
		}
	} else {
		list = append(list, path)
	}
	return list
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.SecurityService = (*Guardrail)(nil)

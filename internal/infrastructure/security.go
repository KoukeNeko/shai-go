package infrastructure

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
	confirmation map[domain.RiskLevel]domain.ConfirmationLevel
	whitelist    []string
}

type compiledPattern struct {
	re   *regexp.Regexp
	rule domain.DangerPattern
}

// PolicyDocument is the YAML schema root.
type PolicyDocument struct {
	Rules struct {
		DangerPatterns []domain.DangerPattern              `yaml:"danger_patterns"`
		ProtectedPaths []domain.ProtectedPath              `yaml:"protected_paths"`
		Preview        domain.PreviewRules                 `yaml:"preview"`
		Confirmation   map[string]domain.ConfirmationLevel `yaml:"confirmation_levels"`
		Whitelist      []string                            `yaml:"whitelist"`
	} `yaml:"rules"`
}

// NewGuardrail loads guardrail rules from disk (or defaults when missing).
func NewGuardrail(path string) (*Guardrail, error) {
	doc, err := loadRules(path)
	if err != nil {
		return nil, err
	}

	var compiled []compiledPattern
	for _, pattern := range doc.Rules.DangerPatterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, compiledPattern{
			re:   re,
			rule: pattern,
		})
	}

	previewLimit := doc.Rules.Preview.MaxFiles
	if previewLimit == 0 {
		previewLimit = 10
	}

	confirmation := map[domain.RiskLevel]domain.ConfirmationLevel{}
	for level, config := range doc.Rules.Confirmation {
		confirmation[parseRiskLevel(level)] = config
	}

	return &Guardrail{
		patterns:     compiled,
		pathRules:    doc.Rules.ProtectedPaths,
		previewLimit: previewLimit,
		confirmation: confirmation,
		whitelist:    doc.Rules.Whitelist,
	}, nil
}

// Evaluate implements ports.SecurityService.
func (g *Guardrail) Evaluate(command string) (domain.RiskAssessment, error) {
	if g == nil {
		return domain.RiskAssessment{}, errors.New("guardrail nil")
	}
	command = strings.TrimSpace(command)
	if g.isWhitelisted(command) {
		return domain.RiskAssessment{
			Level:  domain.RiskSafe,
			Action: domain.ActionAllow,
		}, nil
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
	enrichAssessment(command, &assessment)

	if levelConfig, ok := g.confirmation[assessment.Level]; ok {
		assessment.Action = parseAction(levelConfig.Action, assessment.Level)
		if levelConfig.Message != "" {
			assessment.Reasons = append(assessment.Reasons, levelConfig.Message)
		}
	}

	return assessment, nil
}

func loadRules(path string) (PolicyDocument, error) {
	var rules PolicyDocument
	path = securityExpandPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		// fall back to defaults
		rules.Rules.DangerPatterns = defaultPatterns()
		rules.Rules.ProtectedPaths = defaultProtectedPaths()
		rules.Rules.Preview = domain.PreviewRules{MaxFiles: 10}
		return rules, nil
	}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return PolicyDocument{}, err
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
	if len(rules.Rules.Confirmation) == 0 {
		rules.Rules.Confirmation = defaultConfirmation()
	}
	if len(rules.Rules.Whitelist) == 0 {
		rules.Rules.Whitelist = defaultWhitelist()
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

func securityExpandPath(path string) string {
	if path == "" {
		return filepath.Join(securityUserHome(), ".shai", "guardrail.yaml")
	}
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(securityUserHome(), path[2:])
	}
	return filepath.Join(securityUserHome(), path)
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

func defaultConfirmation() map[string]domain.ConfirmationLevel {
	return map[string]domain.ConfirmationLevel{
		"critical": {Action: "block", Message: "This action is blocked by guardrail policy."},
		"high":     {Action: "explicit_confirm", Message: "Type 'yes' to execute this high-risk operation."},
		"medium":   {Action: "confirm", Message: "Review the command carefully before continuing."},
		"low":      {Action: "simple_confirm", Message: "Confirm execution of this low-risk change."},
	}
}

func defaultWhitelist() []string {
	return []string{"ls", "pwd", "echo", "cat", "grep", "find", "git status"}
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

func (g *Guardrail) isWhitelisted(command string) bool {
	for _, safe := range g.whitelist {
		if safe == "" {
			continue
		}
		if command == safe || strings.HasPrefix(command, safe+" ") {
			return true
		}
	}
	return false
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

func enrichAssessment(command string, assessment *domain.RiskAssessment) {
	if assessment.Level == domain.RiskSafe {
		return
	}
	if assessment.DryRunCommand == "" {
		if dry := suggestDryRunCommand(command); dry != "" {
			assessment.DryRunCommand = dry
		}
	}
	assessment.UndoHints = append(assessment.UndoHints, undoHintsForCommand(command)...)
}

func suggestDryRunCommand(command string) string {
	lower := strings.ToLower(command)
	switch {
	case strings.Contains(lower, "kubectl apply") && !strings.Contains(lower, "--dry-run"):
		return command + " --dry-run=client"
	case strings.HasPrefix(lower, "git ") && !strings.Contains(lower, "status"):
		return "git status"
	case strings.HasPrefix(lower, "rm "):
		parts := strings.SplitN(command, " ", 2)
		if len(parts) == 2 {
			return "ls " + parts[1]
		}
	}
	return ""
}

func undoHintsForCommand(command string) []string {
	lower := strings.ToLower(command)
	var hints []string
	if strings.Contains(lower, "git ") {
		hints = append(hints, "Use `git status` and `git reflog` to inspect or roll back changes.")
	}
	if strings.Contains(lower, "kubectl ") {
		hints = append(hints, "Use `kubectl rollout undo` or `kubectl get events` if deployment behaves unexpectedly.")
	}
	if strings.Contains(lower, "rm ") {
		hints = append(hints, "Restore from backups or use `git checkout -- <path>` if the file was tracked.")
	}
	return hints
}

func securityUserHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.SecurityService = (*Guardrail)(nil)

// LoadPolicyDocument returns the raw YAML structure.
func LoadPolicyDocument(path string) (PolicyDocument, error) {
	return loadRules(path)
}

// SavePolicyDocument writes the YAML structure to disk.
func SavePolicyDocument(path string, doc PolicyDocument) error {
	path = securityExpandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ResolveRulesPath expands the guardrail path to an absolute location.
func ResolveRulesPath(path string) string {
	return securityExpandPath(path)
}

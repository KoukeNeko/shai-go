package helpers

import (
	"bufio"
	"fmt"
	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure"
	"github.com/doeshing/shai-go/internal/ports"
	"github.com/doeshing/shai-go/internal/services"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ====================================================================================
// Config Helpers
// ====================================================================================

// GetConfigLoader extracts the config loader from container with error handling
func GetConfigLoader(container *app.Container) (*configinfra.FileLoader, error) {
	if container.ConfigLoader == nil {
		return nil, fmt.Errorf("config loader unavailable")
	}
	return container.ConfigLoader, nil
}

// SaveConfigWithValidation validates and saves configuration with automatic backup
func SaveConfigWithValidation(container *app.Container, cfg domain.Config) error {
	loader, err := GetConfigLoader(container)
	if err != nil {
		return err
	}

	if err := services.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if err := createBackupIfExists(loader); err != nil {
		return err
	}

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// createBackupIfExists creates a backup of the config file if it exists
func createBackupIfExists(loader *configinfra.FileLoader) error {
	if _, err := os.Stat(loader.Path()); err == nil {
		if _, err := loader.Backup(); err != nil {
			return fmt.Errorf("failed to create configuration backup: %w", err)
		}
	}
	return nil
}

// ParseYAMLValue parses a string value as YAML, falling back to literal string
func ParseYAMLValue(input string) (interface{}, error) {
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(input), &parsed); err != nil {
		// If YAML parsing fails, treat as literal string
		return input, nil
	}
	return parsed, nil
}

// SetNestedMapValue sets a value in a nested map using a key path
// Returns true if successful, false otherwise
func SetNestedMapValue(root map[string]interface{}, keyPath []string, value interface{}) bool {
	if len(keyPath) == 0 {
		return false
	}

	current := root
	for i := 0; i < len(keyPath)-1; i++ {
		key := keyPath[i]
		next, exists := current[key]

		if !exists {
			newChild := map[string]interface{}{}
			current[key] = newChild
			current = newChild
			continue
		}

		child, isMap := next.(map[string]interface{})
		if !isMap {
			// Overwrite non-map value with new map
			child = map[string]interface{}{}
			current[key] = child
		}
		current = child
	}

	current[keyPath[len(keyPath)-1]] = value
	return true
}

// TraverseNestedMap retrieves a value from a nested map using a key path
// Returns the value and true if found, nil and false otherwise
func TraverseNestedMap(data interface{}, keyPath []string) (interface{}, bool) {
	if len(keyPath) == 0 {
		return data, true
	}

	switch node := data.(type) {
	case map[string]interface{}:
		next, exists := node[keyPath[0]]
		if !exists {
			return nil, false
		}
		return TraverseNestedMap(next, keyPath[1:])
	default:
		return nil, false
	}
}

// LoadPromptMessagesFromFile reads and parses a YAML file containing prompt messages
func LoadPromptMessagesFromFile(filepath string) ([]domain.PromptMessage, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file %s: %w", filepath, err)
	}

	var prompts []domain.PromptMessage
	if err := yaml.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("failed to parse prompt file %s: %w", filepath, err)
	}

	return prompts, nil
}

// ====================================================================================
// Prompt Helpers
// ====================================================================================

// PromptForChoice prompts the user to select from a list of options
// Returns the user's choice or the default value if no input is provided
func PromptForChoice(out io.Writer, reader *bufio.Reader, promptText string, defaultValue string) string {
	fmt.Fprintf(out, "%s [%s]: ", promptText, defaultValue)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultValue
	}
	return line
}

// PromptForYesNo prompts the user for a yes/no question
// Returns true for yes, false for no, or the default value if no input
func PromptForYesNo(out io.Writer, reader *bufio.Reader, promptText string, defaultValue bool) bool {
	label := buildYesNoLabel(defaultValue)
	fmt.Fprintf(out, "%s [%s]: ", promptText, label)

	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))

	if line == "" {
		return defaultValue
	}

	return isAffirmativeResponse(line)
}

// PromptForString prompts the user for a string input with an optional default value
func PromptForString(out io.Writer, reader *bufio.Reader, promptText string, defaultValue string) string {
	fmt.Fprintf(out, "%s ", promptText)

	if defaultValue != "" {
		fmt.Fprintf(out, "(default: %s)", defaultValue)
	}

	fmt.Fprint(out, ": ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultValue
	}
	return line
}

// PromptForConfirmation asks the user to confirm an action
// Returns true if the user confirms, false otherwise
func PromptForConfirmation(out io.Writer, reader *bufio.Reader, question string) bool {
	return PromptForYesNo(out, reader, question, false)
}

// buildYesNoLabel constructs the appropriate y/N or Y/n label based on the default
func buildYesNoLabel(defaultIsYes bool) string {
	if defaultIsYes {
		return "Y/n"
	}
	return "y/N"
}

// isAffirmativeResponse checks if a response is affirmative (yes)
func isAffirmativeResponse(response string) bool {
	return response == "y" || response == "yes"
}

// PrintWarnings outputs a list of warning messages to the writer
func PrintWarnings(out io.Writer, warnings []string) {
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		fmt.Fprintf(out, "Warning: %s\n", warning)
	}
}

// ====================================================================================
// Shell Helpers
// ====================================================================================

const (
	shellAutoDetect = "auto"
	shellAll        = "all"
)

// DetermineTargetShells resolves which shells to operate on based on the flag value
// Returns a list of shell names to target
func DetermineTargetShells(shellFlag string, integrator ports.ShellIntegrator) ([]domain.ShellName, error) {
	normalizedFlag := normalizeShellFlag(shellFlag)

	switch normalizedFlag {
	case "", shellAutoDetect:
		return autoDetectShells(integrator), nil
	case shellAll:
		return allSupportedShells(), nil
	default:
		return parseSingleShell(normalizedFlag)
	}
}

// autoDetectShells attempts to detect the current shell, falling back to all shells
func autoDetectShells(integrator ports.ShellIntegrator) []domain.ShellName {
	detectedShell := ParseShellName(integrator.DetectShell())

	if detectedShell != domain.ShellUnknown {
		return []domain.ShellName{detectedShell}
	}

	// Fallback to all supported shells if detection fails
	return allSupportedShells()
}

// allSupportedShells returns all shells currently supported by the application
func allSupportedShells() []domain.ShellName {
	return []domain.ShellName{domain.ShellZsh, domain.ShellBash}
}

// parseSingleShell parses a single shell name from a flag value
func parseSingleShell(value string) ([]domain.ShellName, error) {
	shellName := ParseShellName(value)

	if shellName == domain.ShellUnknown {
		return nil, fmt.Errorf("unsupported shell: %s", value)
	}

	return []domain.ShellName{shellName}, nil
}

// ParseShellName converts a string to a ShellName constant
// Handles both simple names and full paths (e.g., "/bin/zsh" -> "zsh")
func ParseShellName(value string) domain.ShellName {
	// Extract basename and normalize
	normalized := normalizeShellName(value)

	switch normalized {
	case "zsh":
		return domain.ShellZsh
	case "bash":
		return domain.ShellBash
	default:
		return domain.ShellUnknown
	}
}

// normalizeShellFlag normalizes a shell flag value
func normalizeShellFlag(flag string) string {
	return strings.ToLower(strings.TrimSpace(flag))
}

// normalizeShellName extracts the basename and normalizes a shell name
func normalizeShellName(value string) string {
	basename := filepath.Base(value)
	return strings.ToLower(strings.TrimSpace(basename))
}

// ====================================================================================
// Stats Helpers
// ====================================================================================

// CommandStatistic represents usage statistics for a command
type CommandStatistic struct {
	Command string
	Count   int
}

// CalculateTopCommands returns the top N most frequently used commands
// If limit is 0 or negative, returns all commands
func CalculateTopCommands(commandFrequency map[string]int, limit int) []CommandStatistic {
	stats := convertFrequencyMapToStatistics(commandFrequency)
	sortStatisticsByFrequency(stats)

	if shouldLimitResults(limit, len(stats)) {
		return stats[:limit]
	}
	return stats
}

// convertFrequencyMapToStatistics converts a map to a slice of CommandStatistic
func convertFrequencyMapToStatistics(frequency map[string]int) []CommandStatistic {
	stats := make([]CommandStatistic, 0, len(frequency))
	for cmd, count := range frequency {
		stats = append(stats, CommandStatistic{
			Command: cmd,
			Count:   count,
		})
	}
	return stats
}

// sortStatisticsByFrequency sorts statistics by count (descending) then by command name (ascending)
func sortStatisticsByFrequency(stats []CommandStatistic) {
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Command < stats[j].Command
		}
		return stats[i].Count > stats[j].Count
	})
}

// shouldLimitResults checks if we should limit the results based on the limit and actual length
func shouldLimitResults(limit int, actualLength int) bool {
	return limit > 0 && actualLength > limit
}

// CalculateSuccessRate calculates the success rate as a percentage
func CalculateSuccessRate(successfulCount int, executedCount int) float64 {
	if executedCount == 0 {
		return 0.0
	}
	return float64(successfulCount) / float64(executedCount) * 100.0
}

// DeriveUndoHints generates undo hints based on command history
// Returns a sorted list of unique hints
func DeriveUndoHints(records []domain.HistoryRecord) []string {
	hintMap := make(map[string]string)

	for _, record := range records {
		normalizedCommand := strings.ToLower(record.Command)
		addHintIfApplicable(hintMap, normalizedCommand)
	}

	return convertHintMapToSortedList(hintMap)
}

// addHintIfApplicable adds a hint to the map if the command matches known patterns
func addHintIfApplicable(hintMap map[string]string, command string) {
	hints := map[string]struct {
		prefix string
		hint   string
	}{
		"git": {
			prefix: "git ",
			hint:   "Use `git status`, `git reflog`, or `git restore` to inspect and undo git changes.",
		},
		"kubectl": {
			prefix: "kubectl ",
			hint:   "Use `kubectl rollout undo` or `kubectl get events` to recover from cluster issues.",
		},
		"rm": {
			prefix: "rm ",
			hint:   "Restore files via backups or `git checkout -- <path>` if tracked.",
		},
		"docker": {
			prefix: "docker ",
			hint:   "Use `docker ps -a` and `docker logs` to review container history before repeating.",
		},
	}

	for key, config := range hints {
		if strings.HasPrefix(command, config.prefix) {
			hintMap[key] = config.hint
		}
	}
}

// convertHintMapToSortedList converts a hint map to a sorted slice
func convertHintMapToSortedList(hintMap map[string]string) []string {
	hints := make([]string, 0, len(hintMap))
	for _, hint := range hintMap {
		hints = append(hints, hint)
	}
	sort.Strings(hints)
	return hints
}

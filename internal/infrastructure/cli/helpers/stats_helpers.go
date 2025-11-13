package helpers

import (
	"sort"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
)

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

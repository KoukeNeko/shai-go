package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
)

const (
	msgNoHistoryRecorded = "No history recorded yet."
)

// NewHistoryCommand creates the history command with all subcommands
func NewHistoryCommand(container *app.Container) *cobra.Command {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect SHAI history",
	}

	historyCmd.AddCommand(
		newHistoryListCommand(container),
		newHistorySearchCommand(container),
		newHistoryClearCommand(container),
		newHistoryExportCommand(container),
		newHistoryStatsCommand(container),
		newHistoryRetainCommand(container),
	)

	return historyCmd
}

// newHistoryListCommand creates the 'history list' subcommand
func newHistoryListCommand(container *app.Container) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent history entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listHistoryEntries(cmd.OutOrStdout(), container, limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", DefaultHistoryLimit, "Max entries to show")
	return cmd
}

// newHistorySearchCommand creates the 'history search' subcommand
func newHistorySearchCommand(container *app.Container) *cobra.Command {
	var query string
	var searchLimit int

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search history for a keyword",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return fmt.Errorf("--query required")
			}
			return searchHistoryEntries(cmd.OutOrStdout(), container, query, searchLimit)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search keyword")
	cmd.Flags().IntVar(&searchLimit, "limit", DefaultHistorySearchLimit, "Limit search results")
	return cmd
}

// newHistoryClearCommand creates the 'history clear' subcommand
func newHistoryClearCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear history file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return clearHistory(container)
		},
	}
}

// newHistoryExportCommand creates the 'history export' subcommand
func newHistoryExportCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "export <path>",
		Short: "Export history to JSONL file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return exportHistory(container, args[0])
		},
	}
}

// newHistoryStatsCommand creates the 'history stats' subcommand
func newHistoryStatsCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show success rate and top commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showHistoryStats(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newHistoryRetainCommand creates the 'history retain' subcommand
func newHistoryRetainCommand(container *app.Container) *cobra.Command {
	var retainDays int

	cmd := &cobra.Command{
		Use:   "retain",
		Short: "Prune history older than N days and update retention policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if retainDays <= 0 {
				return fmt.Errorf("--days must be > 0")
			}
			return updateHistoryRetention(cmd.Context(), cmd.OutOrStdout(), container, retainDays)
		},
	}

	cmd.Flags().IntVar(&retainDays, "days", DefaultHistoryRetainDays, "Days to retain history")
	return cmd
}

// listHistoryEntries lists recent history entries
func listHistoryEntries(out io.Writer, container *app.Container, limit int) error {
	store := container.HistoryStore
	if store == nil {
		return fmt.Errorf("history store unavailable")
	}

	records, err := store.Records(limit, "")
	if err != nil {
		return fmt.Errorf("failed to retrieve history records: %w", err)
	}

	for _, rec := range records {
		fmt.Fprintf(out, "%s | %s | %s | %s\n",
			rec.Timestamp.Format(TimestampFormat),
			rec.Model,
			rec.RiskLevel,
			rec.Command)
	}

	return nil
}

// searchHistoryEntries searches history for a keyword
func searchHistoryEntries(out io.Writer, container *app.Container, query string, limit int) error {
	store := container.HistoryStore
	if store == nil {
		return fmt.Errorf("history store unavailable")
	}

	records, err := store.Records(limit, query)
	if err != nil {
		return fmt.Errorf("failed to search history: %w", err)
	}

	for _, rec := range records {
		fmt.Fprintf(out, "%s | %s\n",
			rec.Timestamp.Format(TimestampFormat),
			rec.Command)
	}

	return nil
}

// clearHistory clears the history file
func clearHistory(container *app.Container) error {
	if container.HistoryStore == nil {
		return fmt.Errorf("history store unavailable")
	}

	if err := container.HistoryStore.Clear(); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	return nil
}

// exportHistory exports history to a JSONL file
func exportHistory(container *app.Container, path string) error {
	store := container.HistoryStore
	if store == nil {
		return fmt.Errorf("history store unavailable")
	}

	if err := store.ExportJSON(path); err != nil {
		return fmt.Errorf("failed to export history to %s: %w", path, err)
	}

	return nil
}

// showHistoryStats displays success rate and top commands
func showHistoryStats(ctx context.Context, out io.Writer, container *app.Container) error {
	store := container.HistoryStore
	if store == nil {
		return fmt.Errorf("history store unavailable")
	}

	records, err := store.Records(MaxHistoryAnalysisRecords, "")
	if err != nil {
		return fmt.Errorf("failed to retrieve history for analysis: %w", err)
	}

	if len(records) == 0 {
		fmt.Fprintln(out, msgNoHistoryRecorded)
		return nil
	}

	stats := analyzeHistoryRecords(records)
	displayHistoryStatistics(out, stats, records)

	return nil
}

// updateHistoryRetention prunes old history and updates retention policy
func updateHistoryRetention(ctx context.Context, out io.Writer, container *app.Container, days int) error {
	store := container.HistoryStore
	if store == nil {
		return fmt.Errorf("history store unavailable")
	}

	// Prune old records
	if err := store.PruneOlderThan(days); err != nil {
		return fmt.Errorf("failed to prune old history: %w", err)
	}

	// Update configuration
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg.History.RetentionDays = days

	if err := helpers.SaveConfigWithValidation(container, cfg); err != nil {
		return err
	}

	// Update store retention setting
	store.SetRetentionDays(days)

	fmt.Fprintf(out, "Retained last %d days of history.\n", days)
	return nil
}

// historyStatistics holds analyzed history statistics
type historyStatistics struct {
	executed    int
	successful  int
	commandFreq map[string]int
	riskCounts  map[domain.RiskLevel]int
}

// analyzeHistoryRecords analyzes history records and computes statistics
func analyzeHistoryRecords(records []domain.HistoryRecord) historyStatistics {
	stats := historyStatistics{
		commandFreq: make(map[string]int),
		riskCounts:  make(map[domain.RiskLevel]int),
	}

	for _, rec := range records {
		if rec.Executed {
			stats.executed++
			if rec.Success {
				stats.successful++
			}
		}
		stats.commandFreq[rec.Command]++
		stats.riskCounts[rec.RiskLevel]++
	}

	return stats
}

// displayHistoryStatistics displays formatted history statistics
func displayHistoryStatistics(out io.Writer, stats historyStatistics, records []domain.HistoryRecord) {
	// Overall statistics
	fmt.Fprintf(out, "Entries analyzed: %d\nExecuted: %d\nSuccess rate: %.1f%%\n",
		len(records),
		stats.executed,
		helpers.CalculateSuccessRate(stats.successful, stats.executed))

	// Top commands
	fmt.Fprintln(out, "Top commands:")
	topCommands := helpers.CalculateTopCommands(stats.commandFreq, 5)
	for _, stat := range topCommands {
		fmt.Fprintf(out, "  %s (%d)\n", stat.Command, stat.Count)
	}

	// Risk distribution
	fmt.Fprintln(out, "Risk distribution:")
	for level, count := range stats.riskCounts {
		fmt.Fprintf(out, "  %s: %d\n", level, count)
	}

	// Undo hints
	hints := helpers.DeriveUndoHints(records)
	if len(hints) > 0 {
		fmt.Fprintln(out, "Undo hints:")
		for _, hint := range hints {
			fmt.Fprintf(out, "  - %s\n", hint)
		}
	}
}

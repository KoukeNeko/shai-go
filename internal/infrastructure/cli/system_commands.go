package cli

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/version"
)

// ============================================================================
// Version Command
// ============================================================================

// newVersionCommand creates the version command to display version information.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show SHAI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return displayVersionInformation(cmd.OutOrStdout())
		},
	}
}

func displayVersionInformation(out io.Writer) error {
	fmt.Fprintf(out, "SHAI version %s\n", version.Version)

	if version.Commit != "" {
		fmt.Fprintf(out, "Commit: %s\n", version.Commit)
	}

	if version.BuildDate != "" {
		fmt.Fprintf(out, "Built: %s\n", version.BuildDate)
	}

	fmt.Fprintf(out, "Go version: %s\n", runtime.Version())

	return nil
}

// ============================================================================
// Reload Command
// ============================================================================

// newReloadCommand creates the reload command to reload configuration.
func newReloadCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reloadConfiguration(cmd, cmd.OutOrStdout(), container)
		},
	}
}

func reloadConfiguration(cmd *cobra.Command, out io.Writer, container *app.Container) error {
	ctx := cmd.Context()

	// Reload configuration by loading it fresh
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	fmt.Fprintln(out, "Configuration reloaded successfully.")
	fmt.Fprintf(out, "Config version: %s\n", cfg.ConfigFormatVersion)
	fmt.Fprintf(out, "Models configured: %d\n", len(cfg.Models))
	fmt.Fprintf(out, "Guardrails: %s\n", formatEnabledStatus(cfg.Security.Enabled))

	return nil
}

func formatEnabledStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// ============================================================================
// Health Command
// ============================================================================

// newHealthCommand creates the health command to diagnose environment setup.
func newHealthCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check system health and diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealthDiagnostics(cmd, cmd.OutOrStdout(), container)
		},
	}
}

func runHealthDiagnostics(cmd *cobra.Command, out io.Writer, container *app.Container) error {
	if container.HealthService == nil {
		return fmt.Errorf("health service unavailable")
	}

	ctx := cmd.Context()
	report, err := container.HealthService.Run(ctx)

	// Display report even if there were errors
	displayHealthReport(out, report)

	if err != nil {
		return fmt.Errorf("diagnostics completed with errors: %w", err)
	}

	return nil
}

func displayHealthReport(out io.Writer, report domain.HealthReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(out, "[%s] %s - %s\n",
			strings.ToUpper(string(check.Status)),
			check.Name,
			check.Details)
	}
}

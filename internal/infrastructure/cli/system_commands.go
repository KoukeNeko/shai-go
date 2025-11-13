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
// Update Command
// ============================================================================

const (
	releaseChannelStable  = "stable"
	releaseChannelNightly = "nightly"
)

// newUpdateCommand creates the update command to check for new releases.
func newUpdateCommand() *cobra.Command {
	var channel string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for new releases and update instructions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return displayUpdateInstructions(cmd.OutOrStdout(), channel)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", releaseChannelStable, "Release channel (stable/nightly)")

	return cmd
}

func displayUpdateInstructions(out io.Writer, channel string) error {
	fmt.Fprintf(out, "Current version: %s\n", version.Version)
	fmt.Fprintf(out, "Release channel: %s\n", channel)
	fmt.Fprintln(out, "Update instructions:")
	fmt.Fprintln(out, "  1. Visit https://github.com/doeshing/shai-go/releases for the latest binary.")
	fmt.Fprintln(out, "  2. Or run the install script: curl -sSL https://shai.dev/install.sh | bash")
	fmt.Fprintln(out, "  3. Homebrew: brew tap doeshing/shai && brew install shai")
	fmt.Fprintln(out, "  4. Debian/Ubuntu: sudo apt install shai (coming soon)")
	fmt.Fprintln(out, "After updating, run 'shai reload' to refresh shell integrations.")

	return nil
}

// ============================================================================
// Doctor Command
// ============================================================================

// newDoctorCommand creates the doctor command to diagnose environment setup.
func newDoctorCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose environment setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorDiagnostics(cmd, cmd.OutOrStdout(), container)
		},
	}
}

func runDoctorDiagnostics(cmd *cobra.Command, out io.Writer, container *app.Container) error {
	if container.DoctorService == nil {
		return fmt.Errorf("doctor service unavailable")
	}

	ctx := cmd.Context()
	report, err := container.DoctorService.Run(ctx)

	// Display report even if there were errors
	displayDoctorReport(out, report)

	if err != nil {
		return fmt.Errorf("diagnostics completed with errors: %w", err)
	}

	return nil
}

func displayDoctorReport(out io.Writer, report domain.HealthReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(out, "[%s] %s - %s\n",
			strings.ToUpper(string(check.Status)),
			check.Name,
			check.Details)
	}
}

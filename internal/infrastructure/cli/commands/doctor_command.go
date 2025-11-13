package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
)

// NewDoctorCommand creates the doctor command
func NewDoctorCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose environment setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorDiagnostics(cmd, cmd.OutOrStdout(), container)
		},
	}
}

// runDoctorDiagnostics runs environment diagnostics
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

// displayDoctorReport displays the health check report
func displayDoctorReport(out io.Writer, report domain.HealthReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(out, "[%s] %s - %s\n",
			strings.ToUpper(string(check.Status)),
			check.Name,
			check.Details)
	}
}

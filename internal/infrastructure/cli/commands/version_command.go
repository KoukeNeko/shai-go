package commands

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/version"
)

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show SHAI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return displayVersionInformation(cmd.OutOrStdout())
		},
	}
}

// displayVersionInformation displays version information
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

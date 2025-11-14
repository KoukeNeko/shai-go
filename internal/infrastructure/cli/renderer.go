package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
)

// RenderResponse prints the response in a friendly, ASCII-only format.
// If verbose is true, shows detailed context information (directory, tools, model).
func RenderResponse(resp domain.QueryResponse, verbose bool) {
	// For shell integration: output command to file descriptor 3 if available
	if fd3 := os.Getenv("SHAI_SHELL_MODE"); fd3 != "" {
		fmt.Fprintln(os.Stderr, resp.Command)
		return
	}

	if verbose {
		fmt.Println("SHAI analysis complete")
		fmt.Printf("Directory: %s\n", resp.ContextInformation.WorkingDir)
		fmt.Printf("Tools: %s\n", strings.Join(resp.ContextInformation.AvailableTools, ", "))
		if resp.FromCache {
			fmt.Println("Note: result served from cache")
		}
		if resp.ModelUsed != "" {
			fmt.Printf("Model: %s\n", resp.ModelUsed)
		}
		fmt.Println()
	}

	fmt.Println("Generated Command:")
	fmt.Printf("  %s\n", resp.Command)

	fmt.Printf("\nRisk: %s (%s)\n", strings.ToUpper(string(resp.RiskAssessment.Level)), resp.RiskAssessment.Action)
	for _, reason := range resp.RiskAssessment.Reasons {
		fmt.Printf(" - %s\n", reason)
	}
	if resp.RiskAssessment.DryRunCommand != "" {
		fmt.Printf("Dry-run suggestion: %s\n", resp.RiskAssessment.DryRunCommand)
	}
	if len(resp.RiskAssessment.UndoHints) > 0 {
		fmt.Println("Undo hints:")
		for _, hint := range resp.RiskAssessment.UndoHints {
			fmt.Printf(" * %s\n", hint)
		}
	}

	if resp.ExecutionResult != nil {
		if resp.ExecutionResult.Ran {
			fmt.Println("\nCommand executed successfully.")
		} else if resp.ExecutionResult.Err != nil {
			fmt.Printf("\nCommand failed: %v\n", resp.ExecutionResult.Err)
		}
		if resp.ExecutionResult.Stdout != "" {
			fmt.Println("\nstdout:")
			fmt.Println(resp.ExecutionResult.Stdout)
		}
		if resp.ExecutionResult.Stderr != "" {
			fmt.Println("\nstderr:")
			fmt.Println(resp.ExecutionResult.Stderr)
		}
	} else {
		fmt.Println("\nCommand was not executed (preview mode or confirmation pending).")
	}
}

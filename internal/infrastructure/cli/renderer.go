package cli

import (
	"fmt"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
)

// stripMarkdownFormatting removes markdown code block backticks from command output.
func stripMarkdownFormatting(cmd string) string {
	// Remove leading/trailing backticks and whitespace
	cleaned := strings.TrimSpace(cmd)
	cleaned = strings.Trim(cleaned, "`")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

// RenderResponse prints the response in a friendly, ASCII-only format.
// If verbose is false, only outputs the command (for shell integration).
// If verbose is true, shows detailed context information.
// Always shows guardrail blocks regardless of verbose setting.
func RenderResponse(resp domain.QueryResponse, verbose bool) {
	// Check if command was blocked by guardrail
	isBlocked := resp.RiskAssessment.Action == "block"

	// If not verbose and not blocked, only output the command
	if !verbose && !isBlocked {
		// Strip markdown code block formatting (backticks)
		cleanCmd := stripMarkdownFormatting(resp.Command)
		fmt.Print(cleanCmd) // No newline for shell integration
		return
	}

	// Verbose mode or blocked: show full details
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
	} else if verbose || isBlocked {
		fmt.Println("\nCommand was not executed (preview mode or confirmation pending).")
	}
}

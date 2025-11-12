package cli

import (
	"fmt"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
)

// RenderResponse prints the response in a friendly, ASCII-only format.
func RenderResponse(resp domain.QueryResponse) {
	fmt.Println("SHAI analysis complete")
	fmt.Printf("Directory: %s\n", resp.ContextInformation.WorkingDir)
	fmt.Printf("Tools: %s\n", strings.Join(resp.ContextInformation.AvailableTools, ", "))
	if resp.FromCache {
		fmt.Println("Note: result served from cache")
	}

	fmt.Println()
	fmt.Println("Generated Command:")
	fmt.Printf("  %s\n", resp.Command)

	fmt.Printf("\nRisk: %s (%s)\n", strings.ToUpper(string(resp.RiskAssessment.Level)), resp.RiskAssessment.Action)
	for _, reason := range resp.RiskAssessment.Reasons {
		fmt.Printf(" - %s\n", reason)
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

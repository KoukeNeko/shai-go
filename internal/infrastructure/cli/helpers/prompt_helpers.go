package helpers

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

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

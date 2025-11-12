package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Prompter implements ConfirmationPrompter using stdin/stdout.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// NewPrompter constructs a prompter referencing stdio.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &Prompter{
		in:  bufio.NewReader(in),
		out: out,
	}
}

// Enabled indicates the prompter is interactive.
func (p *Prompter) Enabled() bool {
	return true
}

// Confirm asks the user for confirmation based on guardrail action.
func (p *Prompter) Confirm(action domain.GuardrailAction, level domain.RiskLevel, command string, reasons []string) (bool, error) {
	fmt.Fprintf(p.out, "\n⚠️  %s risk detected (%s)\n", strings.ToUpper(string(level)), action)
	for _, reason := range reasons {
		fmt.Fprintf(p.out, " - %s\n", reason)
	}
	fmt.Fprintf(p.out, "Command:\n  %s\n", command)

	switch action {
	case domain.ActionSimpleConfirm, domain.ActionConfirm:
		return p.ask("[y/N]: ")
	case domain.ActionExplicitConfirm:
		return p.askExplicit()
	default:
		return false, nil
	}
}

func (p *Prompter) ask(prompt string) (bool, error) {
	fmt.Fprint(p.out, "Continue? ", prompt)
	line, err := p.in.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}

func (p *Prompter) askExplicit() (bool, error) {
	fmt.Fprint(p.out, "Type 'yes' to confirm (or anything else to cancel): ")
	line, err := p.in.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(line) == "yes", nil
}

var _ ports.ConfirmationPrompter = (*Prompter)(nil)

package infrastructure

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// LocalExecutor runs commands on the host shell.
type LocalExecutor struct {
	shell string
}

// NewLocalExecutor builds a new executor, shell defaults to /bin/sh.
func NewLocalExecutor(shell string) *LocalExecutor {
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	return &LocalExecutor{shell: shell}
}

// Execute implements ports.CommandExecutor.
func (e *LocalExecutor) Execute(ctx context.Context, command string, previewOnly bool) (domain.ExecutionResult, error) {
	if previewOnly {
		return domain.ExecutionResult{
			Ran:         false,
			DryRunNotes: "preview mode enabled",
		}, nil
	}

	c := exec.CommandContext(ctx, e.shell, "-c", command)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	start := time.Now()
	err := c.Run()
	duration := time.Since(start).Milliseconds()

	result := domain.ExecutionResult{
		Ran:        err == nil,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMS: duration,
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		result.Err = err
		return result, err
	}
	if err != nil {
		result.Err = err
		return result, err
	}
	result.ExitCode = 0
	return result, nil
}

var _ ports.CommandExecutor = (*LocalExecutor)(nil)

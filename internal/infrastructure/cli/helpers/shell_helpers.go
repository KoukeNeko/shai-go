package helpers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

const (
	shellAutoDetect = "auto"
	shellAll        = "all"
)

// DetermineTargetShells resolves which shells to operate on based on the flag value
// Returns a list of shell names to target
func DetermineTargetShells(shellFlag string, integrator ports.ShellIntegrator) ([]domain.ShellName, error) {
	normalizedFlag := normalizeShellFlag(shellFlag)

	switch normalizedFlag {
	case "", shellAutoDetect:
		return autoDetectShells(integrator), nil
	case shellAll:
		return allSupportedShells(), nil
	default:
		return parseSingleShell(normalizedFlag)
	}
}

// autoDetectShells attempts to detect the current shell, falling back to all shells
func autoDetectShells(integrator ports.ShellIntegrator) []domain.ShellName {
	detectedShell := ParseShellName(integrator.DetectShell())

	if detectedShell != domain.ShellUnknown {
		return []domain.ShellName{detectedShell}
	}

	// Fallback to all supported shells if detection fails
	return allSupportedShells()
}

// allSupportedShells returns all shells currently supported by the application
func allSupportedShells() []domain.ShellName {
	return []domain.ShellName{domain.ShellZsh, domain.ShellBash}
}

// parseSingleShell parses a single shell name from a flag value
func parseSingleShell(value string) ([]domain.ShellName, error) {
	shellName := ParseShellName(value)

	if shellName == domain.ShellUnknown {
		return nil, fmt.Errorf("unsupported shell: %s", value)
	}

	return []domain.ShellName{shellName}, nil
}

// ParseShellName converts a string to a ShellName constant
// Handles both simple names and full paths (e.g., "/bin/zsh" -> "zsh")
func ParseShellName(value string) domain.ShellName {
	// Extract basename and normalize
	normalized := normalizeShellName(value)

	switch normalized {
	case "zsh":
		return domain.ShellZsh
	case "bash":
		return domain.ShellBash
	default:
		return domain.ShellUnknown
	}
}

// normalizeShellFlag normalizes a shell flag value
func normalizeShellFlag(flag string) string {
	return strings.ToLower(strings.TrimSpace(flag))
}

// normalizeShellName extracts the basename and normalizes a shell name
func normalizeShellName(value string) string {
	basename := filepath.Base(value)
	return strings.ToLower(strings.TrimSpace(basename))
}

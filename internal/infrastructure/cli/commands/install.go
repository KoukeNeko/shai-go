package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/assets"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/pkg/filesystem"
)

// ShellType represents supported shell types
type ShellType string

const (
	ShellZsh  ShellType = "zsh"
	ShellBash ShellType = "bash"
)

const (
	shaiMarkerStart = "# >>> SHAI integration >>>"
	shaiMarkerEnd   = "# <<< SHAI integration <<<"
)

// NewInstallCommand creates the installation command for shell integration
func NewInstallCommand() *cobra.Command {
	var shellFlag string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install SHAI shell integration",
		Long: `Install SHAI shell integration to enable '#' prefix command generation.

This command will:
1. Detect your current shell (or use --shell flag)
2. Copy integration script to ~/.shai/shell/
3. Add source line to your shell RC file (~/.zshrc or ~/.bashrc)
4. Create backup of original RC file

Example:
  shai install              # Auto-detect shell
  shai install --shell zsh  # Install for zsh
  shai install --shell bash # Install for bash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd.OutOrStdout(), cmd.ErrOrStderr(), shellFlag)
		},
	}

	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell type (zsh, bash). Auto-detected if not specified")

	return cmd
}

func runInstall(out, errOut io.Writer, shellFlag string) error {
	// Detect shell
	shell, err := detectShell(shellFlag)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Installing SHAI integration for %s...\n\n", shell)

	// Get paths
	shaiDir := filepath.Join(filesystem.UserHomeDir(), ".shai")
	shellDir := filepath.Join(shaiDir, "shell")
	rcFile := getRCFile(shell)
	scriptFile := filepath.Join(shellDir, string(shell)+".sh")

	// Create ~/.shai/shell directory
	if err := os.MkdirAll(shellDir, domain.DirectoryPermissions); err != nil {
		return fmt.Errorf("create shell directory: %w", err)
	}

	// Copy shell script from embedded assets
	var scriptContent []byte
	switch shell {
	case ShellZsh:
		scriptContent = assets.ShellZshScript
	case ShellBash:
		scriptContent = assets.ShellBashScript
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	if err := os.WriteFile(scriptFile, scriptContent, domain.SecureFilePermissions); err != nil {
		return fmt.Errorf("write shell script: %w", err)
	}
	fmt.Fprintf(out, "✓ Created shell script: %s\n", scriptFile)

	// Check if RC file exists
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		// Create empty RC file
		if err := os.WriteFile(rcFile, []byte{}, domain.SecureFilePermissions); err != nil {
			return fmt.Errorf("create RC file: %w", err)
		}
		fmt.Fprintf(out, "✓ Created RC file: %s\n", rcFile)
	}

	// Check if already installed
	installed, err := isAlreadyInstalled(rcFile)
	if err != nil {
		return fmt.Errorf("check installation: %w", err)
	}

	if installed {
		fmt.Fprintf(out, "\n⚠️  SHAI integration already installed in %s\n", rcFile)
		fmt.Fprintf(out, "\nTo reinstall, first run:\n  shai uninstall\n")
		return nil
	}

	// Backup RC file
	backupFile := fmt.Sprintf("%s.shai-backup.%s", rcFile, time.Now().Format("20060102-150405"))
	if err := copyFile(rcFile, backupFile); err != nil {
		return fmt.Errorf("backup RC file: %w", err)
	}
	fmt.Fprintf(out, "✓ Backup created: %s\n", backupFile)

	// Add source line to RC file
	sourceLine := fmt.Sprintf("[ -f %s ] && source %s", scriptFile, scriptFile)
	integrationBlock := fmt.Sprintf("\n%s\n%s\n%s\n", shaiMarkerStart, sourceLine, shaiMarkerEnd)

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY, domain.SecureFilePermissions)
	if err != nil {
		return fmt.Errorf("open RC file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(integrationBlock); err != nil {
		return fmt.Errorf("write to RC file: %w", err)
	}

	fmt.Fprintf(out, "✓ Added integration to %s\n", rcFile)
	fmt.Fprintf(out, "\n✨ Installation complete!\n\n")
	fmt.Fprintf(out, "To activate, run:\n")
	fmt.Fprintf(out, "  source %s\n\n", rcFile)
	fmt.Fprintf(out, "Usage:\n")
	fmt.Fprintf(out, "  # list all docker containers\n")
	fmt.Fprintf(out, "  → Press Enter to generate and execute command\n")

	return nil
}

func detectShell(override string) (ShellType, error) {
	if override != "" {
		shell := ShellType(strings.ToLower(override))
		if shell != ShellZsh && shell != ShellBash {
			return "", fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", override)
		}
		return shell, nil
	}

	// Try SHELL environment variable
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "", errors.New("could not detect shell (SHELL env var not set). Use --shell flag")
	}

	shellName := filepath.Base(shellPath)
	switch shellName {
	case "zsh":
		return ShellZsh, nil
	case "bash":
		return ShellBash, nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: zsh, bash). Use --shell flag", shellName)
	}
}

func getRCFile(shell ShellType) string {
	home := filesystem.UserHomeDir()
	switch shell {
	case ShellZsh:
		return filepath.Join(home, ".zshrc")
	case ShellBash:
		return filepath.Join(home, ".bashrc")
	default:
		return ""
	}
}

func isAlreadyInstalled(rcFile string) (bool, error) {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return false, err
	}

	content := string(data)
	return strings.Contains(content, shaiMarkerStart) || strings.Contains(content, ".shai/shell/"), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, domain.SecureFilePermissions)
}

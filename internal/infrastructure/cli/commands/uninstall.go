package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/pkg/filesystem"
)

// NewUninstallCommand creates the uninstall command for shell integration
func NewUninstallCommand() *cobra.Command {
	var shellFlag string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall SHAI shell integration",
		Long: `Uninstall SHAI shell integration by removing all integration files.

This command will:
1. Detect your current shell (or use --shell flag)
2. Remove SHAI integration from shell RC file (~/.zshrc or ~/.bashrc)
3. Delete shell scripts from ~/.shai/shell/
4. Create backup of RC file before modification

Example:
  shai uninstall              # Auto-detect shell
  shai uninstall --shell zsh  # Uninstall from zsh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd.OutOrStdout(), cmd.ErrOrStderr(), shellFlag)
		},
	}

	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell type (zsh, bash). Auto-detected if not specified")

	return cmd
}

func runUninstall(out, errOut io.Writer, shellFlag string) error {
	// Detect shell
	shell, err := detectShell(shellFlag)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Uninstalling SHAI integration for %s...\n\n", shell)

	// Get paths
	rcFile := getRCFile(shell)
	shellDir := filepath.Join(filesystem.UserHomeDir(), ".shai", "shell")
	scriptFile := filepath.Join(shellDir, string(shell)+".sh")

	// Check if RC file exists
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		fmt.Fprintf(out, "⚠️  RC file not found: %s\n", rcFile)
		return nil
	}

	// Check if SHAI is installed
	installed, err := isAlreadyInstalled(rcFile)
	if err != nil {
		return fmt.Errorf("check installation: %w", err)
	}

	if !installed {
		fmt.Fprintf(out, "⚠️  SHAI integration not found in %s\n", rcFile)
		fmt.Fprintf(out, "Nothing to uninstall.\n")
		return nil
	}

	// Backup RC file
	backupFile := fmt.Sprintf("%s.shai-backup.%s", rcFile, time.Now().Format("20060102-150405"))
	if err := copyFile(rcFile, backupFile); err != nil {
		return fmt.Errorf("backup RC file: %w", err)
	}
	fmt.Fprintf(out, "✓ Backup created: %s\n", backupFile)

	// Remove SHAI integration from RC file
	if err := removeShaiIntegration(rcFile); err != nil {
		return fmt.Errorf("remove integration: %w", err)
	}
	fmt.Fprintf(out, "✓ Removed integration from %s\n", rcFile)

	// Remove script file
	if _, err := os.Stat(scriptFile); err == nil {
		if err := os.Remove(scriptFile); err != nil {
			return fmt.Errorf("remove script file: %w", err)
		}
		fmt.Fprintf(out, "✓ Removed script: %s\n", scriptFile)
	}

	// Remove shell directory if empty
	if isEmpty, _ := isDirEmpty(shellDir); isEmpty {
		if err := os.Remove(shellDir); err == nil {
			fmt.Fprintf(out, "✓ Removed empty directory: %s\n", shellDir)
		}
	}

	fmt.Fprintf(out, "\n✨ Uninstallation complete!\n\n")
	fmt.Fprintf(out, "The changes will take effect after you:\n")
	fmt.Fprintf(out, "  source %s\n", rcFile)
	fmt.Fprintf(out, "Or simply restart your terminal.\n")

	return nil
}

func removeShaiIntegration(rcFile string) error {
	// Read RC file
	file, err := os.Open(rcFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	inShaiBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		// Detect SHAI block markers
		if strings.Contains(line, shaiMarkerStart) {
			inShaiBlock = true
			continue
		}
		if strings.Contains(line, shaiMarkerEnd) {
			inShaiBlock = false
			continue
		}

		// Skip lines inside SHAI block
		if inShaiBlock {
			continue
		}

		// Also skip standalone SHAI source lines (without markers)
		if strings.Contains(line, ".shai/shell/") && strings.Contains(line, "source") {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Write back to file
	content := strings.Join(lines, "\n")
	// Ensure file ends with newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(rcFile, []byte(content), domain.SecureFilePermissions)
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

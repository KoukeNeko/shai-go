package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
)

// NewInstallCommand creates the install command
func NewInstallCommand(container *app.Container) *cobra.Command {
	var shell string
	var force bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install SHAI shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellInstall(cmd, container, shell, force)
		},
	}

	cmd.Flags().StringVar(&shell, "shell", "", "Shell to install (zsh|bash|all, auto-detected by default)")
	cmd.Flags().BoolVar(&force, "force", false, "Force rewrite of rc entry")

	return cmd
}

// NewUninstallCommand creates the uninstall command
func NewUninstallCommand(container *app.Container) *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove SHAI shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellUninstall(cmd, container, shell)
		},
	}

	cmd.Flags().StringVar(&shell, "shell", "", "Shell to uninstall (zsh|bash|all, auto-detected by default)")

	return cmd
}

// NewReloadCommand creates the reload command
func NewReloadCommand(container *app.Container) *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellReload(cmd, container, shell)
		},
	}

	cmd.Flags().StringVar(&shell, "shell", "", "Shell to reload (zsh|bash|all, auto-detected by default)")

	return cmd
}

// runShellInstall installs shell integration
func runShellInstall(cmd *cobra.Command, container *app.Container, shellFlag string, force bool) error {
	if container.ShellIntegrator == nil {
		return fmt.Errorf("shell installer unavailable")
	}

	shells, err := helpers.DetermineTargetShells(shellFlag, container.ShellIntegrator)
	if err != nil {
		return fmt.Errorf("failed to determine target shells: %w", err)
	}

	for _, sh := range shells {
		if err := installForSingleShell(cmd, container, sh, force); err != nil {
			return err
		}
	}

	return nil
}

// runShellUninstall removes shell integration
func runShellUninstall(cmd *cobra.Command, container *app.Container, shellFlag string) error {
	if container.ShellIntegrator == nil {
		return fmt.Errorf("shell installer unavailable")
	}

	shells, err := helpers.DetermineTargetShells(shellFlag, container.ShellIntegrator)
	if err != nil {
		return fmt.Errorf("failed to determine target shells: %w", err)
	}

	for _, sh := range shells {
		if err := uninstallForSingleShell(cmd, container, sh); err != nil {
			return err
		}
	}

	return nil
}

// runShellReload reloads shell integration
func runShellReload(cmd *cobra.Command, container *app.Container, shellFlag string) error {
	if container.ShellIntegrator == nil {
		return fmt.Errorf("shell installer unavailable")
	}

	shells, err := helpers.DetermineTargetShells(shellFlag, container.ShellIntegrator)
	if err != nil {
		return fmt.Errorf("failed to determine target shells: %w", err)
	}

	for _, sh := range shells {
		displayReloadInstructions(cmd, container, sh)
	}

	return nil
}

// installForSingleShell installs integration for a single shell
func installForSingleShell(cmd *cobra.Command, container *app.Container, shell domain.ShellName, force bool) error {
	result, err := container.ShellIntegrator.Install(string(shell), force)
	if err != nil {
		return fmt.Errorf("failed to install for %s: %w", shell, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed for %s\nScript: %s\nRC File: %s\n",
		result.Shell,
		result.ScriptPath,
		result.RCFile)

	helpers.PrintWarnings(cmd.ErrOrStderr(), result.Warnings)

	return nil
}

// uninstallForSingleShell removes integration for a single shell
func uninstallForSingleShell(cmd *cobra.Command, container *app.Container, shell domain.ShellName) error {
	result, err := container.ShellIntegrator.Uninstall(string(shell))
	if err != nil {
		return fmt.Errorf("failed to uninstall for %s: %w", shell, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed sourcing line for %s in %s\n",
		result.Shell,
		result.RCFile)

	helpers.PrintWarnings(cmd.ErrOrStderr(), result.Warnings)

	return nil
}

// displayReloadInstructions displays reload instructions for a shell
func displayReloadInstructions(cmd *cobra.Command, container *app.Container, shell domain.ShellName) {
	status := container.ShellIntegrator.Status(string(shell))

	if status.Error != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s\n", shell, status.Error)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "[%s] reload by running: source %s (or open a new shell)\n",
		status.Shell,
		status.RCFile)

	helpers.PrintWarnings(cmd.ErrOrStderr(), status.Warnings)
}

package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
	"github.com/doeshing/shai-go/internal/services"
)

const (
	msgInitCancelled = "Init cancelled."
)

// NewInitCommand creates the init command to initialize SHAI configuration.
// This command creates a default configuration file with sensible defaults.
// Users can then edit ~/.shai/config.yaml to add their preferred AI models.
func NewInitCommand(container *app.Container) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize SHAI configuration",
		Long: `Initialize SHAI configuration with default settings.

This command creates ~/.shai/config.yaml with sensible defaults.
After initialization, you should:
  1. Edit ~/.shai/config.yaml to configure your AI models
  2. Set required API keys (e.g., ANTHROPIC_API_KEY, OPENAI_API_KEY)
  3. Run 'shai doctor' to verify your setup

Example configuration:
  models:
    - name: claude-sonnet-4
      endpoint: https://api.anthropic.com/v1/messages
      model_id: claude-sonnet-4-20250514
      auth_env_var: ANTHROPIC_API_KEY
      max_tokens: 1024
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWizard(cmd, container, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config without prompting")

	return cmd
}

// runInitWizard runs the configuration initialization wizard
func runInitWizard(cmd *cobra.Command, container *app.Container, force bool) error {
	loader, err := helpers.GetConfigLoader(container)
	if err != nil {
		return err
	}

	configPath := loader.Path()

	// Check if config exists and handle overwrite confirmation
	if !shouldProceedWithInit(cmd, configPath, force) {
		fmt.Fprintln(cmd.OutOrStdout(), msgInitCancelled)
		return nil
	}

	// Build configuration with defaults
	cfg := configinfra.DefaultConfig()

	// Prompt for user preferences (optional interactive mode)
	reader := bufio.NewReader(cmd.InOrStdin())
	cfg = promptForUserPreferences(cmd, reader, cfg)

	// Validate configuration
	if err := services.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Backup existing config if present
	if err := backupExistingConfig(loader); err != nil {
		return err
	}

	// Save configuration
	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Display completion instructions
	displayCompletionInstructions(cmd.OutOrStdout(), configPath)

	return nil
}

// shouldProceedWithInit checks if we should proceed with initialization
func shouldProceedWithInit(cmd *cobra.Command, configPath string, force bool) bool {
	if _, err := os.Stat(configPath); err != nil {
		return true // Config doesn't exist, proceed
	}

	if force {
		return true // Force flag set, proceed
	}

	// Ask for confirmation
	reader := bufio.NewReader(cmd.InOrStdin())
	question := fmt.Sprintf("%s exists. Overwrite?", configPath)
	return helpers.PromptForYesNo(cmd.OutOrStdout(), reader, question, false)
}

// promptForUserPreferences prompts for user preferences and updates config
func promptForUserPreferences(cmd *cobra.Command, reader *bufio.Reader, cfg domain.Config) domain.Config {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "\nConfiguration preferences:")

	// Context settings
	cfg.Context.IncludeGit = helpers.PromptForChoice(out, reader,
		"Include git context (auto/always/never)?", cfg.Context.IncludeGit)

	cfg.Context.IncludeK8s = helpers.PromptForChoice(out, reader,
		"Include kubernetes context (auto/always/never)?", cfg.Context.IncludeK8s)

	cfg.Context.IncludeEnv = helpers.PromptForYesNo(out, reader,
		"Include environment variables (PATH, KUBECONFIG)?", cfg.Context.IncludeEnv)

	// Preferences
	cfg.Preferences.PreviewMode = helpers.PromptForChoice(out, reader,
		"Preview mode (always/never)?", cfg.Preferences.PreviewMode)

	cfg.Preferences.AutoExecuteSafe = helpers.PromptForYesNo(out, reader,
		"Auto execute safe commands?", cfg.Preferences.AutoExecuteSafe)

	// Execution settings
	cfg.Execution.ConfirmBeforeExecute = helpers.PromptForYesNo(out, reader,
		"Confirm before executing commands?", cfg.Execution.ConfirmBeforeExecute)

	return cfg
}

// backupExistingConfig creates a backup of existing config file if it exists
func backupExistingConfig(loader *configinfra.FileLoader) error {
	if _, err := os.Stat(loader.Path()); err != nil {
		return nil // Config doesn't exist, no backup needed
	}

	backupPath, err := loader.Backup()
	if err != nil {
		return fmt.Errorf("failed to create configuration backup: %w", err)
	}

	fmt.Printf("Existing config backed up to: %s\n", backupPath)
	return nil
}

// displayCompletionInstructions displays instructions after successful initialization
func displayCompletionInstructions(out io.Writer, configPath string) {
	fmt.Fprintf(out, "\n✓ Configuration initialized: %s\n\n", configPath)
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintln(out, "  1. Edit the config file to add your AI models:")
	fmt.Fprintf(out, "     %s\n\n", configPath)
	fmt.Fprintln(out, "  2. Set required environment variables:")
	fmt.Fprintln(out, "     export ANTHROPIC_API_KEY=your-key-here")
	fmt.Fprintln(out, "     export OPENAI_API_KEY=your-key-here")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  3. Verify your setup:")
	fmt.Fprintln(out, "     shai doctor")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  4. Test a query:")
	fmt.Fprintln(out, "     shai query \"list files\"")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Documentation: https://docs.shai.dev/configuration")
}

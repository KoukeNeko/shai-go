package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	configapp "github.com/doeshing/shai-go/internal/application/config"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure/config"
)

const (
	msgInitCancelled = "Init cancelled."
)

// providerTemplate defines a template for AI provider configuration
type providerTemplate struct {
	Key          string
	Label        string
	Model        domain.ModelDefinition
	Instructions string
}

// providerOptions lists available provider templates
var providerOptions = []providerTemplate{
	{
		Key:   ProviderKeyAnthropic,
		Label: "Anthropic Claude (claude-sonnet-4)",
		Model: domain.ModelDefinition{
			Name:       "claude-sonnet-4",
			Endpoint:   "https://api.anthropic.com/v1/messages",
			ModelID:    "claude-3-5-sonnet-20240620",
			AuthEnvVar: "ANTHROPIC_API_KEY",
			MaxTokens:  DefaultMaxTokens,
			Prompt:     configinfra.DefaultConfig().Models[0].Prompt,
		},
		Instructions: "Set ANTHROPIC_API_KEY in your environment.",
	},
	{
		Key:   ProviderKeyOpenAI,
		Label: "OpenAI GPT-4o",
		Model: domain.ModelDefinition{
			Name:       "gpt-4o",
			Endpoint:   "https://api.openai.com/v1/chat/completions",
			ModelID:    "gpt-4o-mini",
			AuthEnvVar: "OPENAI_API_KEY",
			OrgEnvVar:  "OPENAI_ORG_ID",
			MaxTokens:  800,
		},
		Instructions: "Set OPENAI_API_KEY (and OPENAI_ORG_ID if required).",
	},
	{
		Key:   ProviderKeyOllama,
		Label: "Ollama (local codellama)",
		Model: domain.ModelDefinition{
			Name:      "codellama",
			Endpoint:  "http://localhost:11434/v1/chat/completions",
			ModelID:   "codellama:7b",
			MaxTokens: 512,
		},
		Instructions: "Ensure Ollama is running locally (`ollama run codellama`).",
	},
}

// NewInitCommand creates the init command
func NewInitCommand(container *app.Container) *cobra.Command {
	var providerKey string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive configuration wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWizard(cmd, container, providerKey, force)
		},
	}

	cmd.Flags().StringVar(&providerKey, "provider", "", "Default provider (anthropic|openai|ollama)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config without prompting")

	return cmd
}

// runInitWizard runs the interactive configuration wizard
func runInitWizard(cmd *cobra.Command, container *app.Container, providerKey string, force bool) error {
	loader, err := helpers.GetConfigLoader(container)
	if err != nil {
		return err
	}

	// Check if config exists and handle overwrite confirmation
	if !shouldProceedWithInit(cmd, loader.Path(), force) {
		fmt.Fprintln(cmd.OutOrStdout(), msgInitCancelled)
		return nil
	}

	// Select provider
	reader := bufio.NewReader(cmd.InOrStdin())
	selectedProvider, err := selectProvider(cmd.OutOrStdout(), reader, providerKey)
	if err != nil {
		return err
	}

	// Build configuration
	cfg := buildInitialConfiguration(selectedProvider)
	cfg = promptForUserPreferences(cmd, reader, cfg)
	cfg = promptForFallbackModels(cmd, reader, cfg)

	// Validate configuration
	if err := configapp.Validate(cfg); err != nil {
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
	displayCompletionInstructions(cmd.OutOrStdout(), loader.Path(), selectedProvider)

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

// selectProvider selects an AI provider either by key or interactively
func selectProvider(out io.Writer, reader *bufio.Reader, key string) (providerTemplate, error) {
	// If key is provided, find matching provider
	if key != "" {
		normalizedKey := strings.ToLower(key)
		for _, option := range providerOptions {
			if option.Key == normalizedKey {
				return option, nil
			}
		}
		return providerTemplate{}, fmt.Errorf("unknown provider %s", key)
	}

	// Interactive selection
	return selectProviderInteractively(out, reader)
}

// selectProviderInteractively prompts the user to select a provider
func selectProviderInteractively(out io.Writer, reader *bufio.Reader) (providerTemplate, error) {
	fmt.Fprintln(out, "Select a provider:")
	for i, option := range providerOptions {
		fmt.Fprintf(out, "  %d) %s\n", i+1, option.Label)
	}

	choice := helpers.PromptForString(out, reader, "Choice [1]:", "1")

	index := DefaultProviderChoice
	fmt.Sscanf(choice, "%d", &index)

	if index < MinProviderChoice || index > len(providerOptions) {
		return providerOptions[0], nil // Default to first provider
	}

	return providerOptions[index-1], nil
}

// buildInitialConfiguration creates initial configuration from selected provider
func buildInitialConfiguration(provider providerTemplate) domain.Config {
	cfg := configinfra.DefaultConfig()
	cfg.Models = []domain.ModelDefinition{provider.Model}
	cfg.Preferences.DefaultModel = provider.Model.Name
	cfg.Preferences.FallbackModels = nil
	return cfg
}

// promptForUserPreferences prompts for user preferences and updates config
func promptForUserPreferences(cmd *cobra.Command, reader *bufio.Reader, cfg domain.Config) domain.Config {
	out := cmd.OutOrStdout()

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

// promptForFallbackModels prompts for fallback models
func promptForFallbackModels(cmd *cobra.Command, reader *bufio.Reader, cfg domain.Config) domain.Config {
	fallbackInput := helpers.PromptForString(cmd.OutOrStdout(), reader,
		"Fallback models (comma separated, optional):", "")

	if fallbackInput == "" {
		return cfg
	}

	fallbackNames := helpers.SplitAndTrimCSV(fallbackInput)
	for _, name := range fallbackNames {
		if name != "" && name != cfg.Preferences.DefaultModel {
			cfg.Preferences.FallbackModels = append(cfg.Preferences.FallbackModels, name)
		}
	}

	return cfg
}

// backupExistingConfig creates a backup of existing config file if it exists
func backupExistingConfig(loader *configinfra.FileLoader) error {
	if _, err := os.Stat(loader.Path()); err != nil {
		return nil // Config doesn't exist, no backup needed
	}

	if _, err := loader.Backup(); err != nil {
		return fmt.Errorf("failed to create configuration backup: %w", err)
	}

	return nil
}

// displayCompletionInstructions displays instructions after successful initialization
func displayCompletionInstructions(out io.Writer, configPath string, provider providerTemplate) {
	fmt.Fprintf(out, "Configuration written to %s\n", configPath)

	// Display environment variable instructions
	if provider.Model.AuthEnvVar != "" {
		fmt.Fprintf(out, "Remember to export %s\n", provider.Model.AuthEnvVar)
	}

	if provider.Model.OrgEnvVar != "" {
		fmt.Fprintf(out, "If required, set %s as well.\n", provider.Model.OrgEnvVar)
	}

	// Display provider-specific instructions
	if provider.Instructions != "" {
		fmt.Fprintln(out, provider.Instructions)
	}
}

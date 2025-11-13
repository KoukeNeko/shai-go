package commands

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/ai"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
	"github.com/doeshing/shai-go/internal/ports"
)

// NewModelsCommand creates the models command with all subcommands
func NewModelsCommand(container *app.Container) *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Manage AI model configurations",
	}

	modelsCmd.AddCommand(
		newModelsListCommand(container),
		newModelsTestCommand(container),
		newModelsUseCommand(container),
		newModelsAddCommand(container),
		newModelsRemoveCommand(container),
	)

	return modelsCmd
}

// newModelsListCommand creates the 'models list' subcommand
func newModelsListCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listModels(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newModelsTestCommand creates the 'models test' subcommand
func newModelsTestCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test connectivity for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return testModel(cmd.Context(), cmd.OutOrStdout(), container, args[0])
		},
	}
}

// newModelsUseCommand creates the 'models use' subcommand
func newModelsUseCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set default model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setDefaultModel(cmd.Context(), container, args[0])
		},
	}
}

// newModelsAddCommand creates the 'models add' subcommand
func newModelsAddCommand(container *app.Container) *cobra.Command {
	var (
		name       string
		endpoint   string
		modelID    string
		authEnv    string
		orgEnv     string
		promptFile string
		maxTokens  int
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new model definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := modelAddOptions{
				Name:       name,
				Endpoint:   endpoint,
				ModelID:    modelID,
				AuthEnv:    authEnv,
				OrgEnv:     orgEnv,
				PromptFile: promptFile,
				MaxTokens:  maxTokens,
			}
			return addModel(cmd.Context(), container, opts)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Model name (identifier)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Provider endpoint URL")
	cmd.Flags().StringVar(&modelID, "model-id", "", "Model identifier at provider")
	cmd.Flags().StringVar(&authEnv, "auth-env", "", "Environment variable containing API key")
	cmd.Flags().StringVar(&orgEnv, "org-env", "", "Environment variable containing org/project ID")
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Path to YAML prompt template for this model")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", domain.DefaultMaxTokens, "Max tokens for responses")

	return cmd
}

// newModelsRemoveCommand creates the 'models remove' subcommand
func newModelsRemoveCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove model definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeModel(cmd.Context(), container, args[0])
		},
	}
}

// modelAddOptions holds options for adding a new model
type modelAddOptions struct {
	Name       string
	Endpoint   string
	ModelID    string
	AuthEnv    string
	OrgEnv     string
	PromptFile string
	MaxTokens  int
}

// listModels lists all configured models
func listModels(ctx context.Context, out io.Writer, container *app.Container) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Fprintf(out, "NAME\tMODEL ID\tENDPOINT\tDEFAULT\n")

	for _, model := range cfg.Models {
		defaultMarker := ""
		if cfg.Preferences.DefaultModel == model.Name {
			defaultMarker = "*"
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
			model.Name,
			model.ModelID,
			model.Endpoint,
			defaultMarker)
	}

	if len(cfg.Preferences.FallbackModels) > 0 {
		fmt.Fprintf(out, "Fallbacks: %s\n", strings.Join(cfg.Preferences.FallbackModels, ", "))
	}

	return nil
}

// testModel tests connectivity for a specific model
func testModel(ctx context.Context, out io.Writer, container *app.Container, modelName string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	model, exists := cfg.FindModelByName(modelName)
	if !exists {
		return fmt.Errorf("model %s not found", modelName)
	}

	provider, err := ai.NewFactory().ForModel(model)
	if err != nil {
		return fmt.Errorf("failed to create provider for model %s: %w", modelName, err)
	}

	testCtx, cancel := context.WithTimeout(ctx, domain.DefaultModelTestTimeout)
	defer cancel()

	snapshot := domain.ContextSnapshot{
		WorkingDir: ".",
		Shell:      "sh",
		OS:         runtime.GOOS,
	}

	_, err = provider.Generate(testCtx, ports.ProviderRequest{
		Prompt:  "echo testing",
		Context: snapshot,
		Model:   model,
	})

	if err != nil {
		return fmt.Errorf("model %s test failed: %w", modelName, err)
	}

	fmt.Fprintf(out, "Model %s responded successfully.\n", modelName)
	return nil
}

// setDefaultModel sets the default model
func setDefaultModel(ctx context.Context, container *app.Container, modelName string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.SetDefaultModel(modelName); err != nil {
		return err
	}

	return helpers.SaveConfigWithValidation(container, cfg)
}

// addModel adds a new model definition
func addModel(ctx context.Context, container *app.Container, opts modelAddOptions) error {
	if err := validateModelAddOptions(opts); err != nil {
		return err
	}

	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load prompt messages if prompt file is provided
	var prompts []domain.PromptMessage
	if opts.PromptFile != "" {
		prompts, err = helpers.LoadPromptMessagesFromFile(opts.PromptFile)
		if err != nil {
			return err
		}
	}

	// Create new model definition
	model := domain.ModelDefinition{
		Name:       opts.Name,
		Endpoint:   opts.Endpoint,
		ModelID:    opts.ModelID,
		AuthEnvVar: opts.AuthEnv,
		OrgEnvVar:  opts.OrgEnv,
		MaxTokens:  opts.MaxTokens,
		Prompt:     prompts,
	}

	// Add model to configuration
	if err := cfg.AddModel(model); err != nil {
		return err
	}

	return helpers.SaveConfigWithValidation(container, cfg)
}

// removeModel removes a model definition
func removeModel(ctx context.Context, container *app.Container, modelName string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.RemoveModel(modelName); err != nil {
		return err
	}

	return helpers.SaveConfigWithValidation(container, cfg)
}

// validateModelAddOptions validates the options for adding a model
func validateModelAddOptions(opts modelAddOptions) error {
	if opts.Name == "" || opts.Endpoint == "" {
		return fmt.Errorf("--name and --endpoint are required")
	}

	if opts.MaxTokens <= 0 {
		return fmt.Errorf("max-tokens must be positive, got %d", opts.MaxTokens)
	}

	return nil
}

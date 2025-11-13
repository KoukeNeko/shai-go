package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	configapp "github.com/doeshing/shai-go/internal/application/config"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure/config"
)

const (
	envKeyEditor              = "EDITOR"
	defaultEditor             = "vi"
	msgConfigurationValid     = "Configuration valid"
	msgNoDifferencesFromDefault = "No differences from default configuration."
)

// NewConfigCommand creates the config command with all subcommands
func NewConfigCommand(container *app.Container) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect SHAI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfiguration(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	configCmd.AddCommand(
		newConfigShowCommand(container),
		newConfigGetCommand(container),
		newConfigSetCommand(container),
		newConfigEditCommand(container),
		newConfigValidateCommand(container),
		newConfigResetCommand(container),
		newConfigDiffCommand(container),
	)

	return configCmd
}

// newConfigShowCommand creates the 'config show' subcommand
func newConfigShowCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show full configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfiguration(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newConfigGetCommand creates the 'config get' subcommand
func newConfigGetCommand(container *app.Container) *cobra.Command {
	var key string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a specific configuration value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			return getConfigurationValue(cmd.Context(), cmd.OutOrStdout(), container, key)
		},
	}

	cmd.Flags().StringVar(&key, "key", "", "Key path (e.g., preferences.default_model)")
	return cmd
}

// newConfigSetCommand creates the 'config set' subcommand
func newConfigSetCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (value accepts YAML syntax)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := strings.Join(args[1:], " ")
			return setConfigurationValue(cmd.Context(), container, key, value)
		},
	}
}

// newConfigEditCommand creates the 'config edit' subcommand
func newConfigEditCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return editConfigurationInEditor(container)
		},
	}
}

// newConfigValidateCommand creates the 'config validate' subcommand
func newConfigValidateCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := container.ConfigProvider.Load(cmd.Context()); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), msgConfigurationValid)
			return nil
		},
	}
}

// newConfigResetCommand creates the 'config reset' subcommand
func newConfigResetCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			return resetConfigurationToDefaults(cmd.OutOrStdout(), container)
		},
	}
}

// newConfigDiffCommand creates the 'config diff' subcommand
func newConfigDiffCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show diff versus default configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfigurationDiff(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// showConfiguration displays the full configuration in YAML format
func showConfiguration(ctx context.Context, out io.Writer, container *app.Container) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	fmt.Fprint(out, string(data))
	return nil
}

// getConfigurationValue retrieves a specific configuration value by key path
func getConfigurationValue(ctx context.Context, out io.Writer, container *app.Container, keyPath string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Convert config to generic map for traversal
	genericMap, err := convertConfigToGenericMap(cfg)
	if err != nil {
		return err
	}

	// Traverse the key path
	keys := strings.Split(keyPath, ".")
	value, found := helpers.TraverseNestedMap(genericMap, keys)
	if !found {
		return fmt.Errorf("key %s not found in configuration", keyPath)
	}

	// Marshal the value as YAML
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	fmt.Fprint(out, string(data))
	return nil
}

// setConfigurationValue updates a configuration value by key path
func setConfigurationValue(ctx context.Context, container *app.Container, keyPath string, value string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Convert to map for manipulation
	cfgMap, err := convertDomainConfigToMap(cfg)
	if err != nil {
		return err
	}

	// Parse and set the value
	parsedValue, err := helpers.ParseYAMLValue(value)
	if err != nil {
		return fmt.Errorf("failed to parse value: %w", err)
	}

	keys := strings.Split(keyPath, ".")
	if !helpers.SetNestedMapValue(cfgMap, keys, parsedValue) {
		return fmt.Errorf("unable to set key %s", keyPath)
	}

	// Convert back to domain.Config
	updatedConfig, err := convertMapToDomainConfig(cfgMap)
	if err != nil {
		return err
	}

	// Validate and save
	return helpers.SaveConfigWithValidation(container, updatedConfig)
}

// editConfigurationInEditor opens the configuration file in the user's editor
func editConfigurationInEditor(container *app.Container) error {
	loader, err := helpers.GetConfigLoader(container)
	if err != nil {
		return err
	}

	editorCommand := getEditorCommand()
	cmd := exec.Command(editorCommand, loader.Path())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor %s: %w", editorCommand, err)
	}

	return nil
}

// resetConfigurationToDefaults resets the configuration to default values
func resetConfigurationToDefaults(out io.Writer, container *app.Container) error {
	loader, err := helpers.GetConfigLoader(container)
	if err != nil {
		return err
	}

	defaultConfig, err := loader.Reset()
	if err != nil {
		return fmt.Errorf("failed to reset configuration: %w", err)
	}

	fmt.Fprintf(out, "Configuration reset at %s\n", loader.Path())

	data, _ := yaml.Marshal(defaultConfig)
	fmt.Fprint(out, string(data))

	return nil
}

// showConfigurationDiff shows the difference between current and default configuration
func showConfigurationDiff(ctx context.Context, out io.Writer, container *app.Container) error {
	currentConfig, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load current configuration: %w", err)
	}

	defaultConfig := configinfra.DefaultConfig()
	diff := cmp.Diff(defaultConfig, currentConfig)

	if diff == "" {
		fmt.Fprintln(out, msgNoDifferencesFromDefault)
		return nil
	}

	fmt.Fprintln(out, diff)
	return nil
}

// Helper functions

// getEditorCommand retrieves the editor command from environment or returns default
func getEditorCommand() string {
	if editor := os.Getenv(envKeyEditor); editor != "" {
		return editor
	}
	return defaultEditor
}

// convertConfigToGenericMap converts domain.Config to a generic map for traversal
func convertConfigToGenericMap(cfg domain.Config) (interface{}, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to generic map: %w", err)
	}

	return generic, nil
}

// convertDomainConfigToMap converts domain.Config to map[string]interface{}
func convertDomainConfigToMap(cfg domain.Config) (map[string]interface{}, error) {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var cfgMap map[string]interface{}
	if err := yaml.Unmarshal(raw, &cfgMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return cfgMap, nil
}

// convertMapToDomainConfig converts map[string]interface{} to domain.Config
func convertMapToDomainConfig(cfgMap map[string]interface{}) (domain.Config, error) {
	updatedRaw, err := yaml.Marshal(cfgMap)
	if err != nil {
		return domain.Config{}, fmt.Errorf("failed to marshal updated map: %w", err)
	}

	var updated domain.Config
	if err := yaml.Unmarshal(updatedRaw, &updated); err != nil {
		return domain.Config{}, fmt.Errorf("failed to unmarshal to Config: %w", err)
	}

	if err := configapp.Validate(updated); err != nil {
		return domain.Config{}, fmt.Errorf("validation failed: %w", err)
	}

	return updated, nil
}

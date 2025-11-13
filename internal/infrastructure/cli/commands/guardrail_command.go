package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/infrastructure"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/helpers"
)

// NewGuardrailCommand creates the guardrail command with enable/disable subcommands
func NewGuardrailCommand(container *app.Container) *cobra.Command {
	guardrailCmd := &cobra.Command{
		Use:   "guardrail",
		Short: "Manage security guardrails",
	}

	guardrailCmd.AddCommand(
		newGuardrailEnableCommand(container),
		newGuardrailDisableCommand(container),
		newGuardrailStatusCommand(container),
	)

	return guardrailCmd
}

// newGuardrailEnableCommand enables security guardrails
func newGuardrailEnableCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable security guardrails",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setGuardrailState(cmd.Context(), container, true)
		},
	}
}

// newGuardrailDisableCommand disables security guardrails
func newGuardrailDisableCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable security guardrails (not recommended)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setGuardrailState(cmd.Context(), container, false)
		},
	}
}

// newGuardrailStatusCommand shows current guardrail status
func newGuardrailStatusCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show guardrail status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showGuardrailStatus(cmd.Context(), container)
		},
	}
}

// setGuardrailState enables or disables guardrails
func setGuardrailState(ctx context.Context, container *app.Container, enabled bool) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg.Security.Enabled = enabled

	if err := helpers.SaveConfigWithValidation(container, cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	fmt.Printf("Guardrails %s successfully.\n", status)
	return nil
}

// showGuardrailStatus displays the current guardrail status
func showGuardrailStatus(ctx context.Context, container *app.Container) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	status := "disabled"
	if cfg.Security.Enabled {
		status = "enabled"
	}

	fmt.Printf("Guardrails are currently %s.\n", status)
	if cfg.Security.Enabled {
		rulesPath := infrastructure.ResolveRulesPath(cfg.Security.RulesFile)
		fmt.Printf("Rules file: %s\n", rulesPath)
	}

	return nil
}

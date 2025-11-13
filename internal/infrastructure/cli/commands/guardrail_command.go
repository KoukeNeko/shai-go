package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure"
)

const (
	msgWhitelistEmpty = "Whitelist is empty."
)

// NewGuardrailCommand creates the guardrail command with all subcommands
func NewGuardrailCommand(container *app.Container) *cobra.Command {
	guardrailCmd := &cobra.Command{
		Use:   "guardrail",
		Short: "Inspect and edit guardrail policy",
	}

	guardrailCmd.AddCommand(
		newGuardrailShowCommand(container),
		newGuardrailValidateCommand(container),
		newGuardrailWhitelistCommand(container),
		newGuardrailConfirmCommand(container),
		newGuardrailPreviewCommand(container),
	)

	return guardrailCmd
}

// newGuardrailShowCommand creates the 'guardrail show' subcommand
func newGuardrailShowCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display guardrail policy document",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showGuardrailPolicy(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newGuardrailValidateCommand creates the 'guardrail validate' subcommand
func newGuardrailValidateCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate guardrail file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateGuardrailPolicy(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newGuardrailWhitelistCommand creates the 'guardrail whitelist' subcommand group
func newGuardrailWhitelistCommand(container *app.Container) *cobra.Command {
	whitelistCmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage guardrail whitelist",
	}

	whitelistCmd.AddCommand(
		newWhitelistAddCommand(container),
		newWhitelistRemoveCommand(container),
		newWhitelistListCommand(container),
	)

	return whitelistCmd
}

// newWhitelistAddCommand creates the 'guardrail whitelist add' subcommand
func newWhitelistAddCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "add <command>",
		Short: "Add command to whitelist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addToWhitelist(cmd.Context(), container, args[0])
		},
	}
}

// newWhitelistRemoveCommand creates the 'guardrail whitelist remove' subcommand
func newWhitelistRemoveCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <command>",
		Short: "Remove command from whitelist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeFromWhitelist(cmd.Context(), container, args[0])
		},
	}
}

// newWhitelistListCommand creates the 'guardrail whitelist list' subcommand
func newWhitelistListCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List whitelisted commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listWhitelist(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
}

// newGuardrailConfirmCommand creates the 'guardrail confirm' subcommand
func newGuardrailConfirmCommand(container *app.Container) *cobra.Command {
	var confAction, confMessage string

	cmd := &cobra.Command{
		Use:   "confirm set <level>",
		Short: "Set confirmation action/message for a risk level",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if confAction == "" {
				return fmt.Errorf("--action is required")
			}
			return setConfirmationLevel(cmd.Context(), container, args[0], confAction, confMessage)
		},
	}

	cmd.Flags().StringVar(&confAction, "action", "", "Action (allow/preview_only/simple_confirm/confirm/explicit_confirm/block)")
	cmd.Flags().StringVar(&confMessage, "message", "", "Confirmation message")

	return cmd
}

// newGuardrailPreviewCommand creates the 'guardrail preview' subcommand
func newGuardrailPreviewCommand(container *app.Container) *cobra.Command {
	var previewMax int

	cmd := &cobra.Command{
		Use:   "preview set",
		Short: "Set preview max files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setPreviewMaxFiles(cmd.Context(), container, previewMax)
		},
	}

	cmd.Flags().IntVar(&previewMax, "max-files", domain.DefaultPreviewMaxFiles, "Maximum files to preview")

	return cmd
}

// showGuardrailPolicy displays the guardrail policy document
func showGuardrailPolicy(ctx context.Context, out io.Writer, container *app.Container) error {
	doc, path, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Policy file: %s\n", path)

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal policy document: %w", err)
	}

	fmt.Fprint(out, string(data))
	return nil
}

// validateGuardrailPolicy validates the guardrail policy file
func validateGuardrailPolicy(ctx context.Context, out io.Writer, container *app.Container) error {
	path, err := resolveGuardrailPath(ctx, container)
	if err != nil {
		return err
	}

	if _, err := infrastructure.NewGuardrail(path); err != nil {
		return fmt.Errorf("guardrail policy validation failed: %w", err)
	}

	fmt.Fprintf(out, "Guardrail policy at %s is valid.\n", path)
	return nil
}

// addToWhitelist adds a command to the whitelist
func addToWhitelist(ctx context.Context, container *app.Container, entry string) error {
	doc, path, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	entry = strings.TrimSpace(entry)
	if entry == "" {
		return fmt.Errorf("whitelist entry cannot be empty")
	}

	// Check if already exists
	for _, existing := range doc.Rules.Whitelist {
		if existing == entry {
			return fmt.Errorf("%s already in whitelist", entry)
		}
	}

	doc.Rules.Whitelist = append(doc.Rules.Whitelist, entry)

	if err := infrastructure.SavePolicyDocument(path, doc); err != nil {
		return fmt.Errorf("failed to save policy document: %w", err)
	}

	return nil
}

// removeFromWhitelist removes a command from the whitelist
func removeFromWhitelist(ctx context.Context, container *app.Container, entry string) error {
	doc, path, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	entry = strings.TrimSpace(entry)

	var filtered []string
	found := false

	for _, existing := range doc.Rules.Whitelist {
		if existing == entry {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}

	if !found {
		return fmt.Errorf("%s not found in whitelist", entry)
	}

	doc.Rules.Whitelist = filtered

	if err := infrastructure.SavePolicyDocument(path, doc); err != nil {
		return fmt.Errorf("failed to save policy document: %w", err)
	}

	return nil
}

// listWhitelist lists all whitelisted commands
func listWhitelist(ctx context.Context, out io.Writer, container *app.Container) error {
	doc, _, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	if len(doc.Rules.Whitelist) == 0 {
		fmt.Fprintln(out, msgWhitelistEmpty)
		return nil
	}

	for _, entry := range doc.Rules.Whitelist {
		fmt.Fprintln(out, entry)
	}

	return nil
}

// setConfirmationLevel sets the confirmation action and message for a risk level
func setConfirmationLevel(ctx context.Context, container *app.Container, level string, action string, message string) error {
	doc, path, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	if doc.Rules.Confirmation == nil {
		doc.Rules.Confirmation = map[string]domain.ConfirmationLevel{}
	}

	doc.Rules.Confirmation[level] = domain.ConfirmationLevel{
		Action:  action,
		Message: message,
	}

	if err := infrastructure.SavePolicyDocument(path, doc); err != nil {
		return fmt.Errorf("failed to save policy document: %w", err)
	}

	return nil
}

// setPreviewMaxFiles sets the maximum number of files for preview
func setPreviewMaxFiles(ctx context.Context, container *app.Container, maxFiles int) error {
	if maxFiles < domain.MinPreviewMaxFiles {
		return fmt.Errorf("max-files must be >= %d", domain.MinPreviewMaxFiles)
	}

	doc, path, err := loadGuardrailDocument(ctx, container)
	if err != nil {
		return err
	}

	doc.Rules.Preview.MaxFiles = maxFiles

	if err := infrastructure.SavePolicyDocument(path, doc); err != nil {
		return fmt.Errorf("failed to save policy document: %w", err)
	}

	return nil
}

// resolveGuardrailPath resolves the path to the guardrail policy file
func resolveGuardrailPath(ctx context.Context, container *app.Container) (string, error) {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load configuration: %w", err)
	}

	return infrastructure.ResolveRulesPath(cfg.Security.RulesFile), nil
}

// loadGuardrailDocument loads the guardrail policy document
func loadGuardrailDocument(ctx context.Context, container *app.Container) (infrastructure.PolicyDocument, string, error) {
	path, err := resolveGuardrailPath(ctx, container)
	if err != nil {
		return infrastructure.PolicyDocument{}, "", err
	}

	doc, err := infrastructure.LoadPolicyDocument(path)
	if err != nil {
		return infrastructure.PolicyDocument{}, "", fmt.Errorf("failed to load policy document: %w", err)
	}

	return doc, path, nil
}

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
)

func newInstallCommand(container *app.Container) *cobra.Command {
	var shell string
	var force bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install SHAI shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.ShellIntegrator == nil {
				return fmt.Errorf("shell installer unavailable")
			}
			res, err := container.ShellIntegrator.Install(shell, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed for %s\nScript: %s\nRC File: %s\n", res.Shell, res.ScriptPath, res.RCFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "", "Shell to install (zsh|bash, auto-detected by default)")
	cmd.Flags().BoolVar(&force, "force", false, "Force rewrite of rc entry")
	return cmd
}

func newUninstallCommand(container *app.Container) *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove SHAI shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.ShellIntegrator == nil {
				return fmt.Errorf("shell installer unavailable")
			}
			res, err := container.ShellIntegrator.Uninstall(shell)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed sourcing line for %s in %s\n", res.Shell, res.RCFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "", "Shell to uninstall (zsh|bash, auto-detected by default)")
	return cmd
}

func newDoctorCommand(container *app.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose environment setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.DoctorService == nil {
				return fmt.Errorf("doctor service unavailable")
			}
			report, err := container.DoctorService.Run(cmd.Context())
			renderDoctorReport(cmd.OutOrStdout(), report)
			return err
		},
	}
}

func newConfigCommand(container *app.Container) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect SHAI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show full configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	var key string
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a specific configuration value",
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			return runConfigGet(cmd.Context(), cmd.OutOrStdout(), container, key)
		},
	}
	getCmd.Flags().StringVar(&key, "key", "", "Key path (e.g., preferences.default_model)")

	configCmd.AddCommand(showCmd, getCmd)
	return configCmd
}

func runConfigShow(ctx context.Context, out io.Writer, container *app.Container) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	fmt.Fprint(out, string(data))
	return nil
}

func runConfigGet(ctx context.Context, out io.Writer, container *app.Container, key string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	value, ok := traverseKey(generic, strings.Split(key, "."))
	if !ok {
		return fmt.Errorf("key %s not found", key)
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	fmt.Fprint(out, string(data))
	return nil
}

func traverseKey(data interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 {
		return data, true
	}
	switch node := data.(type) {
	case map[string]interface{}:
		next, ok := node[path[0]]
		if !ok {
			return nil, false
		}
		return traverseKey(next, path[1:])
	default:
		return nil, false
	}
}

func renderDoctorReport(out io.Writer, report domain.HealthReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(out, "[%s] %s - %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Details)
	}
}

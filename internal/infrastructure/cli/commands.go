package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/config"
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

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (value accepts YAML syntax)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := strings.Join(args[1:], " ")
			return runConfigSet(cmd.Context(), container, key, value)
		},
	}

	editCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEdit(container)
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := container.ConfigProvider.Load(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration valid")
			return nil
		},
	}

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			loader, err := configLoader(container)
			if err != nil {
				return err
			}
			cfg, err := loader.Reset()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Configuration reset at %s\n", loader.Path())
			data, _ := yaml.Marshal(cfg)
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	configCmd.AddCommand(showCmd, getCmd, setCmd, editCmd, validateCmd, resetCmd)
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

func runConfigSet(ctx context.Context, container *app.Container, key, value string) error {
	loader, err := configLoader(container)
	if err != nil {
		return err
	}
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	cfgMap := map[string]interface{}{}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(raw, &cfgMap); err != nil {
		return err
	}

	parsedValue, err := parseValue(value)
	if err != nil {
		return err
	}
	if !setMapValue(cfgMap, strings.Split(key, "."), parsedValue) {
		return fmt.Errorf("unable to set key %s", key)
	}

	updatedRaw, err := yaml.Marshal(cfgMap)
	if err != nil {
		return err
	}

	var updated domain.Config
	if err := yaml.Unmarshal(updatedRaw, &updated); err != nil {
		return err
	}

	return loader.Save(updated)
}

func runConfigEdit(container *app.Container) error {
	loader, err := configLoader(container)
	if err != nil {
		return err
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, loader.Path())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func configLoader(container *app.Container) (*config.FileLoader, error) {
	if container.ConfigLoader == nil {
		return nil, fmt.Errorf("config loader unavailable")
	}
	return container.ConfigLoader, nil
}

func parseValue(input string) (interface{}, error) {
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(input), &parsed); err != nil {
		return input, nil
	}
	return parsed, nil
}

func setMapValue(root map[string]interface{}, path []string, value interface{}) bool {
	if len(path) == 0 {
		return false
	}
	current := root
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		next, ok := current[key]
		if !ok {
			newChild := map[string]interface{}{}
			current[key] = newChild
			current = newChild
			continue
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			child = map[string]interface{}{}
			current[key] = child
		}
		current = child
	}
	current[path[len(path)-1]] = value
	return true
}

func newHistoryCommand(container *app.Container) *cobra.Command {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect SHAI history",
	}

	var limit int
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recent history entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := container.HistoryStore
			if store == nil {
				return fmt.Errorf("history store unavailable")
			}
			records, err := store.Records()
			if err != nil {
				return err
			}
			sort.Slice(records, func(i, j int) bool {
				return records[i].Timestamp.After(records[j].Timestamp)
			})
			if limit > 0 && len(records) > limit {
				records = records[:limit]
			}
			for _, rec := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%s | %s | %s | %s\n",
					rec.Timestamp.Format(time.RFC3339),
					rec.Model,
					rec.RiskLevel,
					rec.Command)
			}
			return nil
		},
	}
	listCmd.Flags().IntVar(&limit, "limit", 20, "Max entries to show")

	var query string
	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search history for a keyword",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := container.HistoryStore
			if store == nil {
				return fmt.Errorf("history store unavailable")
			}
			if query == "" {
				return fmt.Errorf("--query required")
			}
			records, err := store.Records()
			if err != nil {
				return err
			}
			for _, rec := range records {
				if strings.Contains(rec.Command, query) || strings.Contains(rec.Prompt, query) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s | %s\n", rec.Timestamp.Format(time.RFC3339), rec.Command)
				}
			}
			return nil
		},
	}
	searchCmd.Flags().StringVar(&query, "query", "", "Search keyword")

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear history file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.HistoryStore == nil {
				return fmt.Errorf("history store unavailable")
			}
			return container.HistoryStore.Clear()
		},
	}

	exportCmd := &cobra.Command{
		Use:   "export <path>",
		Short: "Export history jsonl file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := container.HistoryStore
			if store == nil {
				return fmt.Errorf("history store unavailable")
			}
			src := store.Path()
			dest := args[0]
			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			return os.WriteFile(dest, data, 0o644)
		},
	}

	historyCmd.AddCommand(listCmd, searchCmd, clearCmd, exportCmd)
	return historyCmd
}

func newCacheCommand(container *app.Container) *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect or clear response cache",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List cache entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.CacheStore == nil {
				return fmt.Errorf("cache store unavailable")
			}
			entries, err := container.CacheStore.Entries()
			if err != nil {
				return err
			}
			for _, entry := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s | %s | %s\n", entry.Key, entry.Model, entry.CreatedAt.Format(time.RFC3339))
			}
			return nil
		},
	}

	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear cache directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.CacheStore == nil {
				return fmt.Errorf("cache store unavailable")
			}
			return container.CacheStore.Clear()
		},
	}

	sizeCmd := &cobra.Command{
		Use:   "size",
		Short: "Show cache size",
		RunE: func(cmd *cobra.Command, args []string) error {
			if container.CacheStore == nil {
				return fmt.Errorf("cache store unavailable")
			}
			dir := container.CacheStore.Dir()
			var total int64
			filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				info, err := d.Info()
				if err == nil {
					total += info.Size()
				}
				return nil
			})
			fmt.Fprintf(cmd.OutOrStdout(), "Cache directory: %s\nSize: %d bytes\n", dir, total)
			return nil
		},
	}

	cacheCmd.AddCommand(listCmd, clearCmd, sizeCmd)
	return cacheCmd
}

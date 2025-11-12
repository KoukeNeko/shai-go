package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/ai"
	"github.com/doeshing/shai-go/internal/infrastructure/config"
	"github.com/doeshing/shai-go/internal/infrastructure/security"
	"github.com/doeshing/shai-go/internal/ports"
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
			records, err := store.Records(limit, "")
			if err != nil {
				return err
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
	var searchLimit int
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
			records, err := store.Records(searchLimit, query)
			if err != nil {
				return err
			}
			for _, rec := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%s | %s\n", rec.Timestamp.Format(time.RFC3339), rec.Command)
			}
			return nil
		},
	}
	searchCmd.Flags().StringVar(&query, "query", "", "Search keyword")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Limit search results")

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
			return store.ExportJSON(args[0])
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

func runModelsList(ctx context.Context, out io.Writer, container *app.Container) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "NAME\tMODEL ID\tENDPOINT\tDEFAULT\n")
	for _, model := range cfg.Models {
		def := ""
		if cfg.Preferences.DefaultModel == model.Name {
			def = "*"
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", model.Name, model.ModelID, model.Endpoint, def)
	}
	if len(cfg.Preferences.FallbackModels) > 0 {
		fmt.Fprintf(out, "Fallbacks: %s\n", strings.Join(cfg.Preferences.FallbackModels, ", "))
	}
	return nil
}

func runModelsTest(ctx context.Context, out io.Writer, container *app.Container, name string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	model, ok := findModel(cfg, name)
	if !ok {
		return fmt.Errorf("model %s not found", name)
	}
	provider, err := ai.NewFactory().ForModel(model)
	if err != nil {
		return err
	}
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
		return err
	}
	fmt.Fprintf(out, "Model %s responded successfully.\n", name)
	return nil
}

func runModelsUse(ctx context.Context, container *app.Container, name string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	if _, ok := findModel(cfg, name); !ok {
		return fmt.Errorf("model %s not found", name)
	}
	cfg.Preferences.DefaultModel = name
	return saveConfig(container, cfg)
}

type modelAddOptions struct {
	Name       string
	Endpoint   string
	ModelID    string
	AuthEnv    string
	OrgEnv     string
	PromptFile string
	MaxTokens  int
}

func runModelsAdd(ctx context.Context, container *app.Container, opts modelAddOptions) error {
	if opts.Name == "" || opts.Endpoint == "" {
		return fmt.Errorf("--name and --endpoint are required")
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 1024
	}
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	if _, ok := findModel(cfg, opts.Name); ok {
		return fmt.Errorf("model %s already exists", opts.Name)
	}
	var prompts []domain.PromptMessage
	if opts.PromptFile != "" {
		prompts, err = loadPromptFile(opts.PromptFile)
		if err != nil {
			return err
		}
	}
	model := domain.ModelDefinition{
		Name:       opts.Name,
		Endpoint:   opts.Endpoint,
		ModelID:    opts.ModelID,
		AuthEnvVar: opts.AuthEnv,
		OrgEnvVar:  opts.OrgEnv,
		MaxTokens:  opts.MaxTokens,
		Prompt:     prompts,
	}
	cfg.Models = append(cfg.Models, model)
	return saveConfig(container, cfg)
}

func runModelsRemove(ctx context.Context, container *app.Container, name string) error {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return err
	}
	index := -1
	for i, model := range cfg.Models {
		if model.Name == name {
			index = i
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("model %s not found", name)
	}
	cfg.Models = append(cfg.Models[:index], cfg.Models[index+1:]...)
	if cfg.Preferences.DefaultModel == name {
		if len(cfg.Models) > 0 {
			cfg.Preferences.DefaultModel = cfg.Models[0].Name
		} else {
			cfg.Preferences.DefaultModel = ""
		}
	}
	var filtered []string
	for _, fallback := range cfg.Preferences.FallbackModels {
		if fallback == name {
			continue
		}
		filtered = append(filtered, fallback)
	}
	cfg.Preferences.FallbackModels = filtered
	return saveConfig(container, cfg)
}

func saveConfig(container *app.Container, cfg domain.Config) error {
	loader, err := configLoader(container)
	if err != nil {
		return err
	}
	return loader.Save(cfg)
}

func loadPromptFile(path string) ([]domain.PromptMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var prompts []domain.PromptMessage
	if err := yaml.Unmarshal(data, &prompts); err != nil {
		return nil, err
	}
	return prompts, nil
}

func findModel(cfg domain.Config, name string) (domain.ModelDefinition, bool) {
	for _, model := range cfg.Models {
		if model.Name == name {
			return model, true
		}
	}
	return domain.ModelDefinition{}, false
}

func runGuardrailShow(ctx context.Context, out io.Writer, container *app.Container) error {
	doc, path, err := loadGuardrailDoc(ctx, container)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Policy file: %s\n", path)
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	fmt.Fprint(out, string(data))
	return nil
}

func runGuardrailValidate(ctx context.Context, out io.Writer, container *app.Container) error {
	path, err := guardrailPath(ctx, container)
	if err != nil {
		return err
	}
	if _, err := security.NewGuardrail(path); err != nil {
		return err
	}
	fmt.Fprintf(out, "Guardrail policy at %s is valid.\n", path)
	return nil
}

func runGuardrailWhitelistAdd(ctx context.Context, container *app.Container, entry string) error {
	doc, path, err := loadGuardrailDoc(ctx, container)
	if err != nil {
		return err
	}
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return fmt.Errorf("whitelist entry cannot be empty")
	}
	for _, existing := range doc.Rules.Whitelist {
		if existing == entry {
			return fmt.Errorf("%s already in whitelist", entry)
		}
	}
	doc.Rules.Whitelist = append(doc.Rules.Whitelist, entry)
	return security.SavePolicyDocument(path, doc)
}

func runGuardrailWhitelistRemove(ctx context.Context, container *app.Container, entry string) error {
	doc, path, err := loadGuardrailDoc(ctx, container)
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
	return security.SavePolicyDocument(path, doc)
}

func runGuardrailWhitelistList(ctx context.Context, out io.Writer, container *app.Container) error {
	doc, _, err := loadGuardrailDoc(ctx, container)
	if err != nil {
		return err
	}
	if len(doc.Rules.Whitelist) == 0 {
		fmt.Fprintln(out, "Whitelist is empty.")
		return nil
	}
	for _, entry := range doc.Rules.Whitelist {
		fmt.Fprintln(out, entry)
	}
	return nil
}

func runGuardrailConfirmSet(ctx context.Context, container *app.Container, level string, action string, message string) error {
	doc, path, err := loadGuardrailDoc(ctx, container)
	if err != nil {
		return err
	}
	if doc.Rules.Confirmation == nil {
		doc.Rules.Confirmation = map[string]domain.ConfirmationLevel{}
	}
	doc.Rules.Confirmation[level] = domain.ConfirmationLevel{Action: action, Message: message}
	return security.SavePolicyDocument(path, doc)
}

func runGuardrailPreviewSet(ctx context.Context, container *app.Container, max int) error {
	if max <= 0 {
		return fmt.Errorf("max-files must be >= 1")
	}
	doc, path, err := loadGuardrailDoc(ctx, container)
	if err != nil {
		return err
	}
	doc.Rules.Preview.MaxFiles = max
	return security.SavePolicyDocument(path, doc)
}

func guardrailPath(ctx context.Context, container *app.Container) (string, error) {
	cfg, err := container.ConfigProvider.Load(ctx)
	if err != nil {
		return "", err
	}
	return security.ResolveRulesPath(cfg.Security.RulesFile), nil
}

func loadGuardrailDoc(ctx context.Context, container *app.Container) (security.PolicyDocument, string, error) {
	path, err := guardrailPath(ctx, container)
	if err != nil {
		return security.PolicyDocument{}, "", err
	}
	doc, err := security.LoadPolicyDocument(path)
	if err != nil {
		return security.PolicyDocument{}, "", err
	}
	return doc, path, nil
}

func newGuardrailCommand(container *app.Container) *cobra.Command {
	guardrailCmd := &cobra.Command{
		Use:   "guardrail",
		Short: "Inspect and edit guardrail policy",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Display guardrail policy document",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailShow(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate guardrail file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailValidate(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	whitelistCmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage guardrail whitelist",
	}
	addWhitelist := &cobra.Command{
		Use:   "add <command>",
		Short: "Add command to whitelist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailWhitelistAdd(cmd.Context(), container, args[0])
		},
	}
	removeWhitelist := &cobra.Command{
		Use:   "remove <command>",
		Short: "Remove command from whitelist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailWhitelistRemove(cmd.Context(), container, args[0])
		},
	}
	listWhitelist := &cobra.Command{
		Use:   "list",
		Short: "List whitelisted commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailWhitelistList(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}
	whitelistCmd.AddCommand(addWhitelist, removeWhitelist, listWhitelist)

	confirmCmd := &cobra.Command{
		Use:   "confirm set <level>",
		Short: "Set confirmation action/message for a risk level",
		Args:  cobra.ExactArgs(1),
	}
	var confAction, confMessage string
	confirmCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if confAction == "" {
			return fmt.Errorf("--action is required")
		}
		return runGuardrailConfirmSet(cmd.Context(), container, args[0], confAction, confMessage)
	}
	confirmCmd.Flags().StringVar(&confAction, "action", "", "Action (allow/preview_only/simple_confirm/confirm/explicit_confirm/block)")
	confirmCmd.Flags().StringVar(&confMessage, "message", "", "Confirmation message")

	var previewMax int
	previewCmd := &cobra.Command{
		Use:   "preview set",
		Short: "Set preview max files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardrailPreviewSet(cmd.Context(), container, previewMax)
		},
	}
	previewCmd.Flags().IntVar(&previewMax, "max-files", 10, "Maximum files to preview")

	guardrailCmd.AddCommand(showCmd, validateCmd, whitelistCmd, confirmCmd, previewCmd)
	return guardrailCmd
}

func newModelsCommand(container *app.Container) *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Manage AI model configurations",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsList(cmd.Context(), cmd.OutOrStdout(), container)
		},
	}

	testCmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Test connectivity for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsTest(cmd.Context(), cmd.OutOrStdout(), container, args[0])
		},
	}

	useCmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Set default model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsUse(cmd.Context(), container, args[0])
		},
	}

	var (
		addName       string
		addEndpoint   string
		addModelID    string
		addAuthEnv    string
		addOrgEnv     string
		addPromptFile string
		addMaxTokens  int
	)

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new model definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := modelAddOptions{
				Name:       addName,
				Endpoint:   addEndpoint,
				ModelID:    addModelID,
				AuthEnv:    addAuthEnv,
				OrgEnv:     addOrgEnv,
				PromptFile: addPromptFile,
				MaxTokens:  addMaxTokens,
			}
			return runModelsAdd(cmd.Context(), container, opts)
		},
	}
	addCmd.Flags().StringVar(&addName, "name", "", "Model name (identifier)")
	addCmd.Flags().StringVar(&addEndpoint, "endpoint", "", "Provider endpoint URL")
	addCmd.Flags().StringVar(&addModelID, "model-id", "", "Model identifier at provider")
	addCmd.Flags().StringVar(&addAuthEnv, "auth-env", "", "Environment variable containing API key")
	addCmd.Flags().StringVar(&addOrgEnv, "org-env", "", "Environment variable containing org/project ID")
	addCmd.Flags().StringVar(&addPromptFile, "prompt-file", "", "Path to YAML prompt template for this model")
	addCmd.Flags().IntVar(&addMaxTokens, "max-tokens", 1024, "Max tokens for responses")

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove model definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsRemove(cmd.Context(), container, args[0])
		},
	}

	modelsCmd.AddCommand(listCmd, testCmd, useCmd, addCmd, removeCmd)
	return modelsCmd
}

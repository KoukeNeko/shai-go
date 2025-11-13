package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/infrastructure/cli/commands"
)

// Options holds CLI-level configuration.
type Options struct {
	Verbose bool
}

// NewRootCmd wires the cobra root command.
func NewRootCmd(ctx context.Context, opts Options) (*cobra.Command, error) {
	container, err := app.BuildContainer(ctx, opts.Verbose)
	if err != nil {
		return nil, err
	}
	container.QueryService.Prompter = NewPrompter(nil, nil)
	container.QueryService.Clipboard = NewClipboard()

	queryCmd := newQueryCommand(container)

	root := &cobra.Command{
		Use:   "shai [query]",
		Short: "SHAI - Shell AI assistant",
		Long:  "SHAI converts natural language to shell commands with safety guardrails.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			queryCmd.SetArgs(args)
			return queryCmd.ExecuteContext(cmd.Context())
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(queryCmd)
	root.AddCommand(commands.NewInstallCommand(container))
	root.AddCommand(commands.NewUninstallCommand(container))
	root.AddCommand(commands.NewConfigCommand(container))
	root.AddCommand(newDoctorCommand(container))
	root.AddCommand(commands.NewHistoryCommand(container))
	root.AddCommand(commands.NewCacheCommand(container))
	root.AddCommand(commands.NewModelsCommand(container))
	root.AddCommand(commands.NewGuardrailCommand(container))
	root.AddCommand(commands.NewReloadCommand(container))
	root.AddCommand(commands.NewInitCommand(container))
	root.AddCommand(newVersionCommand())
	root.AddCommand(newUpdateCommand())
	return root, nil
}

func newQueryCommand(container *app.Container) *cobra.Command {
	var (
		model       string
		previewOnly bool
		autoExecute bool
		copyCmd     bool
		withGit     bool
		withEnv     bool
		withK8s     bool
		debug       bool
		timeout     time.Duration
		stream      bool
	)

	cmd := &cobra.Command{
		Use:   "query [natural language]",
		Short: "Generate a command from natural language",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			req := domain.QueryRequest{
				Context:         ctx,
				Prompt:          strings.Join(args, " "),
				ModelOverride:   model,
				PreviewOnly:     previewOnly,
				AutoExecute:     autoExecute,
				CopyToClipboard: copyCmd,
				WithGitStatus:   withGit,
				WithEnv:         withEnv,
				WithK8sInfo:     withK8s,
				Debug:           debug,
				Stream:          stream,
			}
			if stream {
				req.StreamWriter = NewStreamWriter(cmd.OutOrStdout())
			}
			resp, err := container.QueryService.Run(req)
			RenderResponse(resp)
			return err
		},
	}

	cmd.Flags().StringVarP(&model, "model", "m", "", "Override model name (default from config)")
	cmd.Flags().BoolVarP(&previewOnly, "preview-only", "p", false, "Only preview command, do not execute")
	cmd.Flags().BoolVarP(&autoExecute, "auto-execute", "a", false, "Auto execute without extra confirmation (still subject to guardrails)")
	cmd.Flags().BoolVarP(&copyCmd, "copy", "c", false, "Copy generated command to clipboard")
	cmd.Flags().BoolVar(&withGit, "with-git-status", false, "Force include git status")
	cmd.Flags().BoolVar(&withEnv, "with-env", false, "Include select environment variables")
	cmd.Flags().BoolVar(&withK8s, "with-k8s-info", false, "Include Kubernetes context")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable verbose logging")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "Override request timeout")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream provider reasoning output")

	return cmd
}

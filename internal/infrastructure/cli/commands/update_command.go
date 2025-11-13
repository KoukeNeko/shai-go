package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/doeshing/shai-go/internal/version"
)

const (
	releaseChannelStable = "stable"
	releaseChannelNightly = "nightly"
)

// NewUpdateCommand creates the update command
func NewUpdateCommand() *cobra.Command {
	var channel string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for new releases and update instructions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return displayUpdateInstructions(cmd.OutOrStdout(), channel)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", releaseChannelStable, "Release channel (stable/nightly)")

	return cmd
}

// displayUpdateInstructions displays update instructions
func displayUpdateInstructions(out io.Writer, channel string) error {
	fmt.Fprintf(out, "Current version: %s\n", version.Version)
	fmt.Fprintf(out, "Release channel: %s\n", channel)
	fmt.Fprintln(out, "Update instructions:")
	fmt.Fprintln(out, "  1. Visit https://github.com/doeshing/shai-go/releases for the latest binary.")
	fmt.Fprintln(out, "  2. Or run the install script: curl -sSL https://shai.dev/install.sh | bash")
	fmt.Fprintln(out, "  3. Homebrew: brew tap doeshing/shai && brew install shai")
	fmt.Fprintln(out, "  4. Debian/Ubuntu: sudo apt install shai (coming soon)")
	fmt.Fprintln(out, "After updating, run 'shai reload' to refresh shell integrations.")

	return nil
}

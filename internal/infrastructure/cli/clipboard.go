package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/doeshing/shai-go/internal/ports"
)

// Clipboard implements ports.Clipboard using platform-specific tools.
type Clipboard struct{}

// NewClipboard builds the clipboard helper.
func NewClipboard() *Clipboard {
	return &Clipboard{}
}

func (c *Clipboard) Enabled() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	default:
		return false
	}
}

// Copy copies text to the system clipboard.
func (c *Clipboard) Copy(text string) error {
	if !c.Enabled() {
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	default: // linux
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("clipboard utilities not found")
		}
	}
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

var _ ports.Clipboard = (*Clipboard)(nil)

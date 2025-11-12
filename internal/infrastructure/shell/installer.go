package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rootassets "github.com/doeshing/shai-go/assets"
	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// Installer handles shell script deployment.
type Installer struct {
	logger ports.Logger
}

// NewInstaller builds a shell installer.
func NewInstaller(logger ports.Logger) *Installer {
	return &Installer{logger: logger}
}

// Install installs shell integration for the given shell name (auto-detected when empty).
func (i *Installer) Install(shell string, force bool) (domain.ShellInstallResult, error) {
	name := normalizeShell(shell)
	scriptContent, err := scriptFor(name)
	if err != nil {
		return domain.ShellInstallResult{}, err
	}
	scriptPath, rcFile := scriptPaths(name)
	if scriptPath == "" || rcFile == "" {
		return domain.ShellInstallResult{}, fmt.Errorf("unsupported shell: %s", name)
	}
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		return domain.ShellInstallResult{}, err
	}

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644); err != nil {
		return domain.ShellInstallResult{}, err
	}

	rcUpdated, err := ensureRCLine(rcFile, sourceLine(scriptPath), force)
	if err != nil {
		return domain.ShellInstallResult{}, err
	}

	return domain.ShellInstallResult{
		Shell:         name,
		ScriptPath:    scriptPath,
		RCFile:        rcFile,
		ScriptUpdated: true,
		RCUpdated:     rcUpdated,
	}, nil
}

// Uninstall removes sourcing line from rc file (script retained as backup).
func (i *Installer) Uninstall(shell string) (domain.ShellInstallResult, error) {
	name := normalizeShell(shell)
	scriptPath, rcFile := scriptPaths(name)
	if scriptPath == "" || rcFile == "" {
		return domain.ShellInstallResult{}, fmt.Errorf("unsupported shell: %s", name)
	}
	updated, err := removeRCLine(rcFile, sourceLine(scriptPath))
	if err != nil {
		return domain.ShellInstallResult{}, err
	}
	return domain.ShellInstallResult{
		Shell:         name,
		ScriptPath:    scriptPath,
		RCFile:        rcFile,
		ScriptUpdated: false,
		RCUpdated:     updated,
	}, nil
}

// Status reports current integration state.
func (i *Installer) Status(shell string) domain.ShellStatus {
	name := normalizeShell(shell)
	scriptPath, rcFile := scriptPaths(name)
	status := domain.ShellStatus{
		Shell:      name,
		ScriptPath: scriptPath,
		RCFile:     rcFile,
	}
	if scriptPath == "" || rcFile == "" {
		status.Error = "unsupported shell"
		return status
	}

	if info, err := os.Stat(scriptPath); err == nil && info.Mode().IsRegular() {
		status.ScriptExists = true
	}

	line := sourceLine(scriptPath)
	if contents, err := os.ReadFile(rcFile); err == nil {
		status.LinePresent = strings.Contains(string(contents), line)
	}

	return status
}

// DetectShell inspects the SHELL env var.
func (i *Installer) DetectShell() string {
	return os.Getenv("SHELL")
}

func normalizeShell(shell string) domain.ShellName {
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	switch strings.ToLower(shell) {
	case "zsh":
		return domain.ShellZsh
	case "bash":
		return domain.ShellBash
	default:
		return domain.ShellUnknown
	}
}

func scriptFor(shell domain.ShellName) (string, error) {
	switch shell {
	case domain.ShellZsh:
		return rootassets.ZshHook, nil
	case domain.ShellBash:
		return rootassets.BashHook, nil
	default:
		return "", errors.New("unsupported shell")
	}
}

func scriptPaths(shell domain.ShellName) (string, string) {
	home := userHome()
	switch shell {
	case domain.ShellZsh:
		return filepath.Join(home, ".shai", "shell", "zsh.sh"), filepath.Join(home, ".zshrc")
	case domain.ShellBash:
		return filepath.Join(home, ".shai", "shell", "bash.sh"), filepath.Join(home, ".bashrc")
	default:
		return "", ""
	}
}

func ensureRCLine(path string, line string, force bool) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(headerComment()+line+"\n"), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	if strings.Contains(string(contents), line) && !force {
		return false, nil
	}
	lines := strings.Split(string(contents), "\n")
	var filtered []string
	for _, existing := range lines {
		if strings.Contains(existing, line) {
			continue
		}
		filtered = append(filtered, existing)
	}
	filtered = append(filtered, line)
	final := strings.Join(filtered, "\n")
	if !strings.HasSuffix(final, "\n") {
		final += "\n"
	}
	return true, os.WriteFile(path, []byte(final), 0o644)
}

func removeRCLine(path string, line string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(string(contents), "\n")
	var filtered []string
	removed := false
	for _, existing := range lines {
		if strings.Contains(existing, line) {
			removed = true
			continue
		}
		filtered = append(filtered, existing)
	}
	if !removed {
		return false, nil
	}
	final := strings.Join(filtered, "\n")
	if !strings.HasSuffix(final, "\n") {
		final += "\n"
	}
	return true, os.WriteFile(path, []byte(final), 0o644)
}

func sourceLine(scriptPath string) string {
	return fmt.Sprintf("[ -f %s ] && source %s", friendlyPath(scriptPath), friendlyPath(scriptPath))
}

func friendlyPath(path string) string {
	home := userHome()
	if strings.HasPrefix(path, home) {
		rel := strings.TrimPrefix(path, home)
		rel = strings.TrimPrefix(rel, string(os.PathSeparator))
		return filepath.Join("$HOME", rel)
	}
	return path
}

func headerComment() string {
	return "# Added by SHAI installer\n"
}

func userHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.ShellIntegrator = (*Installer)(nil)

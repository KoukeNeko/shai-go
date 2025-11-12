package domain

// ShellName enumerates supported shells.
type ShellName string

const (
	ShellUnknown ShellName = "unknown"
	ShellZsh     ShellName = "zsh"
	ShellBash    ShellName = "bash"
)

// ShellInstallResult describes install/uninstall outcomes.
type ShellInstallResult struct {
	Shell         ShellName
	ScriptPath    string
	RCFile        string
	ScriptUpdated bool
	RCUpdated     bool
}

// ShellStatus captures current integration state.
type ShellStatus struct {
	Shell        ShellName
	ScriptPath   string
	RCFile       string
	ScriptExists bool
	LinePresent  bool
	Error        string
}

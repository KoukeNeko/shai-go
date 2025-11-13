package commands

// CLI-specific constants
const (
	// DefaultEditorCommand is the default editor command
	DefaultEditorCommand = "vi"
)

// Shell integration constants
const (
	ShellAutoDetect = "auto"
	ShellAll        = "all"
)

// Error messages
const (
	ErrConfigLoaderUnavailable   = "config loader unavailable"
	ErrDoctorServiceUnavailable  = "doctor service unavailable"
	ErrHistoryStoreUnavailable   = "history store unavailable"
	ErrCacheStoreUnavailable     = "cache store unavailable"
	ErrShellInstallerUnavailable = "shell installer unavailable"
	ErrKeyRequired               = "--key is required"
	ErrQueryRequired             = "--query required"
	ErrActionRequired            = "--action is required"
	ErrInvalidPreviewMaxFiles    = "max-files must be >= 1"
	ErrInvalidRetainDays         = "--days must be > 0"
	ErrModelNameEndpointRequired = "--name and --endpoint are required"
	ErrWhitelistEntryEmpty       = "whitelist entry cannot be empty"
)

// Success messages
const (
	MsgConfigurationValid       = "Configuration valid"
	MsgNoDifferencesFromDefault = "No differences from default configuration."
	MsgNoHistoryRecorded        = "No history recorded yet."
	MsgNoCachedResponses        = "No cached responses."
	MsgWhitelistEmpty           = "Whitelist is empty."
	MsgInitCancelled            = "Init cancelled."
)

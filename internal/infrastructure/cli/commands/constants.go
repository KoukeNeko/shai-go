package commands

import "time"

// Command execution constants
const (
	DefaultHistoryLimit       = 20
	DefaultHistorySearchLimit = 50
	DefaultHistoryRetainDays  = 30
	MaxHistoryAnalysisRecords = 1000
)

// Model configuration constants
const (
	DefaultMaxTokens        = 1024
	DefaultModelTestTimeout = 30 * time.Second
	DefaultProviderChoice   = 1
	MinProviderChoice       = 1
	DefaultEditorCommand    = "vi"
)

// Cache configuration constants
const (
	DefaultPreviewMaxFiles = 10
	MinPreviewMaxFiles     = 1
)

// Shell integration constants
const (
	ShellAutoDetect = "auto"
	ShellAll        = "all"
)

// Provider configuration
const (
	ProviderKeyAnthropic = "anthropic"
	ProviderKeyOpenAI    = "openai"
	ProviderKeyOllama    = "ollama"
)

// Time formats
const (
	TimestampFormat = time.RFC3339
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

package domain

import "time"

// File permissions constants
const (
	// DirectoryPermissions is the default permission for directories (rwxr-xr-x)
	DirectoryPermissions = 0o755
	// SecureFilePermissions is the permission for sensitive files (rw-------)
	SecureFilePermissions = 0o600
)

// Timeout and duration constants
const (
	// DefaultToolCacheDuration is how long tool availability is cached
	DefaultToolCacheDuration = 10 * time.Minute
	// DefaultCommandTimeout is the default timeout for command execution
	DefaultCommandTimeout = 2 * time.Second
	// DefaultHTTPClientTimeout is the timeout for HTTP client requests
	DefaultHTTPClientTimeout = 60 * time.Second
)

// Limit constants
const (
	// DefaultPreviewMaxFiles is the default number of files to preview
	DefaultPreviewMaxFiles = 10
	// MinPreviewMaxFiles is the minimum number of files to preview
	MinPreviewMaxFiles = 1
)

// Model configuration constants
const (
	// DefaultMaxTokens is the default maximum number of tokens
	DefaultMaxTokens = 1024
	// DefaultModelTestTimeout is the default timeout for model testing
	DefaultModelTestTimeout = 30 * time.Second
)

// Time formats
const (
	// TimestampFormat is the standard timestamp format
	TimestampFormat = time.RFC3339
)

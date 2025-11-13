package filesystem

import "os"

// UserHomeDir returns the current user's home directory.
// If the home directory cannot be determined, it returns "." as a fallback.
func UserHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

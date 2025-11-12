package ai

import "os"

func resolveAuth(primary string, fallback string) string {
	if primary != "" {
		if value := os.Getenv(primary); value != "" {
			return value
		}
	}
	if fallback == "" {
		return ""
	}
	return os.Getenv(fallback)
}

func resolveOrg(primary string, fallback string) string {
	if primary != "" {
		if value := os.Getenv(primary); value != "" {
			return value
		}
	}
	if fallback == "" {
		return ""
	}
	return os.Getenv(fallback)
}

func valueOrDefault(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func valueOrDefaultInt(value int, def int) int {
	if value == 0 {
		return def
	}
	return value
}

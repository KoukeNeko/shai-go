package domain

import "time"

// HistoryRecord captures executed or generated command metadata.
type HistoryRecord struct {
	Timestamp       time.Time `json:"timestamp"`
	Prompt          string    `json:"prompt"`
	Command         string    `json:"command"`
	Model           string    `json:"model"`
	Executed        bool      `json:"executed"`
	Success         bool      `json:"success"`
	ExitCode        int       `json:"exit_code"`
	RiskLevel       RiskLevel `json:"risk_level"`
	ExecutionTimeMS int64     `json:"execution_time_ms"`
}

// CacheEntry stores cached provider responses.
type CacheEntry struct {
	Key       string    `json:"key"`
	Command   string    `json:"command"`
	Reply     string    `json:"reply"`
	Reasoning string    `json:"reasoning"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

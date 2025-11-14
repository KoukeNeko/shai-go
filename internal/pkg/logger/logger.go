package logger

import (
	"log"
)

// StdLogger is a lightweight implementation backed by Go's log package.
type StdLogger struct {
	verbose bool
}

// NewStd creates a StdLogger.
func NewStd(verbose bool) *StdLogger {
	return &StdLogger{verbose: verbose}
}

func (l *StdLogger) Debug(msg string, fields map[string]interface{}) {
	if !l.verbose {
		return
	}
	log.Println("[DEBUG]", msg, fields)
}

func (l *StdLogger) Info(msg string, fields map[string]interface{}) {
	if !l.verbose {
		return
	}
	log.Println("[INFO]", msg, fields)
}

func (l *StdLogger) Warn(msg string, fields map[string]interface{}) {
	if !l.verbose {
		return
	}
	log.Println("[WARN]", msg, fields)
}

func (l *StdLogger) Error(msg string, err error, fields map[string]interface{}) {
	if !l.verbose {
		return
	}
	log.Println("[ERROR]", msg, err, fields)
}

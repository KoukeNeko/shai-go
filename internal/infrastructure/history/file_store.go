package history

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// FileStore appends history records to a jsonl file.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a new history store under ~/.shai/history/history.jsonl.
func NewFileStore() *FileStore {
	return &FileStore{
		path: filepath.Join(userHome(), ".shai", "history", "history.jsonl"),
	}
}

// Save implements ports.HistoryStore.
func (f *FileStore) Save(record domain.HistoryRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = file.Write(data)
	return err
}

// Path returns the backing file path.
func (f *FileStore) Path() string {
	return f.path
}

// Clear removes the history file.
func (f *FileStore) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Records loads all history entries (best-effort).
func (f *FileStore) Records() ([]domain.HistoryRecord, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	var records []domain.HistoryRecord
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec domain.HistoryRecord
		if err := json.Unmarshal(line, &rec); err == nil {
			records = append(records, rec)
		}
	}
	return records, nil
}

func userHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.HistoryStore = (*FileStore)(nil)

package history

import (
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

func userHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.HistoryStore = (*FileStore)(nil)

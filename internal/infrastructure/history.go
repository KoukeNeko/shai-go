package infrastructure

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

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
func (f *FileStore) Records(limit int, search string) ([]domain.HistoryRecord, error) {
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
			if search != "" && !strings.Contains(rec.Command, search) && !strings.Contains(rec.Prompt, search) {
				continue
			}
			records = append(records, rec)
		}
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records, nil
}

// ExportJSON writes history entries to the given destination as jsonl.
func (f *FileStore) ExportJSON(dest string) error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

// PruneOlderThan removes entries older than N days.
func (f *FileStore) PruneOlderThan(days int) error {
	if days <= 0 {
		return nil
	}
	records, err := f.Records(0, "")
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var buf bytes.Buffer
	for _, rec := range records {
		if rec.Timestamp.Before(cutoff) {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return os.WriteFile(f.path, buf.Bytes(), 0o644)
}

func (f *FileStore) SetRetentionDays(int) {}

var _ ports.HistoryRepository = (*FileStore)(nil)

// SQLiteStore persists history in a SQLite database.
type SQLiteStore struct {
	db            *sql.DB
	path          string
	mu            sync.Mutex
	retentionDays int
}

// NewSQLiteStore creates (or opens) the ~/.shai/history/history.db database.
func NewSQLiteStore(retentionDays int) *SQLiteStore {
	path := filepath.Join(userHome(), ".shai", "history", "history.db")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return &SQLiteStore{path: path, retentionDays: retentionDays}
	}
	store := &SQLiteStore{db: db, path: path, retentionDays: retentionDays}
	if err := store.init(); err != nil {
		return &SQLiteStore{path: path, retentionDays: retentionDays}
	}
	return store
}

func (s *SQLiteStore) init() error {
	if s.db == nil {
		return os.ErrInvalid
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS commands (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT,
		prompt TEXT,
		command TEXT,
		model TEXT,
		executed INTEGER,
		success INTEGER,
		exit_code INTEGER,
		risk_level TEXT,
		execution_time_ms INTEGER
	);`)
	return err
}

// Save inserts a new record.
func (s *SQLiteStore) Save(record domain.HistoryRecord) error {
	if s.db == nil {
		return (&FileStore{path: s.path}).Save(record)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO commands
		(timestamp, prompt, command, model, executed, success, exit_code, risk_level, execution_time_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Timestamp.Format(time.RFC3339),
		record.Prompt,
		record.Command,
		record.Model,
		boolToInt(record.Executed),
		boolToInt(record.Success),
		record.ExitCode,
		record.RiskLevel,
		record.ExecutionTimeMS,
	)
	if err != nil {
		return err
	}
	if s.retentionDays > 0 {
		return s.PruneOlderThan(s.retentionDays)
	}
	return nil
}

// Records returns history entries (limit/search optional).
func (s *SQLiteStore) Records(limit int, search string) ([]domain.HistoryRecord, error) {
	if s.db == nil {
		return (&FileStore{path: s.path}).Records(limit, search)
	}
	builder := strings.Builder{}
	builder.WriteString("SELECT timestamp, prompt, command, model, executed, success, exit_code, risk_level, execution_time_ms FROM commands")
	var args []interface{}
	if search != "" {
		builder.WriteString(" WHERE prompt LIKE ? OR command LIKE ?")
		args = append(args, "%"+search+"%", "%"+search+"%")
	}
	builder.WriteString(" ORDER BY datetime(timestamp) DESC")
	if limit > 0 {
		builder.WriteString(" LIMIT ?")
		args = append(args, limit)
	}
	rows, err := s.db.Query(builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []domain.HistoryRecord
	for rows.Next() {
		var rec domain.HistoryRecord
		var ts string
		var executed, success int
		if err := rows.Scan(&ts, &rec.Prompt, &rec.Command, &rec.Model, &executed, &success, &rec.ExitCode, &rec.RiskLevel, &rec.ExecutionTimeMS); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			rec.Timestamp = t
		}
		rec.Executed = executed == 1
		rec.Success = success == 1
		records = append(records, rec)
	}
	return records, nil
}

// Clear deletes all history entries.
func (s *SQLiteStore) Clear() error {
	if s.db == nil {
		return (&FileStore{path: s.path}).Clear()
	}
	_, err := s.db.Exec("DELETE FROM commands")
	return err
}

// ExportJSON writes the command table to a jsonl file.
func (s *SQLiteStore) ExportJSON(dest string) error {
	records, err := s.Records(0, "")
	if err != nil {
		return err
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, rec := range records {
		if rec.Timestamp.IsZero() {
			rec.Timestamp = time.Now()
		}
		b, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// Path returns the sqlite database path.
func (s *SQLiteStore) Path() string {
	return s.path
}

// PruneOlderThan removes entries older than N days.
func (s *SQLiteStore) PruneOlderThan(days int) error {
	if days <= 0 {
		return nil
	}
	if s.db == nil {
		return (&FileStore{path: s.path}).PruneOlderThan(days)
	}
	_, err := s.db.Exec("DELETE FROM commands WHERE datetime(timestamp) < datetime('now', ?)", fmt.Sprintf("-%d days", days))
	return err
}

// SetRetentionDays updates retention policy.
func (s *SQLiteStore) SetRetentionDays(days int) {
	s.retentionDays = days
}

var _ ports.HistoryRepository = (*SQLiteStore)(nil)

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func userHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

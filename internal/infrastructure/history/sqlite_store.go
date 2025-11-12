package history

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// SQLiteStore persists history in a SQLite database.
type SQLiteStore struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// NewSQLiteStore creates (or opens) the ~/.shai/history/history.db database.
func NewSQLiteStore() *SQLiteStore {
	path := filepath.Join(userHome(), ".shai", "history", "history.db")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		// fallback to file store
		return &SQLiteStore{path: path}
	}
	store := &SQLiteStore{db: db, path: path}
	if err := store.init(); err != nil {
		return &SQLiteStore{path: path}
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
	return err
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ ports.HistoryRepository = (*SQLiteStore)(nil)

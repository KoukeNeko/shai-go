package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// FileCache stores provider responses as JSON blobs addressed by hash key.
type FileCache struct {
	dir string
	mu  sync.Mutex
}

// NewFileCache returns a cache rooted under ~/.shai/cache/responses.
func NewFileCache() *FileCache {
	return &FileCache{
		dir: filepath.Join(userHome(), ".shai", "cache", "responses"),
	}
}

// Get retrieves a cache entry.
func (c *FileCache) Get(key string) (domain.CacheEntry, bool, error) {
	if key == "" {
		return domain.CacheEntry{}, false, nil
	}
	path := c.pathFor(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.CacheEntry{}, false, nil
		}
		return domain.CacheEntry{}, false, err
	}
	var entry domain.CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return domain.CacheEntry{}, false, err
	}
	return entry, true, nil
}

// Set stores a cache entry.
func (c *FileCache) Set(entry domain.CacheEntry) error {
	if entry.Key == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(c.pathFor(entry.Key), data, 0o644)
}

func (c *FileCache) pathFor(key string) string {
	return filepath.Join(c.dir, key+".json")
}

func userHome() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

var _ ports.CacheStore = (*FileCache)(nil)

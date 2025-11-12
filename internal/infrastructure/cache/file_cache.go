package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// FileCache stores provider responses as JSON blobs addressed by hash key.
type FileCache struct {
	dir        string
	mu         sync.Mutex
	maxEntries int
	ttl        time.Duration
}

// NewFileCache returns a cache rooted under ~/.shai/cache/responses.
func NewFileCache() *FileCache {
	return &FileCache{
		dir:        filepath.Join(userHome(), ".shai", "cache", "responses"),
		maxEntries: 100,
		ttl:        time.Hour,
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
	if c.ttl > 0 && time.Since(entry.CreatedAt) > c.ttl {
		_ = os.Remove(path)
		return domain.CacheEntry{}, false, nil
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
	if err := os.WriteFile(c.pathFor(entry.Key), data, 0o644); err != nil {
		return err
	}
	return c.evictIfNeeded()
}

// Dir exposes the cache directory path.
func (c *FileCache) Dir() string {
	return c.dir
}

// Clear removes all cached entries.
func (c *FileCache) Clear() error {
	return os.RemoveAll(c.dir)
}

// Entries lists cache entries (best-effort).
func (c *FileCache) Entries() ([]domain.CacheEntry, error) {
	files, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []domain.CacheEntry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.dir, f.Name()))
		if err != nil {
			continue
		}
		var entry domain.CacheEntry
		if err := json.Unmarshal(data, &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries, nil
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

func (c *FileCache) evictIfNeeded() error {
	if c.maxEntries <= 0 {
		return nil
	}
	files, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(files) <= c.maxEntries {
		return nil
	}
	type fileInfo struct {
		name string
		mod  time.Time
	}
	var infos []fileInfo
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		infos = append(infos, fileInfo{name: f.Name(), mod: info.ModTime()})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].mod.Before(infos[j].mod) })
	for len(infos) > c.maxEntries {
		old := infos[0]
		_ = os.Remove(filepath.Join(c.dir, old.name))
		infos = infos[1:]
	}
	return nil
}

var _ ports.CacheStore = (*FileCache)(nil)
var _ ports.CacheRepository = (*FileCache)(nil)

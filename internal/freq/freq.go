// Package freq persists how often (and how recently) each repo was opened.
package freq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry records open stats for one repo path.
type Entry struct {
	Count      int       `json:"count"`
	LastOpened time.Time `json:"last_opened"`
}

// Store maps absolute repo paths to their open stats.
type Store struct {
	Entries map[string]Entry `json:"entries"`
	path    string
}

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openrepo", "frequency.json"), nil
}

// Load reads the store from disk. A missing or corrupt file yields an empty
// store; loading never fails in a way the caller must handle.
func Load() *Store {
	s := &Store{Entries: map[string]Entry{}}
	path, err := storePath()
	if err != nil {
		return s
	}
	s.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(data, s) != nil || s.Entries == nil {
		s.Entries = map[string]Entry{}
	}
	return s
}

// Get returns the stats for a repo path (zero value if never opened).
func (s *Store) Get(path string) Entry { return s.Entries[path] }

// Bump records an open of the repo at path and saves the store.
func (s *Store) Bump(path string) error {
	e := s.Entries[path]
	e.Count++
	e.LastOpened = time.Now()
	s.Entries[path] = e

	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

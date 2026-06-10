// Package freq persists how often (and how recently) each repo was opened,
// exposing a frecency score: each open is worth 1, decaying exponentially
// with a configurable half-life so stale habits stop dominating.
package freq

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const defaultHalfLifeDays = 30.0

// Entry records open stats for one repo path. Count is the raw lifetime
// total (shown in the picker); Score is the decayed frecency value as of
// LastOpened, used for ranking.
type Entry struct {
	Count      int       `json:"count"`
	Score      float64   `json:"score"`
	LastOpened time.Time `json:"last_opened"`
}

// Store maps absolute repo paths to their open stats.
type Store struct {
	Entries  map[string]Entry `json:"entries"`
	path     string
	halfLife float64 // days
}

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openrepo", "frequency.json"), nil
}

func halfLifeDays() float64 {
	if v := os.Getenv("OPENREPO_HALF_LIFE_DAYS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return defaultHalfLifeDays
}

// Load reads the store from disk. A missing or corrupt file yields an empty
// store; loading never fails in a way the caller must handle.
func Load() *Store {
	s := &Store{Entries: map[string]Entry{}, halfLife: halfLifeDays()}
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

// Score returns the frecency score for a repo path: the decayed sum of past
// opens, plus a small lifetime floor (0.1·ln(1+count)) that never decays, so
// a long-time favourite stays ahead of dormant one-offs and never-opened
// repos no matter how long it sits idle. The floor stays below a single
// fresh open (1), so it cannot outrank current work.
func (s *Store) Score(path string) float64 {
	e := s.Entries[path]
	if e.Count == 0 && e.Score == 0 {
		return 0
	}
	return s.decayed(e) + 0.1*math.Log(1+float64(e.Count))
}

// decayed returns the sum of past opens decayed to now, without the floor.
func (s *Store) decayed(e Entry) float64 {
	score := e.Score
	if score == 0 {
		// Entry written before scores existed: seed from the raw count.
		score = float64(e.Count)
	}
	if days := time.Since(e.LastOpened).Hours() / 24; days > 0 {
		score *= math.Exp2(-days / s.halfLife)
	}
	return score
}

// Bump records an open of the repo at path and saves the store. Entries are
// never pruned: dormant history is still ranking signal, and a deleted repo
// may be re-cloned to the same path and pick its history back up.
func (s *Store) Bump(path string) error {
	e := s.Entries[path]
	e.Score = s.decayed(e) + 1
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

package main

import (
	"sort"
	"sync"
	"time"
)

// ScoreEntry is a single scoreboard record.
type ScoreEntry struct {
	Nick      string    `json:"nick"`
	Score     int       `json:"score"`
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// MarshalJSON emits Timestamp as a simple date-time string.
func (s ScoreEntry) timestampStr() string {
	return s.Timestamp.UTC().Format(time.RFC3339)
}

// ScoreStore is an in-memory, thread-safe scoreboard.
type ScoreStore struct {
	mu        sync.RWMutex
	entries   []ScoreEntry
	rateLimit map[string]time.Time // ip → last submission
}

func NewScoreStore() *ScoreStore {
	return &ScoreStore{
		rateLimit: make(map[string]time.Time),
	}
}

// Add validates and stores a new entry.
// Returns an error string (empty = ok), and an HTTP status code.
func (s *ScoreStore) Add(entry ScoreEntry, ip string) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Rate limit: 1 per IP per minute
	if last, ok := s.rateLimit[ip]; ok {
		if time.Since(last) < time.Minute {
			return "too many requests", 429
		}
	}
	s.rateLimit[ip] = time.Now()

	entry.Timestamp = time.Now().UTC()
	s.entries = append(s.entries, entry)

	// Keep only top 100 per level to bound memory
	s.prune()

	return "", 201
}

// Top returns the top n entries for the given level (empty = all levels).
func (s *ScoreStore) Top(level string, n int) []scoreEntryJSON {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []ScoreEntry
	for _, e := range s.entries {
		if level == "" || e.Level == level {
			filtered = append(filtered, e)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	if n > len(filtered) {
		n = len(filtered)
	}

	out := make([]scoreEntryJSON, n)
	for i, e := range filtered[:n] {
		out[i] = scoreEntryJSON{
			Nick:      e.Nick,
			Score:     e.Score,
			Level:     e.Level,
			Timestamp: e.Timestamp.UTC().Format(time.RFC3339),
		}
	}
	return out
}

// scoreEntryJSON is the wire format (Timestamp as string).
type scoreEntryJSON struct {
	Nick      string `json:"nick"`
	Score     int    `json:"score"`
	Level     string `json:"level"`
	Timestamp string `json:"timestamp"`
}

func (s *ScoreStore) prune() {
	levels := []string{"easy", "medium", "hard"}
	maxPerLevel := 100

	var keep []ScoreEntry
	for _, lvl := range levels {
		var lvlEntries []ScoreEntry
		for _, e := range s.entries {
			if e.Level == lvl {
				lvlEntries = append(lvlEntries, e)
			}
		}
		sort.Slice(lvlEntries, func(i, j int) bool {
			return lvlEntries[i].Score > lvlEntries[j].Score
		})
		if len(lvlEntries) > maxPerLevel {
			lvlEntries = lvlEntries[:maxPerLevel]
		}
		keep = append(keep, lvlEntries...)
	}
	s.entries = keep
}

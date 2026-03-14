package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ScoreEntry is the canonical score record used for storage and JSON responses.
type ScoreEntry struct {
	Nick      string `json:"nick"`
	Score     int    `json:"score"`
	Level     string `json:"level"`
	Timestamp string `json:"timestamp"`
}

// scoreEntryJSON aliases ScoreEntry for use in handler code.
type scoreEntryJSON = ScoreEntry

// ScoreStore persists scores in a SQLite database.
type ScoreStore struct {
	db     *sql.DB
	mu     sync.Mutex
	lastIP map[string]time.Time // rate limit: IP → last submission time
}

// NewScoreStore opens (or creates) the SQLite database at dbPath.
// If dbPath is empty it defaults to $XDG_CONFIG_HOME/reverse-pong/scores.db.
func NewScoreStore(dbPath string) *ScoreStore {
	if dbPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = "."
		}
		dbPath = filepath.Join(dir, "reverse-pong", "scores.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("scores db dir: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open scores db %s: %v", dbPath, err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS scores (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		nick      TEXT    NOT NULL,
		score     INTEGER NOT NULL,
		level     TEXT    NOT NULL,
		timestamp TEXT    NOT NULL
	)`)
	if err != nil {
		log.Fatalf("create scores table: %v", err)
	}

	log.Printf("scores db: %s", dbPath)
	return &ScoreStore{db: db, lastIP: make(map[string]time.Time)}
}

// Top returns the top-N scores, optionally filtered by level.
func (s *ScoreStore) Top(level string, n int) []ScoreEntry {
	var (
		rows *sql.Rows
		err  error
	)
	if level == "" {
		rows, err = s.db.Query(
			`SELECT nick, score, level, timestamp FROM scores ORDER BY score DESC LIMIT ?`, n)
	} else {
		rows, err = s.db.Query(
			`SELECT nick, score, level, timestamp FROM scores WHERE level=? ORDER BY score DESC LIMIT ?`, level, n)
	}
	if err != nil {
		log.Printf("scores query: %v", err)
		return nil
	}
	defer rows.Close()

	var entries []ScoreEntry
	for rows.Next() {
		var e ScoreEntry
		if err := rows.Scan(&e.Nick, &e.Score, &e.Level, &e.Timestamp); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

// Add inserts a score entry. Returns a non-empty message and HTTP status on error.
func (s *ScoreStore) Add(entry ScoreEntry, ip string) (string, int) {
	s.mu.Lock()
	last, ok := s.lastIP[ip]
	if ok && time.Since(last) < time.Minute {
		s.mu.Unlock()
		return "too many requests", http.StatusTooManyRequests
	}
	s.lastIP[ip] = time.Now()
	s.mu.Unlock()

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO scores (nick, score, level, timestamp) VALUES (?, ?, ?, ?)`,
		entry.Nick, entry.Score, entry.Level, entry.Timestamp,
	)
	if err != nil {
		log.Printf("scores insert: %v", err)
		return "db error", http.StatusInternalServerError
	}
	return "", 0
}

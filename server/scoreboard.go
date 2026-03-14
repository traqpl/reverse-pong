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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS matches (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		mode      TEXT NOT NULL,
		level     TEXT NOT NULL,
		winner    TEXT NOT NULL,
		timestamp TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatalf("create matches table: %v", err)
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

// TwoPlayerStats holds win counts for a single level in 2P mode.
type TwoPlayerStats struct {
	Total int `json:"total"`
	P1    int `json:"p1"`
	P2    int `json:"p2"`
	Draw  int `json:"draw"`
}

// StatsResponse is the payload returned by GET /api/stats.
type StatsResponse struct {
	Matches1P map[string]int            `json:"matches_1p"`
	Matches2P map[string]TwoPlayerStats `json:"matches_2p"`
}

// Stats returns aggregated match counts.
func (s *ScoreStore) Stats() StatsResponse {
	resp := StatsResponse{
		Matches1P: make(map[string]int),
		Matches2P: make(map[string]TwoPlayerStats),
	}

	rows, err := s.db.Query(`SELECT level, COUNT(*) FROM matches WHERE mode='1p' GROUP BY level`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var level string
			var count int
			if rows.Scan(&level, &count) == nil {
				resp.Matches1P[level] = count
			}
		}
	}

	rows2, err := s.db.Query(`SELECT level, winner, COUNT(*) FROM matches WHERE mode='2p' GROUP BY level, winner`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var level, winner string
			var count int
			if rows2.Scan(&level, &winner, &count) == nil {
				s2 := resp.Matches2P[level]
				s2.Total += count
				switch winner {
				case "p1":
					s2.P1 = count
				case "p2":
					s2.P2 = count
				case "draw":
					s2.Draw = count
				}
				resp.Matches2P[level] = s2
			}
		}
	}

	return resp
}

func (s *ScoreStore) recordMatch(mode, level, winner string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(
		`INSERT INTO matches (mode, level, winner, timestamp) VALUES (?, ?, ?, ?)`,
		mode, level, winner, ts,
	); err != nil {
		log.Printf("matches insert: %v", err)
	}
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
	s.recordMatch("1p", entry.Level, "")
	return "", 0
}

// Add2P inserts both players' scores from a 2P match in one rate-limited operation.
// level is the difficulty ("easy", "medium", "hard"); winner is "p1", "p2", or "draw".
func (s *ScoreStore) Add2P(p1, p2 ScoreEntry, level, winner, ip string) (string, int) {
	s.mu.Lock()
	last, ok := s.lastIP[ip]
	if ok && time.Since(last) < time.Minute {
		s.mu.Unlock()
		return "too many requests", http.StatusTooManyRequests
	}
	s.lastIP[ip] = time.Now()
	s.mu.Unlock()

	ts := time.Now().UTC().Format(time.RFC3339)
	p1.Timestamp = ts
	p2.Timestamp = ts

	tx, err := s.db.Begin()
	if err != nil {
		return "db error", http.StatusInternalServerError
	}
	ins := `INSERT INTO scores (nick, score, level, timestamp) VALUES (?, ?, ?, ?)`
	if _, err = tx.Exec(ins, p1.Nick, p1.Score, p1.Level, p1.Timestamp); err != nil {
		_ = tx.Rollback()
		log.Printf("scores 2p insert p1: %v", err)
		return "db error", http.StatusInternalServerError
	}
	if p2.Nick != "---" {
		if _, err = tx.Exec(ins, p2.Nick, p2.Score, p2.Level, p2.Timestamp); err != nil {
			_ = tx.Rollback()
			log.Printf("scores 2p insert p2: %v", err)
			return "db error", http.StatusInternalServerError
		}
	}
	if err = tx.Commit(); err != nil {
		return "db error", http.StatusInternalServerError
	}
	s.recordMatch("2p", level, winner)
	return "", 0
}

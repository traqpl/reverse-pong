package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//go:embed web
var webFS embed.FS

var store *ScoreStore

func main() {
	loadConfig()
	store = NewScoreStore(os.Getenv("DB_PATH"))

	// Ensure .wasm gets correct MIME type (some systems lack it)
	_ = mime.AddExtensionType(".wasm", "application/wasm")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8073"
	}

	mux := http.NewServeMux()

	// Config API
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gameConfig)
	})

	// Stats API
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.Stats())
	})

	// Scoreboard API
	mux.HandleFunc("/api/scores", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			handleGetScores(w, r)
		case http.MethodPost:
			handlePostScore(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// Static files (web/ directory embedded)
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", fileServer)

	addr := ":" + port
	log.Printf("REVERSE PONG server listening on http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleGetScores(w http.ResponseWriter, r *http.Request) {
	level := strings.ToLower(r.URL.Query().Get("level"))
	if level != "" && level != "easy" && level != "medium" && level != "hard" && level != "2p" {
		http.Error(w, `{"error":"invalid level"}`, http.StatusBadRequest)
		return
	}
	nStr := r.URL.Query().Get("n")
	n := 10
	if nStr != "" {
		if v, err := strconv.Atoi(nStr); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}
	entries := store.Top(level, n)
	if entries == nil {
		entries = []scoreEntryJSON{}
	}
	_ = json.NewEncoder(w).Encode(entries)
}

var nickRe = regexp.MustCompile(`^[A-Za-z]{3}$`)

func handlePostScore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// 1P fields
		Nick  string `json:"nick"`
		Score int    `json:"score"`
		Level string `json:"level"`
		// 2P batch fields
		P1Nick  string `json:"p1_nick"`
		P1Score int    `json:"p1_score"`
		P2Nick  string `json:"p2_nick"`
		P2Score int    `json:"p2_score"`
		// 2P difficulty level
		P2Level string `json:"p2_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[0])
	}

	// 2P batch submission
	if req.P1Nick != "" {
		p1Nick := strings.ToUpper(req.P1Nick)
		p2Nick := strings.ToUpper(req.P2Nick)
		if !nickRe.MatchString(p1Nick) || (!nickRe.MatchString(p2Nick) && p2Nick != "---") {
			http.Error(w, `{"error":"nick must be 3 letters A-Z"}`, http.StatusBadRequest)
			return
		}
		if req.P1Score < 0 || req.P1Score > 9999 || req.P2Score < 0 || req.P2Score > 9999 {
			http.Error(w, `{"error":"score out of range"}`, http.StatusBadRequest)
			return
		}
		matchLevel := strings.ToLower(req.P2Level)
		if matchLevel != "easy" && matchLevel != "medium" && matchLevel != "hard" {
			matchLevel = "easy" // fallback for old clients
		}
		winner := "draw"
		if req.P1Score > req.P2Score {
			winner = "p1"
		} else if req.P2Score > req.P1Score {
			winner = "p2"
		}
		msg, status := store.Add2P(
			ScoreEntry{Nick: p1Nick, Score: req.P1Score, Level: "2p"},
			ScoreEntry{Nick: p2Nick, Score: req.P2Score, Level: "2p"},
			matchLevel, winner, ip,
		)
		if msg != "" {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}

	// 1P submission
	if !nickRe.MatchString(req.Nick) {
		http.Error(w, `{"error":"nick must be 3 letters A-Z"}`, http.StatusBadRequest)
		return
	}
	if req.Score < 0 || req.Score > 9999 {
		http.Error(w, `{"error":"score out of range"}`, http.StatusBadRequest)
		return
	}
	req.Level = strings.ToLower(req.Level)
	if req.Level != "easy" && req.Level != "medium" && req.Level != "hard" {
		http.Error(w, `{"error":"invalid level"}`, http.StatusBadRequest)
		return
	}

	nick := strings.ToUpper(req.Nick)
	for _, r := range nick {
		if !unicode.IsLetter(r) {
			http.Error(w, `{"error":"nick must be letters only"}`, http.StatusBadRequest)
			return
		}
	}

	entry := ScoreEntry{Nick: nick, Score: req.Score, Level: req.Level}
	msg, status := store.Add(entry, ip)
	if msg != "" {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

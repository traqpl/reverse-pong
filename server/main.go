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

var store = NewScoreStore()

func main() {
	loadConfig()

	// Ensure .wasm gets correct MIME type (some systems lack it)
	_ = mime.AddExtensionType(".wasm", "application/wasm")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Config API
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gameConfig)
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
	if level != "" && level != "easy" && level != "medium" && level != "hard" {
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
		Nick  string `json:"nick"`
		Score int    `json:"score"`
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	// Validate
	if !nickRe.MatchString(req.Nick) {
		http.Error(w, `{"error":"nick must be 3 letters A-Z"}`, http.StatusBadRequest)
		return
	}
	if req.Score < 0 || req.Score > 999 {
		http.Error(w, `{"error":"score out of range"}`, http.StatusBadRequest)
		return
	}
	req.Level = strings.ToLower(req.Level)
	if req.Level != "easy" && req.Level != "medium" && req.Level != "hard" {
		http.Error(w, `{"error":"invalid level"}`, http.StatusBadRequest)
		return
	}

	// Uppercase nick
	nick := strings.ToUpper(req.Nick)
	for _, r := range nick {
		if !unicode.IsLetter(r) {
			http.Error(w, `{"error":"nick must be letters only"}`, http.StatusBadRequest)
			return
		}
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	// Trust X-Forwarded-For from local reverse proxy
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[0])
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

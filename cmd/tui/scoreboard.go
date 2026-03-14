package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ── persistence ───────────────────────────────────────────────────────────────

type hofEntry struct {
	Nick  string `json:"nick"`
	Score int    `json:"score"`
	Level string `json:"level"`
	Date  string `json:"date"` // YYYY-MM-DD
}

type scoreboardState struct {
	all    []hofEntry // full list, sorted by score desc
	tab    int        // 0=all 1=easy 2=medium 3=hard
	scroll int

	pendingNick  string
	pendingScore int
}

var tabLabels = [4]string{"ALL", "EASY", "MEDIUM", "HARD"}
var tabLevels = [4]string{"", "easy", "medium", "hard"}

func (s *scoreboardState) tickerText() string {
	if len(s.all) == 0 {
		return "   ★  NO SCORES YET — BE THE FIRST!  ★   "
	}
	var out string
	for i, e := range s.all {
		if i >= 10 {
			break
		}
		out += fmt.Sprintf("   #%d  %s  %d pts  %s  ◆", i+1, e.Nick, e.Score, e.Level)
	}
	return out + "   "
}

func newScoreboardState() *scoreboardState {
	s := &scoreboardState{}
	s.all = loadHOF()
	return s
}

func hofPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "reverse-pong-tui", "hof.json")
}

func loadHOF() []hofEntry {
	data, err := os.ReadFile(hofPath())
	if err != nil {
		return nil
	}
	var entries []hofEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

func saveHOF(entries []hofEntry) {
	path := hofPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(entries, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func (s *scoreboardState) add(nick string, score int, level string) {
	s.all = append(s.all, hofEntry{
		Nick:  nick,
		Score: score,
		Level: level,
		Date:  time.Now().Format("2006-01-02"),
	})
	sort.Slice(s.all, func(i, j int) bool {
		return s.all[i].Score > s.all[j].Score
	})
	if len(s.all) > 100 {
		s.all = s.all[:100]
	}
	saveHOF(s.all)
}

func (s *scoreboardState) visible() []hofEntry {
	level := tabLevels[s.tab]
	if level == "" {
		return s.all
	}
	var out []hofEntry
	for _, e := range s.all {
		if e.Level == level {
			out = append(out, e)
		}
	}
	return out
}

// ── navigation ────────────────────────────────────────────────────────────────

func (s *scoreboardState) prevTab() {
	if s.tab > 0 {
		s.tab--
		s.scroll = 0
	}
}

func (s *scoreboardState) nextTab() {
	if s.tab < 3 {
		s.tab++
		s.scroll = 0
	}
}

const visibleRows = 14

func (s *scoreboardState) scrollUp() {
	if s.scroll > 0 {
		s.scroll--
	}
}

func (s *scoreboardState) scrollDown() {
	if s.scroll+visibleRows < len(s.visible()) {
		s.scroll++
	}
}

// ── draw ──────────────────────────────────────────────────────────────────────

func (e *Engine) drawScoreboard() {
	e.fillBg()
	s := e.sb

	// Title
	e.drawCentered(0, "  HALL OF FAME  ", styleTitle)
	for x := 0; x < e.termW; x++ {
		e.scr.SetContent(x, 1, '─', nil, styleSep)
	}

	// Tabs
	tabW := 10
	tabStartX := (e.termW - 4*tabW) / 2
	for i, label := range tabLabels {
		x := tabStartX + i*tabW
		st := styleDim
		if i == s.tab {
			st = styleSelected
		}
		e.printAt(x, 2, fmt.Sprintf(" %-9s", label), st)
	}
	for x := 0; x < e.termW; x++ {
		e.scr.SetContent(x, 3, '─', nil, styleSep)
	}

	entries := s.visible()

	if len(entries) == 0 {
		e.drawCentered(e.termH/2, "  no scores yet — play a game!  ", styleDim)
		e.drawFooter("← → tab   R / ESC: menu")
		return
	}

	// Header
	headerY := 4
	e.printAt(2, headerY, fmt.Sprintf("%-4s %-5s %-7s %-8s %s", "#", "NICK", "SCORE", "LEVEL", "DATE"), styleDim)
	for x := 0; x < e.termW; x++ {
		e.scr.SetContent(x, headerY+1, '─', nil, styleSep)
	}

	// Rows
	for i := 0; i < visibleRows; i++ {
		idx := s.scroll + i
		if idx >= len(entries) {
			break
		}
		en := entries[idx]
		y := headerY + 2 + i

		line := fmt.Sprintf("%-4s %-5s %-7d %-8s %s",
			fmt.Sprintf("%d.", idx+1), en.Nick, en.Score, en.Level, en.Date)

		st := styleDefault
		if en.Nick == s.pendingNick && en.Score == s.pendingScore {
			st = styleGood
		}
		e.printAt(2, y, line, st)
	}

	// Scroll indicator
	total := len(entries)
	if total > visibleRows {
		indicator := fmt.Sprintf(" %d–%d / %d ", s.scroll+1, min(s.scroll+visibleRows, total), total)
		e.printAt(e.termW-len([]rune(indicator))-1, headerY+2+visibleRows, indicator, styleDim)
		if s.scroll > 0 {
			e.drawCentered(headerY+2-1, "▲", styleDim)
		}
		if s.scroll+visibleRows < total {
			e.drawCentered(headerY+2+visibleRows, "▼", styleDim)
		}
	}

	e.drawFooter("← → tab   ↑ ↓ scroll   R / ESC: menu")
}

func (e *Engine) drawFooter(hint string) {
	e.drawCentered(e.termH-1, fmt.Sprintf("  %s  ", hint), styleDim)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

// ScoreEntry mirrors the server model.
type ScoreEntry struct {
	Nick      string `json:"nick"`
	Score     int    `json:"score"`
	Level     string `json:"level"`
	Timestamp string `json:"timestamp"`
}

// Scoreboard state on Engine
type scoreboardState struct {
	scoreTab           int // 0=all,1=easy,2=medium,3=hard
	scoreEntries       []ScoreEntry
	scoreLoading       bool
	scoreError         string
	lastSubmittedNick  string
	lastSubmittedScore int
	scoreRefreshTimer  float64
}

func (e *Engine) prevScoreTab() {
	if e.scoreTab > 0 {
		e.scoreTab--
		go e.fetchScores(tabLevel(e.scoreTab))
	}
}

func (e *Engine) nextScoreTab() {
	if e.scoreTab < 3 {
		e.scoreTab++
		go e.fetchScores(tabLevel(e.scoreTab))
	}
}

func tabLevel(tab int) string {
	switch tab {
	case 1:
		return "easy"
	case 2:
		return "medium"
	case 3:
		return "hard"
	}
	return ""
}

func (e *Engine) fetchScores(level string) {
	e.scoreLoading = true
	e.scoreError = ""

	url := "/api/scores"
	if level != "" {
		url += "?level=" + level
	}

	ch := make(chan struct{})
	var result []ScoreEntry
	var fetchErr string

	js.Global().Call("fetch", url).Call("then",
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			resp := args[0]
			resp.Call("json").Call("then",
				js.FuncOf(func(_ js.Value, args []js.Value) any {
					jsonStr := js.Global().Get("JSON").Call("stringify", args[0]).String()
					if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
						fetchErr = err.Error()
					}
					close(ch)
					return nil
				}),
				js.FuncOf(func(_ js.Value, args []js.Value) any {
					fetchErr = "parse error"
					close(ch)
					return nil
				}),
			)
			return nil
		}),
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			fetchErr = "network error"
			close(ch)
			return nil
		}),
	)

	<-ch
	e.scoreEntries = result
	e.scoreError = fetchErr
	e.scoreLoading = false
}

func (e *Engine) postScore(nick string, score int, level AILevel) {
	body := fmt.Sprintf(`{"nick":%q,"score":%d,"level":%q}`, nick, score, levelName(level))
	js.Global().Call("fetch", "/api/scores", map[string]any{
		"method": "POST",
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
		"body": body,
	})
}

// handleNickKey processes keyboard input on the GameOver screen.
func (e *Engine) handleNickKey(key string) {
	switch key {
	case "Escape":
		e.state = StateScoreboard
		go e.fetchScores("")
	case "r", "R":
		e.state = StateMenu
	case "Enter":
		if e.nickLen == 3 {
			nick := string(e.pendingNick[:3])
			e.lastSubmittedNick = nick
			e.lastSubmittedScore = e.score
			go e.postScore(nick, e.score, e.level)
			e.state = StateScoreboard
			e.scoreTab = 0
			go e.fetchScores("")
		}
	case "Backspace":
		if e.nickLen > 0 {
			e.nickLen--
		}
	default:
		if len(key) == 1 && e.nickLen < 3 {
			r := rune(key[0])
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			if r >= 'A' && r <= 'Z' {
				e.pendingNick[e.nickLen] = r
				e.nickLen++
			}
		}
	}
}

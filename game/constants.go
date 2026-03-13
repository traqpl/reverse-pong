//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

// GameConfig mirrors server/config.go — fetched from /api/config at startup.
type GameConfig struct {
	Game   GameSettings             `json:"game"`
	Levels map[string]LevelSettings `json:"levels"`
}

type GameSettings struct {
	Duration               float64 `json:"duration"`
	BallSpeedupRate        float64 `json:"ball_speedup_rate"`
	PlayerSpeed            float64 `json:"player_speed"`
	HitFreezeDuration      float64 `json:"hit_freeze_duration"`
	CountdownDigitDuration float64 `json:"countdown_digit_duration"`
	GoDisplayDuration      float64 `json:"go_display_duration"`
}

type LevelSettings struct {
	BallSpeed      float64 `json:"ball_speed"`
	PaddleSpeed    float64 `json:"paddle_speed"`
	Points         int     `json:"points"`
	JitterRange    float64 `json:"jitter_range"`
	JitterInterval float64 `json:"jitter_interval"`
	ErrorChance    float64 `json:"error_chance"`
	ErrorRange     float64 `json:"error_range"`
	ErrorInterval  float64 `json:"error_interval"`
	DeadZone       float64 `json:"dead_zone"`
}

// cfg is the global config populated at startup.
var cfg GameConfig

// fetchConfig loads /api/config synchronously via XHR before the game loop starts.
func fetchConfig() {
	ch := make(chan string, 1)

	xhr := js.Global().Get("XMLHttpRequest").New()
	xhr.Call("open", "GET", "/api/config", false) // false = synchronous
	xhr.Call("send")

	status := xhr.Get("status").Int()
	if status == 200 {
		ch <- xhr.Get("responseText").String()
	} else {
		ch <- ""
	}

	raw := <-ch
	if raw == "" {
		cfg = defaultConfig()
		return
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		cfg = defaultConfig()
	}
}

func defaultConfig() GameConfig {
	return GameConfig{
		Game: GameSettings{
			Duration:               60,
			BallSpeedupRate:        1.05,
			PlayerSpeed:            280,
			HitFreezeDuration:      0.8,
			CountdownDigitDuration: 0.8,
			GoDisplayDuration:      0.6,
		},
		Levels: map[string]LevelSettings{
			"easy": {
				BallSpeed: 240, PaddleSpeed: 180, Points: 1,
				JitterRange: 30, JitterInterval: 0.5, DeadZone: 15,
			},
			"medium": {
				BallSpeed: 300, PaddleSpeed: 300, Points: 2,
				ErrorChance: 0.30, ErrorRange: 70, ErrorInterval: 1.8, DeadZone: 12,
			},
			"hard": {
				BallSpeed: 360, PaddleSpeed: 420, Points: 3,
				ErrorChance: 0.08, ErrorRange: 35, ErrorInterval: 3.0, DeadZone: 3,
			},
		},
	}
}

// Helpers used throughout the game code.

func levelName(l AILevel) string {
	switch l {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	}
	return "easy"
}

func levelCfg(l AILevel) LevelSettings {
	return cfg.Levels[levelName(l)]
}

func baseSpeed(l AILevel) float64   { return levelCfg(l).BallSpeed }
func levelPoints(l AILevel) int     { return levelCfg(l).Points }
func paddleSpeed(l AILevel) float64 { return levelCfg(l).PaddleSpeed }

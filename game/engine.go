//go:build js && wasm

package main

import "syscall/js"

// GameState represents the current screen/phase of the game.
type GameState int

const (
	StateMenu      GameState = iota
	StateCountdown           // "3, 2, 1, GO!"
	StatePlaying
	StatePaused
	StateHit // short freeze after paddle catches the ball
	StateGameOver
	StateScoreboard
)

// Engine holds all game state.
type Engine struct {
	canvas js.Value
	ctx    js.Value
	w, h   float64 // logical canvas size

	state     GameState
	twoPlayer bool // true = player 2 controls paddle with W/S

	ball   Ball
	paddle Paddle
	level  AILevel

	score      int
	streak     int
	bestStreak int
	timeLeft   float64 // seconds remaining

	countdownTimer float64 // time within current countdown digit
	countdownDigit int     // 3, 2, 1, 0 = GO!

	// scoreboard
	scoreboardState
	pendingNick [3]rune
	nickLen     int

	// input
	keys map[string]bool
}

func NewEngine(canvas js.Value) *Engine {
	ctx := canvas.Call("getContext", "2d")
	w := canvas.Get("width").Float()
	h := canvas.Get("height").Float()

	e := &Engine{
		canvas: canvas,
		ctx:    ctx,
		w:      w,
		h:      h,
		state:  StateMenu,
		keys:   make(map[string]bool),
		level:  Medium,
	}
	e.resetBall()
	e.resetPaddle()
	return e
}

func (e *Engine) resetBall() {
	speed := baseSpeed(e.level)
	e.ball = Ball{
		X:  e.w / 2,
		Y:  e.h / 2,
		VX: speed,
		VY: speed * 0.5,
		R:  7,
	}
}

func (e *Engine) resetPaddle() {
	e.paddle = newPaddle(e.level, e.h, levelCfg(e.level))
}

func (e *Engine) startCountdown() {
	e.state = StateCountdown
	e.countdownDigit = 3
	e.countdownTimer = cfg.Game.CountdownDigitDuration
	callAudio("countdown")
}

func (e *Engine) startGame() {
	e.score = 0
	e.streak = 0
	e.bestStreak = 0
	e.timeLeft = cfg.Game.Duration
	e.resetBall()
	e.resetPaddle()
	e.startCountdown()
}

// Update advances game logic by dt seconds.
func (e *Engine) Update(dt float64) {
	switch e.state {
	case StateMenu:
		e.updateMenu(dt)
	case StateCountdown:
		e.updateCountdown(dt)
	case StatePlaying:
		e.updatePlaying(dt)
	case StatePaused:
		// nothing — waiting for input
	case StateGameOver:
		e.updateGameOver(dt)
	case StateScoreboard:
		// handled separately (async fetch)
	}
}

// Render draws the current frame.
func (e *Engine) Render() {
	switch e.state {
	case StateMenu:
		e.renderMenu()
	case StateCountdown:
		e.renderCountdown()
	case StatePlaying:
		e.renderPlaying()
	case StatePaused:
		e.renderPlaying() // draw game underneath
		e.renderPauseOverlay()
	case StateGameOver:
		e.renderGameOver()
	case StateScoreboard:
		e.renderScoreboard()
	}
	e.renderScanlines()
}

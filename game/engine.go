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

	// score flash
	scoreFlashTimer float64

	// 2P scores
	score2      int
	streak2     int
	bestStreak2 int

	// scoreboard / nick entry
	scoreboardState
	pendingNick  [3]rune
	nickLen      int
	pendingNick2 [3]rune
	nickLen2     int
	nickPhase    int // 0=P1 nick, 1=P2 nick (2P mode only)

	// input
	keys map[string]bool

	musicEnabled bool

	// wallCooldownSide blocks player input toward a wall after a bounce:
	// -1 = top wall hit (block upward), +1 = bottom wall hit (block downward), 0 = none.
	wallCooldownSide int
}

func NewEngine(canvas js.Value) *Engine {
	// Request Display P3 color space for wider gamut on capable displays (Chrome/Mac).
	ctx := canvas.Call("getContext", "2d", map[string]any{"colorSpace": "display-p3"})
	if ctx.IsNull() || ctx.IsUndefined() {
		ctx = canvas.Call("getContext", "2d")
	}
	w := canvas.Get("width").Float()
	h := canvas.Get("height").Float()

	e := &Engine{
		canvas:       canvas,
		ctx:          ctx,
		w:            w,
		h:            h,
		state:        StateMenu,
		keys:         make(map[string]bool),
		level:        Medium,
		musicEnabled: true,
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
	e.score2 = 0
	e.streak2 = 0
	e.bestStreak2 = 0
	e.nickPhase = 0
	e.nickLen = 0
	e.nickLen2 = 0
	e.timeLeft = cfg.Game.Duration
	e.resetBall()
	e.resetPaddle()
	e.playMusic("gameMusic")
	e.startCountdown()
}

func (e *Engine) playMusic(name string) {
	if e.musicEnabled {
		callAudio(name)
	}
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

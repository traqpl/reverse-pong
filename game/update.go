//go:build js && wasm

package main

// updateMenu handles animated state in the menu (used for pulsing title).
func (e *Engine) updateMenu(_ float64) {
	// animation driven entirely in render via js.Global().Get("performance").Call("now")
}

// updateCountdown ticks through 3 → 2 → 1 → GO! sequence.
func (e *Engine) updateCountdown(dt float64) {
	e.countdownTimer -= dt
	if e.countdownTimer > 0 {
		return
	}

	e.countdownDigit--
	if e.countdownDigit > 0 {
		e.countdownTimer = cfg.Game.CountdownDigitDuration
		callAudio("countdown")
	} else if e.countdownDigit == 0 {
		// "GO!"
		e.countdownTimer = cfg.Game.GoDisplayDuration
		callAudio("go")
	} else {
		// Start playing
		e.state = StatePlaying
	}
}

// updatePlaying runs the main game loop tick.
func (e *Engine) updatePlaying(dt float64) {
	// Player movement
	e.applyMovementInput(dt)

	// Ball physics
	e.ball.Update(dt, e.w, e.h)

	// AI paddle (skipped in 2P mode — player 2 moves it via W/S in applyMovementInput)
	if !e.twoPlayer {
		e.paddle.Update(e.ball, dt, e.h)
	}

	// Ball reaches left wall — paddle always bounces it back
	if e.ball.X-e.ball.R <= 0 {
		e.ball.X = e.ball.R
		e.ball.VX = -e.ball.VX // flip to positive (going right)
		e.onBounce()
		return
	}

	// Timer
	e.timeLeft -= dt
	if e.timeLeft <= 0 {
		e.timeLeft = 0
		e.state = StateGameOver
		e.nickLen = 0
		callAudio("gameOver")
	}
}

// onBounce scores points and speeds the ball up on every left-wall bounce.
func (e *Engine) onBounce() {
	e.streak++
	if e.streak > e.bestStreak {
		e.bestStreak = e.streak
	}

	pts := levelPoints(e.level)
	multiplier := streakMultiplier(e.streak)
	e.score += int(float64(pts) * multiplier)

	callAudio("bounce")

	// Speed up ball slightly; VX is already positive after the flip above
	speed := e.ball.VX * cfg.Game.BallSpeedupRate
	e.ball.VX = speed
	if e.ball.VY < 0 {
		e.ball.VY = -speed * 0.5
	} else {
		e.ball.VY = speed * 0.5
	}
}

// updateGameOver handles nick input state (key events handle key presses directly).
func (e *Engine) updateGameOver(_ float64) {}

func streakMultiplier(streak int) float64 {
	if streak >= 5 {
		return 2.0
	}
	if streak >= 3 {
		return 1.5
	}
	return 1.0
}

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
	if e.scoreFlashTimer > 0 {
		e.scoreFlashTimer -= dt
	}

	// Player movement
	e.applyMovementInput(dt)

	// Ball physics
	e.ball.Update(dt, e.w, e.h)

	// Wall cooldown: after hitting top/bottom, block player from pushing ball back
	// until it travels at least 1/4 screen height away from that wall.
	if e.ball.LastWallHit != 0 {
		e.wallCooldownSide = e.ball.LastWallHit
	}
	if e.wallCooldownSide == -1 && e.ball.Y > e.h/4 {
		e.wallCooldownSide = 0
	} else if e.wallCooldownSide == 1 && e.ball.Y < 3*e.h/4 {
		e.wallCooldownSide = 0
	}

	// AI paddle (skipped in 2P mode — player 2 moves it via W/S in applyMovementInput)
	if !e.twoPlayer {
		e.paddle.Update(e.ball, dt, e.h)
	}

	// Ball reaches left side
	if e.ball.X-e.ball.R <= 0 {
		e.ball.X = e.ball.R
		e.ball.VX = -e.ball.VX // bounce back right

		if e.paddle.Hits(e.ball) {
			e.streak = 0 // P1 streak resets
			if e.twoPlayer {
				e.onScorePaddle() // P2 scores on catch
			} else {
				callAudio("bounce")
			}
		} else {
			if e.twoPlayer {
				e.streak2 = 0 // P2 streak resets
			}
			e.onScore() // P1 scores when ball passes paddle
		}
		return
	}

	// Timer
	e.timeLeft -= dt
	if e.timeLeft <= 0 {
		e.timeLeft = 0
		e.state = StateGameOver
		e.nickLen = 0
		e.playMusic("stopMusic")
		callAudio("gameOver")
	}
}

func (e *Engine) onScore() {
	e.streak++
	if e.streak > e.bestStreak {
		e.bestStreak = e.streak
	}

	pts := levelPoints(e.level)
	multiplier := streakMultiplier(e.streak)
	e.score += int(float64(pts) * multiplier)

	e.scoreFlashTimer = 0.25
	e.scoreFlashRed = false
	callAudio("score")

	// Speed up ball slightly on each score
	speed := e.ball.VX * cfg.Game.BallSpeedupRate
	e.ball.VX = speed
	if e.ball.VY < 0 {
		e.ball.VY = -speed * 0.5
	} else {
		e.ball.VY = speed * 0.5
	}
}

// onScorePaddle awards a point to the paddle player (P2) in 2P mode.
func (e *Engine) onScorePaddle() {
	e.streak2++
	if e.streak2 > e.bestStreak2 {
		e.bestStreak2 = e.streak2
	}

	pts := levelPoints(e.level)
	multiplier := streakMultiplier(e.streak2)
	e.score2 += int(float64(pts) * multiplier)

	e.scoreFlashTimer = 0.25
	e.scoreFlashRed = true
	callAudio("score")

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

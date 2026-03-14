//go:build js && wasm

package main

import (
	"math"
	"syscall/js"
)

// registerInput sets up keydown/keyup listeners on the JS side.
func (e *Engine) registerInput() {
	js.Global().Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		key := args[0].Get("key").String()
		e.keys[key] = true
		e.handleKeyDown(key, args[0])
		return nil
	}))
	js.Global().Call("addEventListener", "keyup", js.FuncOf(func(_ js.Value, args []js.Value) any {
		key := args[0].Get("key").String()
		delete(e.keys, key)
		return nil
	}))

	// Clear held keys when the window loses focus to prevent ghost inputs.
	clearKeys := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		e.keys = make(map[string]bool)
		return nil
	})
	js.Global().Call("addEventListener", "blur", clearKeys)
	js.Global().Get("document").Call("addEventListener", "visibilitychange", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if js.Global().Get("document").Get("hidden").Bool() {
			e.keys = make(map[string]bool)
		}
		return nil
	}))
}

// handleKeyDown processes one-shot key actions (state transitions, nick entry).
func (e *Engine) handleKeyDown(key string, event js.Value) {
	switch e.state {
	case StateMenu:
		switch key {
		case "1":
			e.level = Easy
		case "2":
			e.level = Medium
		case "3":
			e.level = Hard
		case "ArrowLeft":
			if e.level > Easy {
				e.level--
			}
		case "ArrowRight":
			if e.level < Hard {
				e.level++
			}
		case "Tab":
			e.twoPlayer = !e.twoPlayer
			event.Call("preventDefault")
		case "Enter", " ":
			e.startGame()
		case "m", "M":
			e.musicEnabled = !e.musicEnabled
			if e.musicEnabled {
				callAudio("menuMusic")
			} else {
				callAudio("stopMusic")
			}
		case "s", "S":
			e.state = StateScoreboard
			go e.fetchScores("")
		}

	case StatePlaying:
		if key == "Escape" {
			e.state = StatePaused
		}

	case StatePaused:
		switch key {
		case "Escape", "p", "P":
			e.state = StatePlaying
		case "q", "Q":
			e.state = StateMenu
		}

	case StateGameOver:
		e.handleNickKey(key)

	case StateScoreboard:
		switch key {
		case "Escape", "r", "R":
			e.state = StateMenu
		case "ArrowLeft":
			e.prevScoreTab()
		case "ArrowRight":
			e.nextScoreTab()
		}
	}
}

// applyMovementInput reads held keys, gamepad, and touch to move the ball vertically.
func (e *Engine) applyMovementInput(dt float64) {
	speed := cfg.Game.PlayerSpeed
	dy := 0.0

	// Keyboard — player 1
	if e.keys["ArrowUp"] {
		dy -= speed * dt
	}
	if e.keys["ArrowDown"] {
		dy += speed * dt
	}

	// Keyboard — player 2 controls paddle (W/S)
	if e.twoPlayer {
		paddleSpeed := e.paddle.Speed
		pdiff := 0.0
		if e.keys["w"] || e.keys["W"] {
			pdiff -= paddleSpeed * dt
		}
		if e.keys["s"] || e.keys["S"] {
			pdiff += paddleSpeed * dt
		}
		if pdiff != 0 {
			e.paddle.Y += pdiff
			if e.paddle.Y < 0 {
				e.paddle.Y = 0
			}
			if e.paddle.Y+e.paddle.H > e.h {
				e.paddle.Y = e.h - e.paddle.H
			}
		}
	}

	// Gamepad
	dy += e.gamepadDY(dt)

	// Touch
	touchInput := js.Global().Get("touchInput")
	if touchInput.Get("active").Bool() {
		dy += touchInput.Get("deltaY").Float() * dt
	}

	if dy == 0 {
		return
	}

	e.ball.Y += dy
	// Clamp to canvas
	if e.ball.Y-e.ball.R < 0 {
		e.ball.Y = e.ball.R
	}
	if e.ball.Y+e.ball.R > e.h {
		e.ball.Y = e.h - e.ball.R
	}
}

// gamepadDY returns the vertical delta from the first connected gamepad.
func (e *Engine) gamepadDY(dt float64) float64 {
	gamepads := js.Global().Get("navigator").Call("getGamepads")
	if gamepads.IsNull() || gamepads.IsUndefined() {
		return 0
	}
	for i := 0; i < gamepads.Length(); i++ {
		gp := gamepads.Index(i)
		if gp.IsNull() || gp.IsUndefined() {
			continue
		}

		dy := 0.0

		// Left stick Y axis
		axes := gp.Get("axes")
		if axes.Length() > 1 {
			axisY := axes.Index(1).Float()
			if math.Abs(axisY) > 0.15 {
				dy += axisY * cfg.Game.PlayerSpeed * dt
			}
		}

		// D-Pad buttons 12 (up) / 13 (down)
		buttons := gp.Get("buttons")
		if buttons.Length() > 13 {
			if buttons.Index(12).Get("pressed").Bool() {
				dy -= cfg.Game.PlayerSpeed * dt
			}
			if buttons.Index(13).Get("pressed").Bool() {
				dy += cfg.Game.PlayerSpeed * dt
			}
		}

		if dy != 0 {
			return dy
		}
	}
	return 0
}

//go:build js && wasm

package main

import "syscall/js"

// Ball is the player-controlled dot.
type Ball struct {
	X, Y   float64
	VX, VY float64
	R      float64 // radius

	// LastWallHit is set each Update: -1 = hit top, +1 = hit bottom, 0 = no hit.
	LastWallHit int
}

// Update moves the ball and handles wall bounces (top, bottom, right).
// Returns true if the ball has exited through the left edge (score / miss check done by Engine).
func (b *Ball) Update(dt, canvasW, canvasH float64) {
	b.LastWallHit = 0
	b.X += b.VX * dt
	b.Y += b.VY * dt

	// Top / bottom walls
	if b.Y-b.R < 0 {
		b.Y = b.R
		b.VY = -b.VY
		b.enforceMinAngle()
		b.LastWallHit = -1
		callAudio("bounce")
	} else if b.Y+b.R > canvasH {
		b.Y = canvasH - b.R
		b.VY = -b.VY
		b.enforceMinAngle()
		b.LastWallHit = 1
		callAudio("bounce")
	}

	// Right wall
	if b.X+b.R > canvasW {
		b.X = canvasW - b.R
		b.VX = -b.VX
		callAudio("bounce")
	}
}

// enforceMinAngle ensures VY is at least 20% of |VX| so the ball
// always leaves a wall at a meaningful angle and cannot slide along it.
func (b *Ball) enforceMinAngle() {
	minVY := b.VX * 0.20
	if minVY < 0 {
		minVY = -minVY
	}
	if b.VY < 0 && b.VY > -minVY {
		b.VY = -minVY
	} else if b.VY >= 0 && b.VY < minVY {
		b.VY = minVY
	}
}

// Draw renders the ball as a glowing circle.
func (b *Ball) Draw(ctx js.Value, color, glowColor string) {
	// Glow shadow
	ctx.Set("shadowBlur", 30)
	ctx.Set("shadowColor", glowColor)

	// Radial gradient: bright centre → dim edge
	grad := ctx.Call("createRadialGradient",
		b.X, b.Y, b.R*0.1,
		b.X, b.Y, b.R,
	)
	grad.Call("addColorStop", 0, "#ffffff")
	grad.Call("addColorStop", 0.4, color)
	grad.Call("addColorStop", 1, "rgba(0,0,0,0)")

	ctx.Set("fillStyle", grad)
	ctx.Call("beginPath")
	ctx.Call("arc", b.X, b.Y, b.R, 0, 6.283185307)
	ctx.Call("fill")

	ctx.Set("shadowBlur", 0)
}

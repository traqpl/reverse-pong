//go:build js && wasm

package main

import (
	"math"
	"math/rand"
	"syscall/js"
)

// AILevel controls paddle intelligence.
type AILevel int

const (
	Easy   AILevel = 1
	Medium AILevel = 2
	Hard   AILevel = 3
)

// Paddle is the AI-controlled bat on the left side.
type Paddle struct {
	X, Y  float64
	W, H  float64
	Speed float64
	Level AILevel

	// internal AI state
	targetY      float64
	jitterTimer  float64
	jitterOffset float64
	errorTimer   float64
	errorOffset  float64
	lc           LevelSettings // level config snapshot
}

func newPaddle(level AILevel, canvasH float64, lc LevelSettings) Paddle {
	h := 80.0
	return Paddle{
		X:     0,
		Y:     (canvasH - h) / 2,
		W:     12,
		H:     h,
		Speed: lc.PaddleSpeed,
		Level: level,
		lc:    lc,
	}
}

// Update moves the paddle toward the predicted ball position.
func (p *Paddle) Update(ball Ball, dt, canvasH float64) {
	switch p.Level {
	case Easy:
		p.updateEasy(ball, dt, canvasH)
	case Medium:
		p.updateMedium(ball, dt, canvasH)
	case Hard:
		p.updateHard(ball, dt, canvasH)
	}

	// Clamp to canvas
	if p.Y < 0 {
		p.Y = 0
	}
	if p.Y+p.H > canvasH {
		p.Y = canvasH - p.H
	}
}

func (p *Paddle) updateEasy(ball Ball, dt, canvasH float64) {
	p.jitterTimer -= dt
	if p.jitterTimer <= 0 {
		p.jitterTimer = p.lc.JitterInterval
		p.jitterOffset = (rand.Float64()*2 - 1) * p.lc.JitterRange
	}

	target := ball.Y + p.jitterOffset
	diff := target - (p.Y + p.H/2)

	if math.Abs(diff) < p.lc.DeadZone {
		return
	}

	move := math.Copysign(p.Speed*dt, diff)
	if math.Abs(move) > math.Abs(diff) {
		move = diff
	}
	p.Y += move
}

func (p *Paddle) updateMedium(ball Ball, dt, canvasH float64) {
	p.errorTimer -= dt
	if p.errorTimer <= 0 {
		p.errorTimer = p.lc.ErrorInterval + rand.Float64()*1.2
		if rand.Float64() < p.lc.ErrorChance {
			p.errorOffset = (rand.Float64()*2 - 1) * p.lc.ErrorRange
		} else {
			p.errorOffset = 0
		}
	}

	if ball.VX < 0 {
		p.targetY = predictBallY(ball, p.X+p.W, canvasH) + p.errorOffset
	} else {
		p.targetY = ball.Y + p.errorOffset
	}

	diff := p.targetY - (p.Y + p.H/2)
	if math.Abs(diff) < p.lc.DeadZone {
		return
	}

	move := math.Copysign(p.Speed*dt, diff)
	if math.Abs(move) > math.Abs(diff) {
		move = diff
	}
	p.Y += move
}

func (p *Paddle) updateHard(ball Ball, dt, canvasH float64) {
	p.errorTimer -= dt
	if p.errorTimer <= 0 {
		p.errorTimer = p.lc.ErrorInterval + rand.Float64()*2.0
		if rand.Float64() < p.lc.ErrorChance {
			p.errorOffset = (rand.Float64()*2 - 1) * p.lc.ErrorRange
		} else {
			p.errorOffset = 0
		}
	}

	if ball.VX < 0 {
		p.targetY = predictBallY(ball, p.X+p.W, canvasH) + p.errorOffset
	} else {
		p.targetY = ball.Y + p.errorOffset
	}

	diff := p.targetY - (p.Y + p.H/2)
	if math.Abs(diff) < p.lc.DeadZone {
		return
	}

	move := math.Copysign(p.Speed*dt, diff)
	if math.Abs(move) > math.Abs(diff) {
		move = diff
	}
	p.Y += move
}

// predictBallY simulates the ball's path until it reaches targetX,
// accounting for top/bottom wall bounces.
func predictBallY(ball Ball, targetX, canvasH float64) float64 {
	x, y, vx, vy := ball.X, ball.Y, ball.VX, ball.VY
	r := ball.R

	const step = 0.004 // 4ms steps
	const maxIter = 600

	for i := 0; i < maxIter; i++ {
		x += vx * step
		y += vy * step

		if y-r < 0 {
			y = r
			vy = -vy
		} else if y+r > canvasH {
			y = canvasH - r
			vy = -vy
		}
		if x+r > 800 { // right wall
			x = 800 - r
			vx = -vx
		}

		if vx < 0 && x <= targetX {
			return y
		}
	}
	return y
}

// Hits returns true if the ball collides with the paddle.
func (p *Paddle) Hits(ball Ball) bool {
	return ball.X-ball.R <= p.X+p.W &&
		ball.Y >= p.Y &&
		ball.Y <= p.Y+p.H
}

// Draw renders the paddle with a glow.
func (p *Paddle) Draw(ctx js.Value, color string) {
	ctx.Set("shadowBlur", 12)
	ctx.Set("shadowColor", color)
	ctx.Set("fillStyle", color)
	ctx.Call("fillRect", p.X, p.Y, p.W, p.H)
	ctx.Set("shadowBlur", 0)
}

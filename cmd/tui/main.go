package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

// ── window / field ────────────────────────────────────────────────────────────

// logicCols / logicRows — fixed logical game space (physics always runs here).
// The renderer maps each logical cell to rScale×rScale terminal cells.
const (
	logicCols = 44
	logicRows = 23
	hudRows   = 2
	botWall   = 1
)

// fieldCols / fieldRows kept as aliases so physics code stays unchanged.
const (
	fieldCols = logicCols
	fieldRows = logicRows
)

// ── gameplay constants ────────────────────────────────────────────────────────

const (
	paddleWidth = 2
	paddleH     = 5.0
	playerSpeed = 18.0

	ballSpeedupRate = 1.05
	countdownStep   = 0.8
	goDuration      = 0.6
	gameDuration    = 60.0

	holdThreshold = 90 * time.Millisecond
)

// ── levels ────────────────────────────────────────────────────────────────────

type levelCfg struct {
	ballSpeed, paddleSpeed      float64
	points                      int
	jitterRange, jitterInt      float64
	errChance, errRange, errInt float64
	deadZone                    float64
}

var lvls = [4]levelCfg{
	{},
	{ballSpeed: 16, paddleSpeed: 13, points: 1, jitterRange: 1.0, jitterInt: 0.4, deadZone: 0.5},
	{ballSpeed: 18, paddleSpeed: 18, points: 2, errChance: 0.30, errRange: 3, errInt: 1.8, deadZone: 0.6},
	{ballSpeed: 22, paddleSpeed: 26, points: 3, errChance: 0.08, errRange: 2, errInt: 3.0, deadZone: 0.15},
}

var lvlNames = [4]string{"", "EASY", "MEDIUM", "HARD"}
var lvlKeys = [4]string{"", "easy", "medium", "hard"}

// ── types ─────────────────────────────────────────────────────────────────────

type GameState int

const (
	StateMenu GameState = iota
	StateCountdown
	StatePlaying
	StatePaused
	StateNickInput
	StateScoreboard
)

type Ball struct{ X, Y, VX, VY float64 }

type Paddle struct {
	Y, H        float64
	jitterTimer float64
	jitterOff   float64
	errorTimer  float64
	errorOff    float64
	targetY     float64
}

type Engine struct {
	scr tcell.Screen

	// render state (recomputed on resize)
	termW, termH int // full terminal size
	rScale       int // logical cell → N terminal cells
	fOffX, fOffY int // screen offset of field top-left corner

	state GameState
	level int

	ball   Ball
	paddle Paddle

	score    int
	timeLeft float64

	countdownDigit int
	countdownTimer float64

	lastUp   time.Time
	lastDown time.Time

	// nick input
	nickBuf [3]rune
	nickLen int

	// scoreboard
	sb *scoreboardState
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	initAudio()

	scr, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = scr.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer scr.Fini()

	scr.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))
	scr.HideCursor()
	scr.Clear()

	e := &Engine{scr: scr, level: 2, sb: newScoreboardState()}
	e.calcScale()
	e.resetBall()
	e.resetPaddle()

	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	evCh := make(chan tcell.Event, 64)
	go func() {
		for {
			ev := scr.PollEvent()
			if ev == nil {
				return
			}
			evCh <- ev
		}
	}()

	running := true
	var lastTick time.Time

	for running {
		<-ticker.C

		now := time.Now()
		dt := 0.0
		if !lastTick.IsZero() {
			dt = now.Sub(lastTick).Seconds()
			if dt > 0.05 {
				dt = 0.05
			}
		}
		lastTick = now

	drain:
		for {
			select {
			case ev := <-evCh:
				if !e.processEvent(ev) {
					running = false
				}
			default:
				break drain
			}
		}

		if !running {
			break
		}
		e.update(dt)
		e.draw()
	}
}

// ── render geometry ───────────────────────────────────────────────────────────

func (e *Engine) calcScale() {
	e.termW, e.termH = e.scr.Size()
	avail := e.termH - hudRows - botWall
	byW := e.termW / logicCols
	byH := avail / logicRows
	e.rScale = byW
	if byH < e.rScale {
		e.rScale = byH
	}
	if e.rScale < 1 {
		e.rScale = 1
	}
	fw := logicCols * e.rScale
	fh := logicRows * e.rScale
	e.fOffX = (e.termW - fw) / 2
	e.fOffY = hudRows + (avail-fh)/2
}

// ── reset ─────────────────────────────────────────────────────────────────────

func (e *Engine) lc() levelCfg { return lvls[e.level] }

func (e *Engine) resetBall() {
	lc := e.lc()
	e.ball = Ball{
		X: fieldCols / 2, Y: fieldRows / 2,
		VX: lc.ballSpeed, VY: lc.ballSpeed * 0.5,
	}
}

func (e *Engine) resetPaddle() {
	e.paddle = Paddle{Y: fieldRows/2 - paddleH/2, H: paddleH}
}

func (e *Engine) startGame() {
	e.score = 0
	e.timeLeft = gameDuration
	e.resetBall()
	e.resetPaddle()
	e.countdownDigit = 3
	e.countdownTimer = countdownStep
	e.state = StateCountdown
}

// ── input ─────────────────────────────────────────────────────────────────────

func (e *Engine) processEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		e.scr.Sync()
		e.calcScale()
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyCtrlC {
			return false
		}
		now := time.Now()
		if ev.Key() == tcell.KeyUp {
			e.lastUp = now
		}
		if ev.Key() == tcell.KeyDown {
			e.lastDown = now
		}
		e.handleKey(ev)
	}
	return true
}

func (e *Engine) handleKey(ev *tcell.EventKey) {
	r := ev.Rune()
	k := ev.Key()

	switch e.state {
	case StateMenu:
		switch k {
		case tcell.KeyUp:
			if e.level > 1 {
				e.level--
			}
		case tcell.KeyDown:
			if e.level < 3 {
				e.level++
			}
		case tcell.KeyEnter:
			e.startGame()
		case tcell.KeyRune:
			switch r {
			case '1':
				e.level = 1
			case '2':
				e.level = 2
			case '3':
				e.level = 3
			case 's', 'S':
				e.state = StateScoreboard
			}
		}

	case StatePlaying:
		if k == tcell.KeyEscape || (k == tcell.KeyRune && (r == 'p' || r == 'P')) {
			e.state = StatePaused
		}

	case StatePaused:
		switch {
		case k == tcell.KeyEscape || (k == tcell.KeyRune && (r == 'p' || r == 'P')):
			e.state = StatePlaying
		case k == tcell.KeyRune && (r == 'q' || r == 'Q'):
			e.state = StateMenu
		}

	case StateNickInput:
		e.handleNickKey(k, r)

	case StateScoreboard:
		switch k {
		case tcell.KeyEscape:
			e.state = StateMenu
		case tcell.KeyUp:
			e.sb.scrollUp()
		case tcell.KeyDown:
			e.sb.scrollDown()
		case tcell.KeyLeft:
			e.sb.prevTab()
		case tcell.KeyRight:
			e.sb.nextTab()
		case tcell.KeyRune:
			if r == 'r' || r == 'R' {
				e.state = StateMenu
			}
		}
	}
}

func (e *Engine) handleNickKey(k tcell.Key, r rune) {
	switch k {
	case tcell.KeyEscape:
		// skip to scoreboard without saving
		e.state = StateScoreboard

	case tcell.KeyRune:
		if e.nickLen < 3 {
			c := r
			if c >= 'a' && c <= 'z' {
				c -= 32
			}
			if c >= 'A' && c <= 'Z' {
				e.nickBuf[e.nickLen] = c
				e.nickLen++
			}
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.nickLen > 0 {
			e.nickLen--
		}
	case tcell.KeyEnter:
		if e.nickLen == 3 {
			nick := string(e.nickBuf[:3])
			e.sb.add(nick, e.score, lvlKeys[e.level])
			e.sb.pendingNick = nick
			e.sb.pendingScore = e.score
			e.sb.scroll = 0
			e.state = StateScoreboard
		}
	}
}

// ── update ────────────────────────────────────────────────────────────────────

func (e *Engine) update(dt float64) {
	switch e.state {
	case StateCountdown:
		e.updateCountdown(dt)
	case StatePlaying:
		e.updatePlaying(dt)
	}
}

func (e *Engine) updateCountdown(dt float64) {
	e.countdownTimer -= dt
	if e.countdownTimer > 0 {
		return
	}
	e.countdownDigit--
	if e.countdownDigit > 0 {
		e.countdownTimer = countdownStep
		soundCountdown()
	} else if e.countdownDigit == 0 {
		e.countdownTimer = goDuration
		soundGo()
	} else {
		e.state = StatePlaying
	}
}

func (e *Engine) updatePlaying(dt float64) {
	if time.Since(e.lastUp) < holdThreshold {
		e.ball.Y -= playerSpeed * dt
	}
	if time.Since(e.lastDown) < holdThreshold {
		e.ball.Y += playerSpeed * dt
	}

	e.ball.X += e.ball.VX * dt
	e.ball.Y += e.ball.VY * dt

	if e.ball.Y < 0 {
		e.ball.Y = 0
		e.ball.VY = math.Abs(e.ball.VY)
		soundBounce()
	}
	if e.ball.Y >= fieldRows {
		e.ball.Y = fieldRows - 1
		e.ball.VY = -math.Abs(e.ball.VY)
		soundBounce()
	}
	if e.ball.X >= fieldCols-1 {
		e.ball.X = fieldCols - 2
		e.ball.VX = -math.Abs(e.ball.VX)
		soundBounce()
	}

	e.updatePaddle(dt)

	if e.ball.X < float64(paddleWidth) {
		if e.ball.Y >= e.paddle.Y && e.ball.Y <= e.paddle.Y+e.paddle.H {
			// Smooth bounce off paddle — no freeze, no flash
			e.ball.X = float64(paddleWidth)
			e.ball.VX = math.Abs(e.ball.VX)
			e.ball.VY += (rand.Float64()*2 - 1) * e.lc().ballSpeed * 0.15
			soundHit()
		} else {
			// Player scores
			e.score += e.lc().points
			soundScore()

			speed := math.Abs(e.ball.VX) * ballSpeedupRate
			e.ball.X = fieldCols / 2
			e.ball.Y = fieldRows / 2
			e.ball.VX = speed
			if e.ball.VY < 0 {
				e.ball.VY = -speed * 0.5
			} else {
				e.ball.VY = speed * 0.5
			}
		}
	}

	e.timeLeft -= dt
	if e.timeLeft <= 0 {
		e.timeLeft = 0
		e.nickLen = 0
		e.state = StateNickInput
		soundGameOver()
	}
}

func (e *Engine) updatePaddle(dt float64) {
	lc := e.lc()
	p := &e.paddle

	switch e.level {
	case 1:
		p.jitterTimer -= dt
		if p.jitterTimer <= 0 {
			p.jitterTimer = lc.jitterInt
			p.jitterOff = (rand.Float64()*2 - 1) * lc.jitterRange
		}
		target := e.ball.Y + p.jitterOff
		diff := target - (p.Y + p.H/2)
		if math.Abs(diff) > lc.deadZone {
			move := math.Copysign(lc.paddleSpeed*dt, diff)
			if math.Abs(move) > math.Abs(diff) {
				move = diff
			}
			p.Y += move
		}

	case 2, 3:
		p.errorTimer -= dt
		if p.errorTimer <= 0 {
			p.errorTimer = lc.errInt + rand.Float64()*1.2
			if rand.Float64() < lc.errChance {
				p.errorOff = (rand.Float64()*2 - 1) * lc.errRange
			} else {
				p.errorOff = 0
			}
		}
		if e.ball.VX < 0 {
			p.targetY = predictY(e.ball, float64(paddleWidth), fieldCols, fieldRows) + p.errorOff
		} else {
			p.targetY = e.ball.Y + p.errorOff
		}
		diff := p.targetY - (p.Y + p.H/2)
		if math.Abs(diff) > lc.deadZone {
			move := math.Copysign(lc.paddleSpeed*dt, diff)
			if math.Abs(move) > math.Abs(diff) {
				move = diff
			}
			p.Y += move
		}
	}

	if p.Y < 0 {
		p.Y = 0
	}
	if p.Y+p.H > fieldRows {
		p.Y = fieldRows - p.H
	}
}

func predictY(ball Ball, targetX, fw, fh float64) float64 {
	x, y, vx, vy := ball.X, ball.Y, ball.VX, ball.VY
	for i := 0; i < 5000; i++ {
		x += vx * 0.01
		y += vy * 0.01
		if y < 0 {
			y = 0
			vy = -vy
		} else if y >= fh {
			y = fh - 1
			vy = -vy
		}
		if x >= fw-1 {
			x = fw - 2
			vx = -vx
		}
		if vx < 0 && x <= targetX {
			return y
		}
	}
	return y
}

func streakMult(streak int) float64 {
	if streak >= 5 {
		return 2.0
	}
	if streak >= 3 {
		return 1.5
	}
	return 1.0
}

// ── styles ────────────────────────────────────────────────────────────────────

var (
	styleDefault  = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	stylePaddle   = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGreen).Bold(true)
	styleBall     = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorYellow).Bold(true)
	styleWall     = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)
	styleHUD      = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorTeal)
	styleSep      = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)
	styleTitle    = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorYellow).Bold(true)
	styleSelected = tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack).Bold(true)
	styleFlash    = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorRed).Bold(true)
	styleGood     = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGreen).Bold(true)
	styleDim      = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)
)

// ── draw ──────────────────────────────────────────────────────────────────────

func (e *Engine) draw() {
	e.scr.Clear()
	switch e.state {
	case StateMenu:
		e.drawMenu()
	case StateCountdown:
		e.drawHUD()
		e.drawField()
		e.drawCountdown()
	case StatePlaying:
		e.drawHUD()
		e.drawField()
	case StatePaused:
		e.drawHUD()
		e.drawField()
		e.drawCentered(e.fOffY+logicRows*e.rScale/2, "  PAUSED — p: resume  q: menu  ", styleTitle)
	case StateNickInput:
		e.drawNickInput()
	case StateScoreboard:
		e.drawScoreboard()
	}
	e.scr.Show()
}

func (e *Engine) drawHUD() {
	hud := fmt.Sprintf(" SCORE: %-5d  TIME: %4.0fs  [%s] ",
		e.score, e.timeLeft, lvlNames[e.level])
	e.printAt(0, 0, hud, styleHUD)
	for x := len([]rune(hud)); x < e.termW; x++ {
		e.scr.SetContent(x, 0, ' ', nil, styleHUD)
	}
	for x := 0; x < e.termW; x++ {
		e.scr.SetContent(x, 1, '─', nil, styleSep)
	}
}

func (e *Engine) drawField() {
	s := e.rScale
	ox, oy := e.fOffX, e.fOffY
	fw := logicCols * s
	fh := logicRows * s

	// Bottom wall
	for x := ox; x <= ox+fw; x++ {
		e.scr.SetContent(x, oy+fh, '─', nil, styleWall)
	}
	e.scr.SetContent(ox+fw, oy+fh, '┘', nil, styleWall)

	// Right wall
	for row := 0; row < fh; row++ {
		e.scr.SetContent(ox+fw, oy+row, '│', nil, styleWall)
	}

	// Left wall + paddle
	paddleTop := int(math.Round(e.paddle.Y)) * s
	paddleBot := int(math.Round(e.paddle.Y+e.paddle.H))*s - 1
	for row := 0; row < fh; row++ {
		if row >= paddleTop && row <= paddleBot {
			e.scr.SetContent(ox, oy+row, '█', nil, stylePaddle)
			e.scr.SetContent(ox+1, oy+row, '▌', nil, stylePaddle)
		} else {
			e.scr.SetContent(ox, oy+row, '│', nil, styleWall)
		}
	}

	// Ball — centered in its scaled cell
	bx := int(math.Round(e.ball.X))*s + s/2
	by := int(math.Round(e.ball.Y))*s + s/2
	if bx >= paddleWidth && bx < fw && by >= 0 && by < fh {
		e.scr.SetContent(ox+bx, oy+by, '●', nil, styleBall)
	}
}

func (e *Engine) drawCountdown() {
	cy := e.fOffY + logicRows*e.rScale/2
	var label string
	if e.countdownDigit > 0 {
		label = fmt.Sprintf("  %d  ", e.countdownDigit)
	} else {
		label = "  GO!  "
	}
	e.drawCentered(cy, label, styleTitle)
}

func (e *Engine) drawMenu() {
	e.fillBg()

	// Row 0: scrolling HOF ticker
	e.drawTicker()
	// Row 1: separator
	for x := 0; x < e.termW; x++ {
		e.scr.SetContent(x, 1, '─', nil, styleSep)
	}

	cy := e.termH / 2
	e.drawCentered(cy-3, "╔═══════════════════════╗", styleWall)
	e.drawCentered(cy-2, "║  R E V E R S E  P O N G  ║", styleTitle)
	e.drawCentered(cy-1, "╚═══════════════════════╝", styleWall)
	e.drawCentered(cy, "↑↓ move ball   score by sneaking past the paddle", styleDefault)
	for i, name := range []string{"EASY", "MEDIUM", "HARD"} {
		lvl := i + 1
		st := styleDefault
		if lvl == e.level {
			st = styleSelected
		}
		pts := []int{1, 2, 3}
		e.drawCentered(cy+2+i, fmt.Sprintf("  [%d] %s  +%dpt  ", lvl, name, pts[i]), st)
	}
	e.drawCentered(cy+6, "ENTER to start   S: hall of fame   Ctrl+C: quit", styleDim)
}

func (e *Engine) drawTicker() {
	text := e.sb.tickerText()
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return
	}
	// scroll right→left: offset increases over time
	offset := int(time.Now().UnixMilli()/80) % n
	for x := 0; x < e.termW; x++ {
		idx := (offset + x) % n
		e.scr.SetContent(x, 0, runes[idx], nil, styleTitle)
	}
}

func (e *Engine) drawNickInput() {
	e.fillBg()
	cy := e.termH / 2

	e.drawCentered(cy-4, "  GAME OVER  ", styleFlash)
	e.drawCentered(cy-2, fmt.Sprintf("  SCORE: %d  ", e.score), styleTitle)

	e.drawCentered(cy+1, "  enter your initials:  ", styleDim)

	// build display: filled slots + blinking cursor + empty slots
	slots := [3]string{"_", "_", "_"}
	for i := 0; i < e.nickLen; i++ {
		slots[i] = string(e.nickBuf[i])
	}
	cursor := " "
	if e.nickLen < 3 && (time.Now().UnixMilli()/400)%2 == 0 {
		cursor = "█"
		slots[e.nickLen] = cursor
	}
	_ = cursor
	line := fmt.Sprintf("  %s  %s  %s  ", slots[0], slots[1], slots[2])
	e.drawCentered(cy+2, line, styleTitle)

	e.drawCentered(cy+4, "  ENTER: save   ESC: skip  ", styleDim)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (e *Engine) printAt(x, y int, s string, st tcell.Style) {
	for i, r := range s {
		e.scr.SetContent(x+i, y, r, nil, st)
	}
}

func (e *Engine) drawCentered(y int, s string, st tcell.Style) {
	x := (e.termW - len([]rune(s))) / 2
	if x < 0 {
		x = 0
	}
	e.printAt(x, y, s, st)
}

func (e *Engine) fillBg() {
	for row := 0; row < e.termH; row++ {
		for col := 0; col < e.termW; col++ {
			e.scr.SetContent(col, row, ' ', nil, styleDefault)
		}
	}
}

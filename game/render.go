//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall/js"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// renderScanlines draws CRT-style horizontal scanlines over the entire canvas.
func (e *Engine) renderScanlines() {
	e.ctx.Set("strokeStyle", "rgba(0,0,0,0.18)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	for y := 2.0; y < e.h; y += 4 {
		e.ctx.Call("moveTo", 0, y)
		e.ctx.Call("lineTo", e.w, y)
	}
	e.ctx.Call("stroke")
}

func (e *Engine) clear() {
	e.ctx.Set("fillStyle", "#0b0c0b")
	e.ctx.Call("fillRect", 0, 0, e.w, e.h)
}

// crtColor returns the Display P3 fill colour for the active theme.
// Values extend slightly beyond sRGB for extra vibrancy on P3 displays.
func (e *Engine) crtColor() string {
	theme := js.Global().Get("crtTheme").String()
	switch theme {
	case "theme-green":
		return "color(display-p3 0.50 1.00 0.50)"
	case "theme-cyan":
		return "color(display-p3 0.48 0.92 1.00)"
	default: // theme-amber
		return "color(display-p3 1.00 0.78 0.38)"
	}
}

// glowColor returns a highly saturated Display P3 colour used for shadowColor.
// More saturated than crtColor so the bloom looks intense on HDR/P3 displays.
func (e *Engine) glowColor() string {
	theme := js.Global().Get("crtTheme").String()
	switch theme {
	case "theme-green":
		return "color(display-p3 0.10 1.00 0.10)"
	case "theme-cyan":
		return "color(display-p3 0.00 0.82 1.00)"
	default: // theme-amber
		return "color(display-p3 1.00 0.72 0.00)"
	}
}

func (e *Engine) glow(blur float64) {
	e.ctx.Set("shadowBlur", blur)
	e.ctx.Set("shadowColor", e.glowColor())
}

func (e *Engine) noGlow() {
	e.ctx.Set("shadowBlur", 0)
}

func (e *Engine) text(s string, x, y float64, size int, align string) {
	e.ctx.Set("font", fmt.Sprintf("%dpx VT323, monospace", size))
	e.ctx.Set("textAlign", align)
	e.ctx.Set("textBaseline", "middle")
	e.ctx.Set("fillStyle", e.crtColor())
	e.ctx.Call("fillText", s, x, y)
}

func nowMS() float64 {
	return js.Global().Get("performance").Call("now").Float()
}

// ── Menu ─────────────────────────────────────────────────────────────────────

func (e *Engine) renderMenu() {
	e.clear()
	color := e.crtColor()
	t := nowMS() / 1000.0

	// Pulsing title glow
	pulse := 20 + 32*math.Abs(math.Sin(t*1.8))
	e.ctx.Set("shadowBlur", pulse)
	e.ctx.Set("shadowColor", color)
	e.text("REVERSE PONG", e.w/2, e.h*0.22, 64, "center")
	e.noGlow()

	// Difficulty buttons
	levels := []struct {
		l    AILevel
		name string
	}{
		{Easy, "EASY"},
		{Medium, "MEDIUM"},
		{Hard, "HARD"},
	}
	btnW, btnH := 120.0, 40.0
	gap := 20.0
	totalW := float64(len(levels))*btnW + float64(len(levels)-1)*gap
	startX := (e.w - totalW) / 2

	for i, lv := range levels {
		bx := startX + float64(i)*(btnW+gap)
		by := e.h*0.42 - btnH/2
		active := e.level == lv.l

		if active {
			e.glow(24)
			e.ctx.Set("fillStyle", color)
		} else {
			e.noGlow()
			e.ctx.Set("fillStyle", "rgba(255,255,255,0.08)")
		}
		e.ctx.Call("fillRect", bx, by, btnW, btnH)

		e.ctx.Set("strokeStyle", color)
		e.ctx.Set("lineWidth", 1)
		e.ctx.Call("strokeRect", bx, by, btnW, btnH)

		if active {
			e.ctx.Set("fillStyle", "#0b0c0b")
		} else {
			e.ctx.Set("fillStyle", color)
		}
		e.ctx.Set("font", "22px VT323, monospace")
		e.ctx.Set("textAlign", "center")
		e.ctx.Set("textBaseline", "middle")
		e.ctx.Call("fillText", lv.name, bx+btnW/2, by+btnH/2)
		e.noGlow()
	}

	// Points per level hint
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
	e.text("EASY=1pt  MEDIUM=2pt  HARD=3pt  |  STREAK ×1.5 @ 3,  ×2 @ 5", e.w/2, e.h*0.54, 18, "center")

	// 1P / 2P toggle
	modeLabels := []string{"1 PLAYER", "2 PLAYERS"}
	modeBtnW, modeBtnH := 130.0, 36.0
	modeGap := 14.0
	modeTotalW := 2*modeBtnW + modeGap
	modeStartX := (e.w - modeTotalW) / 2
	for i, label := range modeLabels {
		bx := modeStartX + float64(i)*(modeBtnW+modeGap)
		by := e.h*0.63 - modeBtnH/2
		active := (i == 1) == e.twoPlayer

		if active {
			e.glow(20)
			e.ctx.Set("fillStyle", color)
		} else {
			e.noGlow()
			e.ctx.Set("fillStyle", "rgba(255,255,255,0.08)")
		}
		e.ctx.Call("fillRect", bx, by, modeBtnW, modeBtnH)
		e.ctx.Set("strokeStyle", color)
		e.ctx.Set("lineWidth", 1)
		e.ctx.Call("strokeRect", bx, by, modeBtnW, modeBtnH)

		if active {
			e.ctx.Set("fillStyle", "#0b0c0b")
		} else {
			e.ctx.Set("fillStyle", color)
		}
		e.ctx.Set("font", "20px VT323, monospace")
		e.ctx.Set("textAlign", "center")
		e.ctx.Set("textBaseline", "middle")
		e.ctx.Call("fillText", label, bx+modeBtnW/2, by+modeBtnH/2)
		e.noGlow()
	}
	if e.twoPlayer {
		e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
		e.text("P1: ARROWS  P2: W/S", e.w/2, e.h*0.63+28, 18, "center")
	}

	// Blink "PRESS ENTER"
	if int(nowMS()/600)%2 == 0 {
		e.glow(16)
		e.text("PRESS ENTER TO START", e.w/2, e.h*0.78, 28, "center")
		e.noGlow()
	}

	// Match statistics
	e.renderMenuStats()

	// Music toggle indicator
	musicLabel := "[M] MUSIC: ON"
	if !e.musicEnabled {
		musicLabel = "[M] MUSIC: OFF"
	}
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
	e.text("[S] SCOREBOARD   [1/2/3] DIFFICULTY   [←/→] DIFFICULTY   [TAB] MODE   "+musicLabel, e.w/2, e.h-24, 18, "center")

	// Build timestamp — bottom right corner
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.20)")
	e.text(BuildTime, e.w-8, e.h-8, 14, "right")
}

func (e *Engine) renderMenuStats() {
	s := e.menuStats
	if s == nil {
		return
	}

	const x = 8.0
	y1 := e.h * 0.855
	y2 := e.h * 0.895

	// "STATS" label on the left, spanning both lines vertically
	e.text("STATS", x, (y1+y2)/2, 17, "left")

	const indent = 58.0

	// 1P line
	line1p := fmt.Sprintf("1P  EASY:%d  MEDIUM:%d  HARD:%d",
		s.Matches1P["easy"], s.Matches1P["medium"], s.Matches1P["hard"])
	e.text(line1p, x+indent, y1, 17, "left")

	// 2P line
	line2p := "2P"
	for _, lv := range []string{"easy", "medium", "hard"} {
		st, ok := s.Matches2P[lv]
		if !ok || st.Total == 0 {
			continue
		}
		line2p += fmt.Sprintf("  %s P1:%d/P2:%d/DRAW:%d", strings.ToUpper(lv[:3]), st.P1, st.P2, st.Draw)
	}
	if line2p == "2P" {
		line2p = "2P  no matches yet"
	}
	e.text(line2p, x+indent, y2, 17, "left")
}

// ── Countdown ────────────────────────────────────────────────────────────────

func (e *Engine) renderCountdown() {
	e.clear()
	var label string
	if e.countdownDigit > 0 {
		label = fmt.Sprintf("%d", e.countdownDigit)
	} else {
		label = "GO!"
	}

	e.ctx.Set("shadowBlur", 65)
	e.ctx.Set("shadowColor", e.glowColor())
	e.text(label, e.w/2, e.h/2, 96, "center")
	e.noGlow()
}

// ── Playing ───────────────────────────────────────────────────────────────────

func (e *Engine) renderPlaying() {
	e.clear()
	color := e.crtColor()

	// Right wall
	e.ctx.Set("strokeStyle", color)
	e.ctx.Set("lineWidth", 3)
	e.glow(14)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", e.w-2, 0)
	e.ctx.Call("lineTo", e.w-2, e.h)
	e.ctx.Call("stroke")
	e.ctx.Set("fillStyle", color)
	e.noGlow()

	// Paddle
	e.paddle.Draw(e.ctx, color, e.glowColor())

	// Ball
	e.ball.Draw(e.ctx, color, e.glowColor())

	// HUD
	e.renderHUD()

	// Green flash on score
	e.renderScoreFlash()
}

func (e *Engine) renderHUD() {
	color := e.crtColor()
	e.glow(14)
	e.ctx.Set("fillStyle", color)

	if e.twoPlayer {
		// 2P layout: P1 top-left | TIME centre | P2 top-right
		e.text(fmt.Sprintf("P1 %d", e.score), 8, 20, 28, "left")
		e.text(fmt.Sprintf("%d P2", e.score2), e.w-8, 20, 28, "right")

		secs := int(math.Ceil(e.timeLeft))
		if e.timeLeft < 10 {
			e.ctx.Set("fillStyle", "#ff4444")
			e.ctx.Set("shadowColor", "#ff0000")
		}
		e.text(fmt.Sprintf("%d:%02d", secs/60, secs%60), e.w/2, 20, 24, "center")
		e.ctx.Set("fillStyle", color)
		e.ctx.Set("shadowColor", e.glowColor())

		// P1 streak bottom-left, P2 streak bottom-right
		if e.streak >= 3 {
			e.text(fmt.Sprintf("P1 ×%.1f", streakMultiplier(e.streak)), 8, e.h-20, 20, "left")
		}
		if e.streak2 >= 3 {
			e.text(fmt.Sprintf("P2 ×%.1f", streakMultiplier(e.streak2)), e.w-8, e.h-20, 20, "right")
		}
	} else {
		// 1P layout: SCORE left | LEVEL centre | TIME right
		e.text(strings.ToUpper(levelName(e.level)), e.w/2, 20, 20, "center")
		e.text(fmt.Sprintf("SCORE: %d", e.score), 8, 20, 24, "left")

		secs := int(math.Ceil(e.timeLeft))
		timeStr := fmt.Sprintf("TIME: %d:%02d", secs/60, secs%60)
		if e.timeLeft < 10 {
			e.ctx.Set("fillStyle", "#ff4444")
			e.ctx.Set("shadowColor", "#ff0000")
		}
		e.text(timeStr, e.w-8, 20, 24, "right")

		if e.streak >= 3 {
			e.ctx.Set("fillStyle", color)
			e.ctx.Set("shadowColor", color)
			mult := streakMultiplier(e.streak)
			e.text(fmt.Sprintf("STREAK %d  ×%.1f", e.streak, mult), e.w/2, e.h-20, 22, "center")
		}
	}

	e.noGlow()
}

// ── Pause overlay ─────────────────────────────────────────────────────────────

func (e *Engine) renderPauseOverlay() {
	e.ctx.Set("fillStyle", "rgba(0,0,0,0.55)")
	e.ctx.Call("fillRect", 0, 0, e.w, e.h)

	color := e.crtColor()
	t := nowMS() / 1000.0
	pulse := 16 + 24*math.Abs(math.Sin(t*1.5))
	e.ctx.Set("shadowBlur", pulse)
	e.ctx.Set("shadowColor", color)
	e.text("PAUSED", e.w/2, e.h/2, 72, "center")
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.4)")
	e.text("ESC / P  —  RESUME", e.w/2, e.h/2+52, 22, "center")
	e.text("Q  —  QUIT TO MENU", e.w/2, e.h/2+80, 22, "center")
	e.noGlow()
}

// ── Score flash ───────────────────────────────────────────────────────────────

func (e *Engine) renderScoreFlash() {
	if e.scoreFlashTimer <= 0 {
		return
	}
	alpha := e.scoreFlashTimer / 0.25 * 0.45
	var color string
	if e.scoreFlashRed {
		color = fmt.Sprintf("rgba(255,40,40,%.3f)", alpha)
	} else {
		color = fmt.Sprintf("rgba(0,255,80,%.3f)", alpha)
	}
	e.ctx.Set("fillStyle", color)
	e.ctx.Call("fillRect", 0, 0, e.w, e.h)
}

// ── Game over ─────────────────────────────────────────────────────────────────

func (e *Engine) renderGameOver() {
	e.clear()

	if e.twoPlayer {
		e.renderGameOver2P()
	} else {
		e.renderGameOver1P()
	}
}

func (e *Engine) renderGameOver1P() {
	color := e.crtColor()

	e.glow(50)
	e.text("GAME OVER", e.w/2, e.h*0.2, 72, "center")
	e.noGlow()

	e.glow(18)
	e.text(fmt.Sprintf("YOUR SCORE: %d", e.score), e.w/2, e.h*0.36, 36, "center")
	e.text(fmt.Sprintf("BEST STREAK: %d", e.bestStreak), e.w/2, e.h*0.46, 28, "center")
	e.noGlow()

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
	e.ctx.Call("fillRect", e.w/2-90, e.h*0.57, 180, 44)
	e.ctx.Set("strokeStyle", color)
	e.ctx.Set("lineWidth", 1.5)
	e.ctx.Call("strokeRect", e.w/2-90, e.h*0.57, 180, 44)

	nick := ""
	for i := 0; i < e.nickLen; i++ {
		nick += string(e.pendingNick[i])
	}
	cursor := ""
	if e.nickLen < 3 && int(nowMS()/500)%2 == 0 {
		cursor = "_"
	}
	e.glow(10)
	e.text(nick+cursor, e.w/2, e.h*0.57+22, 36, "center")
	e.noGlow()

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.45)")
	e.text("ENTER YOUR 3-LETTER NICK", e.w/2, e.h*0.57-14, 18, "center")

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
	e.text("[ENTER] SAVE   [ESC] SKIP", e.w/2, e.h-24, 18, "center")
}

func (e *Engine) renderGameOver2P() {
	color := e.crtColor()

	e.glow(50)
	e.text("GAME OVER", e.w/2, e.h*0.12, 72, "center")
	e.noGlow()

	// Scores side by side
	e.glow(18)
	e.text(fmt.Sprintf("P1 (BALL): %d", e.score), e.w/4, e.h*0.26, 30, "center")
	e.text(fmt.Sprintf("P2 (PADDLE): %d", e.score2), 3*e.w/4, e.h*0.26, 30, "center")
	e.noGlow()

	// Winner announcement
	winnerLabel := "DRAW!"
	if e.score > e.score2 {
		winnerLabel = "P1 WINS!"
	} else if e.score2 > e.score {
		winnerLabel = "P2 WINS!"
	}
	e.glow(30)
	e.text(winnerLabel, e.w/2, e.h*0.38, 40, "center")
	e.noGlow()

	// Nick entry — phase 0 = P1, phase 1 = P2
	phase0Active := e.nickPhase == 0
	phase1Active := e.nickPhase == 1

	// P1 nick box
	e.renderNickBox(e.w/4, e.h*0.52, "P1 NICK", e.pendingNick, e.nickLen, phase0Active)
	// P2 nick box
	e.renderNickBox(3*e.w/4, e.h*0.52, "P2 NICK", e.pendingNick2, e.nickLen2, phase1Active)

	_ = color
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
	if phase0Active {
		e.text("P1: TYPE YOUR NICK   [ENTER] CONFIRM   [ESC] SKIP ALL", e.w/2, e.h-24, 18, "center")
	} else {
		e.text("P2: TYPE YOUR NICK   [ENTER] SAVE   [ESC] SKIP P2", e.w/2, e.h-24, 18, "center")
	}
}

func (e *Engine) renderNickBox(cx, y float64, label string, nick [3]rune, length int, active bool) {
	color := e.crtColor()
	boxW, boxH := 160.0, 44.0

	if active {
		e.ctx.Set("fillStyle", "rgba(255,255,255,0.15)")
	} else {
		e.ctx.Set("fillStyle", "rgba(255,255,255,0.05)")
	}
	e.ctx.Call("fillRect", cx-boxW/2, y, boxW, boxH)

	if active {
		e.ctx.Set("strokeStyle", color)
	} else {
		e.ctx.Set("strokeStyle", "rgba(255,255,255,0.25)")
	}
	e.ctx.Set("lineWidth", 1.5)
	e.ctx.Call("strokeRect", cx-boxW/2, y, boxW, boxH)

	nickStr := ""
	for i := 0; i < length; i++ {
		nickStr += string(nick[i])
	}
	cursor := ""
	if active && length < 3 && int(nowMS()/500)%2 == 0 {
		cursor = "_"
	}
	if active {
		e.glow(10)
	} else {
		e.noGlow()
	}
	e.text(nickStr+cursor, cx, y+22, 36, "center")
	e.noGlow()

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.45)")
	e.text(label, cx, y-14, 18, "center")
}

// ── Scoreboard ────────────────────────────────────────────────────────────────

func (e *Engine) renderScoreboard() {
	e.clear()
	color := e.crtColor()

	e.glow(35)
	e.text("SCOREBOARD", e.w/2, 36, 48, "center")
	e.noGlow()

	// Tab bar
	tabs := []string{"ALL", "EASY", "MEDIUM", "HARD", "2P"}
	tabW := 100.0
	startX := e.w/2 - float64(len(tabs))*tabW/2
	for i, tab := range tabs {
		tx := startX + float64(i)*tabW
		if i == e.scoreTab {
			e.ctx.Set("fillStyle", color)
			e.ctx.Call("fillRect", tx, 60, tabW-4, 28)
			e.ctx.Set("fillStyle", "#0b0c0b")
		} else {
			e.ctx.Set("fillStyle", "rgba(255,255,255,0.1)")
			e.ctx.Call("fillRect", tx, 60, tabW-4, 28)
			e.ctx.Set("fillStyle", color)
		}
		e.ctx.Set("font", "20px VT323, monospace")
		e.ctx.Set("textAlign", "center")
		e.ctx.Set("textBaseline", "middle")
		e.ctx.Call("fillText", tab, tx+tabW/2-2, 74)
	}

	// Loading / error
	if e.scoreLoading {
		e.glow(8)
		e.text("LOADING...", e.w/2, e.h/2, 32, "center")
		e.noGlow()
	} else if e.scoreError != "" {
		e.ctx.Set("fillStyle", "#ff4444")
		e.text("ERROR: "+e.scoreError, e.w/2, e.h/2, 22, "center")
	} else {
		e.renderScoreTable()
	}

	e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
	e.text("[ESC/R] MENU   [←/→] FILTER", e.w/2, e.h-24, 18, "center")
}

func (e *Engine) renderScoreTable() {
	color := e.crtColor()
	cols := []struct{ label, align string }{
		{"#", "right"},
		{"NICK", "left"},
		{"SCORE", "right"},
		{"LEVEL", "left"},
		{"DATE", "left"},
	}
	xPositions := []float64{40, 70, 220, 310, 420}
	rowH := 28.0
	startY := 106.0

	// Header
	e.ctx.Set("fillStyle", "rgba(255,255,255,0.45)")
	for i, col := range cols {
		e.ctx.Set("font", "20px VT323, monospace")
		e.ctx.Set("textAlign", col.align)
		e.ctx.Set("textBaseline", "middle")
		e.ctx.Call("fillText", col.label, xPositions[i], startY)
	}

	// Separator
	e.ctx.Set("strokeStyle", "rgba(255,255,255,0.15)")
	e.ctx.Set("lineWidth", 1)
	e.ctx.Call("beginPath")
	e.ctx.Call("moveTo", 30, startY+14)
	e.ctx.Call("lineTo", e.w-30, startY+14)
	e.ctx.Call("stroke")

	entries := e.scoreEntries
	for i, entry := range entries {
		if i >= 10 {
			break
		}
		y := startY + rowH*float64(i+1) + 10

		// Highlight player's last submitted score
		if entry.Nick == e.lastSubmittedNick && entry.Score == e.lastSubmittedScore {
			e.ctx.Set("fillStyle", "rgba(255,255,255,0.08)")
			e.ctx.Call("fillRect", 30, y-12, e.w-60, rowH-2)
		}

		e.ctx.Set("fillStyle", color)
		e.ctx.Set("font", "20px VT323, monospace")
		e.ctx.Set("textBaseline", "middle")

		row := []struct {
			text  string
			align string
			x     float64
		}{
			{fmt.Sprintf("%d.", i+1), "right", xPositions[0]},
			{entry.Nick, "left", xPositions[1]},
			{fmt.Sprintf("%d", entry.Score), "right", xPositions[2]},
			{entry.Level, "left", xPositions[3]},
			{entry.Timestamp[:10], "left", xPositions[4]},
		}
		for _, cell := range row {
			e.ctx.Set("textAlign", cell.align)
			e.ctx.Call("fillText", cell.text, cell.x, y)
		}
	}

	if len(entries) == 0 {
		e.ctx.Set("fillStyle", "rgba(255,255,255,0.35)")
		e.text("NO SCORES YET", e.w/2, e.h/2, 28, "center")
	}
}

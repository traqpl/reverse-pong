# Reverse Pong TUI — how the code works

> Documentation for the terminal version (`cmd/tui/`).
> Read top to bottom — each section builds on the previous one.

---

## Before you start — what is this game?

Reverse pong. **You are the ball** (`●`). The AI controls the paddle on the left side.
Your goal is to *slip past* the paddle — if the ball slides by, you score a point.
If the paddle catches it, the ball bounces back and you lose your streak.

```
 SCORE: 5      TIME:  42s   STREAK: 2 (best 3)  [MEDIUM]
──────────────────────────────────────────────────────
│█▌                                               │
│█▌              ●                               │
│                                                 │
│                                                 │
──────────────────────────────────────────────────────
```

---

## Files

```
cmd/tui/
  main.go        — game engine: loop, physics, rendering
  audio.go       — PCM sound generation in Go
  scoreboard.go  — Hall of Fame: save, load, draw
```

---

## `main.go` — the heart of the game

### Size constants and integer scaling

The game has a **fixed logical space** (44×23) and renders it in any terminal size.

```go
const (
    logicCols = 44   // game field width in logical cells
    logicRows = 23   // game field height in logical cells
    hudRows   = 2    // row 0: HUD, row 1: separator ─────
    botWall   = 1    // row for the bottom wall ─────
)
```

**Why these numbers?** A terminal is a grid of characters, not pixels.
A terminal cell has roughly a 1:2 aspect ratio (width:height).
A 44×23 field at that ratio looks similar to the web canvas of 800×480.

**Integer scaling** — `calcScale()` computes how many terminal cells correspond to one logical cell:

```go
func (e *Engine) calcScale() {
    e.termW, e.termH = e.scr.Size()
    avail := e.termH - hudRows - botWall
    byW := e.termW / logicCols   // how many times it fits horizontally
    byH := avail / logicRows     // how many times it fits vertically
    e.rScale = min(byW, byH)     // pick the smaller (preserve aspect ratio)
    if e.rScale < 1 { e.rScale = 1 }

    fw := logicCols * e.rScale
    fh := logicRows * e.rScale
    e.fOffX = (e.termW - fw) / 2       // center horizontally
    e.fOffY = hudRows + (avail-fh)/2   // center vertically (below HUD)
}
```

Physics always runs in 44×23 space. Rendering multiplies positions by `rScale` and adds `fOffX/fOffY`. At `rScale=1` the game looks like a classic 59×26 terminal; at `rScale=3` each cell is a 3×3 terminal block.

`calcScale()` is called at startup and every time the terminal is resized (`EventResize`).

---

### Game states — finite state machine

The game is a **state machine**. At any moment it is in exactly one state:

```go
type GameState int

const (
    StateMenu        // start screen with HOF ticker
    StateCountdown   // 3… 2… 1… GO!
    StatePlaying     // the game itself
    StatePaused      // pause
    StateNickInput   // entering initials after game over
    StateScoreboard  // Hall of Fame
)
```

State transitions happen **only via keyboard events or time expiry**:

```
Menu ──Enter──► Countdown ──auto──► Playing
                                       │
                              Esc ◄────┤────► Paused
                                       │
                           timeLeft=0 ─┴──► NickInput ──Enter──► Scoreboard
                                                 └──Esc──► Scoreboard
                                                 └──R──► Menu
```

---

### Data structures

#### `Ball` — the ball (= the player)

```go
type Ball struct{ X, Y, VX, VY float64 }
```

- `X, Y` — position in **terminal cells** (float, rounded when drawing)
- `VX, VY` — velocity in **cells per second**
- The player only changes `Y` via arrow keys; `VX` and `VY` run automatically

#### `Paddle` — the AI paddle

```go
type Paddle struct {
    Y, H        float64   // top edge position and height
    jitterTimer float64   // when to roll the next jitter (easy)
    jitterOff   float64   // current jitter offset
    errorTimer  float64   // when to roll the next error (medium/hard)
    errorOff    float64   // current error offset
    targetY     float64   // target the paddle is moving toward
}
```

#### `Engine` — everything together

```go
type Engine struct {
    scr    tcell.Screen   // terminal screen
    state  GameState
    level  int            // 1=easy, 2=medium, 3=hard
    ball   Ball
    paddle Paddle
    score, streak, bestStreak int
    timeLeft float64
    lastUp, lastDown time.Time   // time of last arrow key press
    nickBuf [3]rune              // nick input buffer
    nickLen int
    sb      *scoreboardState
}
```

---

### Main loop

```go
ticker := time.NewTicker(time.Second / 60)  // 60 FPS

for running {
    <-ticker.C            // wait for the next frame

    dt = now - lastTick   // elapsed time (in seconds)

    // 1. drain all events from the buffer
    for { ev := <-evCh; e.processEvent(ev) }

    // 2. update game state
    e.update(dt)

    // 3. draw the frame
    e.draw()
}
```

Why `dt` instead of a fixed step? The computer can be slow, the window inactive, or GC can stall the program. `dt` ensures physics behaves identically regardless of momentary delays. A maximum `dt = 0.05s` prevents ball "teleportation" after a long pause.

---

### Smooth player movement — why not `keydown/keyup`?

`tcell` **has no keyup event** — the terminal doesn't support it. The classic approach (`upHeld = true` on keydown, `upHeld = false` on keyup) doesn't work.

Solution: **timestamp of last key press**.

```go
// in processEvent:
if ev.Key() == tcell.KeyUp {
    e.lastUp = time.Now()
}

// in updatePlaying:
if time.Since(e.lastUp) < holdThreshold {  // 90ms
    e.ball.Y -= playerSpeed * dt
}
```

The OS sends key-repeat (successive events) while the key is held, typically every ~30ms. If less than 90ms has passed since the last event — the key is "held". This gives smooth movement at 60 FPS.

---

### Ball physics

```go
// player movement
if time.Since(e.lastUp) < holdThreshold { e.ball.Y -= playerSpeed * dt }
if time.Since(e.lastDown) < holdThreshold { e.ball.Y += playerSpeed * dt }

// autonomous movement
e.ball.X += e.ball.VX * dt
e.ball.Y += e.ball.VY * dt

// wall bounces (top/bottom/right)
if e.ball.Y < 0       { e.ball.VY =  math.Abs(e.ball.VY); soundBounce() }
if e.ball.Y >= fieldRows { e.ball.VY = -math.Abs(e.ball.VY); soundBounce() }
if e.ball.X >= fieldCols-1 { e.ball.VX = -math.Abs(e.ball.VX); soundBounce() }
```

Both movements (player and physics) are added to the position in the same frame — the player "layers" their movement on top of the ball's autonomous flight.

---

### Collision detection with the paddle

```go
if e.ball.X < float64(paddleWidth) {
    if e.ball.Y >= e.paddle.Y && e.ball.Y <= e.paddle.Y+e.paddle.H {
        // paddle caught — bounce
        e.ball.VX = math.Abs(e.ball.VX)       // fly right
        e.ball.VY += rand * 0.15               // slight angle nudge
        e.streak = 0
    } else {
        // paddle missed — point!
        e.score += points * streakMult
        // ball resets to center, slightly faster
        speed *= ballSpeedupRate  // ×1.05
    }
}
```

---

### Paddle AI

Three levels:

#### Easy — random jitter
Every `jitterInterval` seconds it rolls an offset `±jitterRange`. Aims at `ball.Y + jitterOffset`. Never predicts — just tracks the ball with random error.

#### Medium — prediction + errors
When the ball is moving left (`VX < 0`), calls `predictY()` — simulates the ball's path to the left wall accounting for bounces. With probability `errChance` (30%) adds a random error to the target.

#### Hard — same but rarer and smaller errors
`errChance = 8%`, `errRange` small, `deadZone` near zero — the paddle is almost perfect.

#### `predictY` — path simulator

```go
func predictY(ball Ball, targetX, fw, fh float64) float64 {
    x, y, vx, vy := ball.X, ball.Y, ball.VX, ball.VY
    for i := 0; i < 5000; i++ {
        x += vx * 0.01  // 10ms step
        y += vy * 0.01
        // handle wall bounces...
        if vx < 0 && x <= targetX { return y }  // reached target
    }
    return y
}
```

Simulates the ball's flight in 10ms steps and returns Y at the target. The paddle aims at that point instead of the ball's current position — which is why the AI seems to "read" your moves.

---

### Rendering

`tcell` lets you set any character at any terminal cell:

```go
e.scr.SetContent(col, row, '●', nil, styleBall)
```

The ball is **a single character** `●` at its rounded position:

```go
bx := int(math.Round(e.ball.X))
by := int(math.Round(e.ball.Y)) + hudRows
e.scr.SetContent(bx, by, '●', nil, styleBall)
```

The paddle is two characters wide (`█▌`) drawn in a loop over rows:

```go
for row := paddleTop; row <= paddleBot; row++ {
    e.scr.SetContent(0, row+hudRows, '█', nil, stylePaddle)
    e.scr.SetContent(1, row+hudRows, '▌', nil, stylePaddle)
}
```

Finally `e.scr.Show()` flushes changes to the terminal **in one operation** (double buffering is built into tcell — no flicker).

---

## `audio.go` — sounds without files

The `oto/v3` library opens the audio device and plays raw PCM bytes.
All sounds are **generated mathematically** on the fly.

### Sine wave with fade-out

```go
func tone(freq, freqEnd float64, dur time.Duration, vol float64) []byte {
    n := sampleRate * dur.Seconds()   // number of samples
    for i := 0; i < n; i++ {
        t    := float64(i) / sampleRate
        f    := lerp(freq, freqEnd, i/n)      // frequency sweep
        fade := sqrt(1 - i/n)                  // amplitude fade-out
        s    := sin(2π * f * t) * vol * fade
        // convert to signed int16 little-endian...
    }
}
```

**PCM** = samples of the sound wave amplitude, 44100 per second, 16-bit signed.
Each sample is 2 bytes (little-endian). A sine gives a clean tone; `freqEnd != 0` makes a sweep.

### In-game sounds

| Function | Sound | When |
|---|---|---|
| `soundBounce()` | short 880Hz | wall bounce |
| `soundHit()` | falling sweep 220→110Hz | paddle catches ball |
| `soundScore()` | 3 rising tones 660→880→1100Hz | player scores |
| `soundCountdown()` | beep 440Hz | each countdown digit |
| `soundGo()` | chord 660+880Hz | "GO!" |
| `soundGameOver()` | falling 440→330→220Hz | game over |

Everything plays in separate goroutines (`go playBuf(...)`) so it doesn't block the game loop.

---

## `scoreboard.go` — Hall of Fame

### Where is the data?

```
~/.config/reverse-pong-tui/hof.json   (macOS/Linux)
```

JSON format, max 100 entries, sorted descending by score.

### Entry lifecycle

```
game over
    → StateNickInput  (player types 3 letters)
    → sb.add(nick, score, level)
        → append to s.all
        → sort descending
        → trim to 100
        → saveHOF() → write JSON
    → StateScoreboard (own score highlighted in green)
```

### Filtering by level

`visible()` filters `s.all` by the active tab:

```go
func (s *scoreboardState) visible() []hofEntry {
    if tabLevels[s.tab] == "" { return s.all }   // ALL
    // filter by level...
}
```

Tabs: `ALL / EASY / MEDIUM / HARD` — switched with `←→`.

### Menu ticker

```go
func (e *Engine) drawTicker() {
    runes := []rune(e.sb.tickerText())
    offset := int(time.Now().UnixMilli()/80) % len(runes)
    for x := 0; x < winCols; x++ {
        idx := (offset + x) % len(runes)
        e.scr.SetContent(x, 0, runes[idx], nil, styleTitle)
    }
}
```

`offset` grows over time (`UnixMilli/80` = ~12 steps/second) and "slides the window" over the text. Modulo the text length causes it to loop.

---

## How to run

```bash
make play        # compile + open a new Ghostty window 59×26
make tui         # compile only → ./reverse-pong-tui
```

Keys:

| State | Key | Action |
|---|---|---|
| Menu | `↑↓` | change level |
| Menu | `1` `2` `3` | select level |
| Menu | `ENTER` / any | start |
| Menu | `S` | Hall of Fame |
| Game | `↑↓` | move ball |
| Game | `ESC` / `P` | pause |
| Pause | `ESC` / `P` | resume |
| Pause | `Q` | menu |
| Nick | letters | type initials |
| Nick | `Backspace` | delete |
| Nick | `ENTER` | save (only with 3 letters) |
| Nick | `ESC` | skip save |
| HOF | `←→` | switch tab |
| HOF | `↑↓` | scroll |
| HOF | `R` / `ESC` | menu |
| Anywhere | `Ctrl+C` | quit |

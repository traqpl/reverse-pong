# Reverse Pong Web — how the code works

> Documentation for the web version (`game/` + `server/`).
> Read top to bottom — each section builds on the previous one.

---

## Before you start — what is this game?

Reverse pong. **You are the ball**. The AI (or Player 2) controls the paddle on the left.
Your goal is to slip past the paddle — if the ball passes it, you score a point.
If the paddle catches the ball, it bounces back; in 2P mode the paddle player scores instead.

```
┌────────────────────────────────────────────────────────────┐
│ SCORE: 5              MEDIUM                  TIME: 0:42   │
│                                                            │
│ ██          ●                                           │  │
│ ██                                                      │  │
│ ██                                                      │  │
│                                                            │
│                      STREAK 3  ×1.5                        │
└────────────────────────────────────────────────────────────┘
```

---

## Architecture overview

```
Browser
  index.html          — page shell, theme switcher, scoreboard ticker, WASM loader
  audio.js            — Web Audio API sound effects + background music
  touch.js            — touch input → window.touchInput (read by WASM)
  wasm_exec.js        — Go WASM runtime bridge (official, from Go toolchain)
  game.wasm           — compiled game logic (Go → WASM)
  style.css           — CRT monitor visual effect

Go server (server/)
  main.go             — HTTP server, API handlers
  config.go           — config loading (config.yaml via Viper)
  scoreboard.go       — SQLite score store
```

The game logic lives entirely in `game.wasm`. The server does three things: serves static files, exposes a config API, and persists scores.

---

## Files — game logic (`game/`)

```
game/
  main.go        — WASM entry point, requestAnimationFrame loop
  engine.go      — Engine struct, state machine, game states
  update.go      — per-frame update logic (physics, collisions, scoring)
  ball.go        — Ball struct, wall physics, drawing
  paddle.go      — Paddle struct, AI levels (easy/medium/hard), 2P input
  input.go       — keyboard + gamepad + touch input handling
  render.go      — Canvas 2D rendering (menu, HUD, CRT effects)
  constants.go   — GameConfig, LevelSettings, config fetch from /api/config
  scoreboard.go  — fetch/post scores to /api/scores, nick entry logic
```

All files have the build tag `//go:build js && wasm` — they only compile for the browser target.

---

## `game/main.go` — WASM entry point

```go
func main() {
    fetchConfig()          // synchronous XHR to /api/config before anything else

    canvas := js.Global().Get("document").Call("getElementById", "gameCanvas")
    engine = NewEngine(canvas)
    engine.registerInput()
    go engine.fetchStats()

    var loop js.Func
    loop = js.FuncOf(func(_ js.Value, args []js.Value) any {
        now := args[0].Float()          // DOMHighResTimeStamp from rAF
        dt := (now - lastTime) / 1000.0
        if dt > 0.1 { dt = 0.1 }       // cap: prevents spiral of death on tab switch

        engine.Update(dt)
        engine.Render()

        js.Global().Call("requestAnimationFrame", loop)
        return nil
    })
    js.Global().Call("requestAnimationFrame", loop)

    select {} // block forever — WASM must not exit
}
```

The game runs on `requestAnimationFrame` — the browser calls the loop at the display refresh rate (typically 60 Hz). `dt` is the elapsed time in seconds since the previous frame. Capping at 0.1s prevents the ball from tunnelling through walls after a long tab pause.

**Config fetch** — at startup the WASM makes a synchronous XHR to `/api/config`. This blocks until the server responds and populates the global `cfg` struct (ball speeds, paddle speeds, timing). If the server is unreachable, `defaultConfig()` is used as a fallback.

---

## `game/engine.go` — Engine and state machine

### `Engine` struct (key fields)

```go
type Engine struct {
    canvas js.Value
    ctx    js.Value          // Canvas 2D context (Display P3 if supported)
    w, h   float64           // canvas size: 800 × 500 px

    state     GameState
    twoPlayer bool           // true = P2 controls paddle with W/S

    ball   Ball
    paddle Paddle
    level  AILevel           // Easy / Medium / Hard

    score, streak, bestStreak int
    score2, streak2, bestStreak2 int
    timeLeft float64         // seconds remaining

    scoreFlashTimer float64  // duration of score flash overlay
    scoreFlashRed   bool     // green = P1 scored, red = P2 scored

    keys map[string]bool     // currently held keyboard keys
    wallCooldownSide int     // blocks player input toward a wall after bounce
}
```

### State machine

```go
type GameState int

const (
    StateMenu       // main menu with difficulty/mode selection
    StateCountdown  // 3 → 2 → 1 → GO!
    StatePlaying    // active game
    StatePaused     // game frozen, overlay shown
    StateHit        // (unused freeze state, reserved)
    StateGameOver   // nick entry screen
    StateScoreboard // fetched scoreboard table
)
```

Transitions:

```
Menu ──Enter──► Countdown ──auto──► Playing
                                       │
                              Esc ◄────┤────► Paused ──Esc/P──► Playing
                                       │              └──Q──► Menu
                           timeLeft=0 ─┴──► GameOver ──Enter──► Scoreboard
                                                 └──Esc──► Scoreboard
```

---

## `game/ball.go` — Ball physics

```go
type Ball struct {
    X, Y   float64
    VX, VY float64
    R      float64    // radius = 7px
    LastWallHit int   // -1 = top, +1 = bottom, 0 = none (set per frame)
}
```

Each frame `Ball.Update()` runs:

1. `X += VX * dt`, `Y += VY * dt`
2. Top/bottom wall check — reflect VY, clamp position, call `enforceMinAngle()`
3. Right wall check — reflect VX

**`enforceMinAngle()`** — after a wall bounce, ensures `|VY| >= |VX| * 0.20`. This prevents the ball from skimming along a wall at a near-zero angle, which would make the game unresponsive.

The ball is drawn as a glowing circle using a radial gradient (white center → theme color → transparent) with `shadowBlur` for the bloom effect. The game requests a **Display P3** canvas context for wider color gamut on supported displays (Chrome + Apple Silicon), with a fallback to standard sRGB.

---

## `game/update.go` — per-frame game logic

### Playing tick

```go
func (e *Engine) updatePlaying(dt float64) {
    e.applyMovementInput(dt)       // player 1 moves ball, player 2 moves paddle
    e.ball.Update(dt, e.w, e.h)   // physics + wall bounces

    // wall cooldown: block player from pushing ball back toward the wall
    // it just hit, until the ball is 1/4 screen height away
    if e.ball.LastWallHit != 0 { e.wallCooldownSide = e.ball.LastWallHit }
    if e.wallCooldownSide == -1 && e.ball.Y > e.h/4 { e.wallCooldownSide = 0 }
    if e.wallCooldownSide == 1 && e.ball.Y < 3*e.h/4 { e.wallCooldownSide = 0 }

    if !e.twoPlayer { e.paddle.Update(e.ball, dt, e.h) }  // AI

    // left edge: did the paddle catch the ball?
    if e.ball.X-e.ball.R <= 0 {
        e.ball.X = e.ball.R
        e.ball.VX = -e.ball.VX  // bounce right

        if e.paddle.Hits(e.ball) {
            e.streak = 0
            if e.twoPlayer { e.onScorePaddle() }  // P2 scores
        } else {
            e.onScore()                            // P1 scores
        }
        return
    }
    e.timeLeft -= dt
    if e.timeLeft <= 0 { /* game over */ }
}
```

**Wall cooldown** — after the ball hits a top or bottom wall, the player cannot push the ball back into that wall. This prevents "grinding" the ball along a wall as an exploit. The cooldown lifts once the ball is 25% of the screen height away from the wall.

### Scoring and ball speedup

```go
func (e *Engine) onScore() {
    e.streak++
    pts := levelPoints(e.level)          // 1 / 2 / 3 for Easy/Medium/Hard
    mult := streakMultiplier(e.streak)   // ×1.5 at streak 3, ×2.0 at streak 5
    e.score += int(float64(pts) * mult)

    // speed up ball on each score
    speed := e.ball.VX * cfg.Game.BallSpeedupRate  // ×1.02 per point
    e.ball.VX = speed
    e.ball.VY = ±speed * 0.5             // keep VY proportional
}
```

The ball speeds up by `BallSpeedupRate` (default 1.02×) on every scored point. Over a 60-second match at ~1 point/2s the ball can reach ~1.7× its starting speed.

---

## `game/paddle.go` — Paddle AI

```go
type Paddle struct {
    X, Y  float64
    W, H  float64    // width=12, height=80 (1P) / 130 (2P)
    Speed float64    // from LevelSettings.PaddleSpeed
    Level AILevel
    lc    LevelSettings  // config snapshot
    // AI state
    targetY, jitterTimer, jitterOffset float64
    errorTimer, errorOffset            float64
}
```

### AI levels

#### Easy — jitter tracking
Every `JitterInterval` seconds the AI rolls a random `±JitterRange` pixel offset.
It tracks `ball.Y + jitterOffset` directly — no prediction.
Achieves difficulty through positional noise alone.

#### Medium — prediction + random errors
When `ball.VX < 0` (ball moving toward paddle), calls `predictBallY()` to simulate the ball's exact trajectory to the paddle's X position. With probability `ErrorChance` (30%), applies a random `±ErrorRange` pixel miss offset to the predicted target.

#### Hard — near-perfect prediction
Same as Medium but `ErrorChance = 8%`, `ErrorRange` small, `DeadZone = 3px`.
The paddle almost always positions perfectly; occasional small errors are its only weakness.

#### `predictBallY` — trajectory simulator

```go
func predictBallY(ball Ball, targetX, canvasH float64) float64 {
    x, y, vx, vy := ball.X, ball.Y, ball.VX, ball.VY
    const step = 0.004  // 4ms steps
    const maxIter = 600 // covers up to 2.4 seconds of flight

    for i := 0; i < maxIter; i++ {
        x += vx * step
        y += vy * step
        // reflect at top/bottom walls
        // reflect at right wall
        if vx < 0 && x <= targetX { return y }
    }
    return y
}
```

Simulates the ball's path in 4ms steps accounting for all wall bounces. The AI aims at the predicted landing Y instead of the current ball Y — this makes it appear to "read" the player's moves even when the ball is far away.

### 2P mode
In 2P mode the AI is disabled (`twoPlayer = true`). Player 2 moves the paddle with `W`/`S` keys at the same `PaddleSpeed` the AI would use for the selected level.

---

## `game/input.go` — input handling

Player 1 (ball) input is processed by `applyMovementInput()`:

```go
func (e *Engine) applyMovementInput(dt float64) {
    speed := cfg.Game.PlayerSpeed   // px/s, from config

    dy := 0.0
    if e.keys["ArrowUp"]   { dy -= speed * dt }
    if e.keys["ArrowDown"] { dy += speed * dt }

    // gamepad left stick / D-pad
    dy += e.gamepadDY(dt)

    // touch swipe (from touch.js via window.touchInput)
    if touchInput.active { dy += touchInput.deltaY * dt }

    // wall cooldown: block input toward recently-hit wall
    if e.wallCooldownSide == -1 && dy < 0 { dy = 0 }
    if e.wallCooldownSide == 1 && dy > 0  { dy = 0 }

    e.ball.Y += dy
    // clamp to canvas
}
```

Key state is tracked in `e.keys map[string]bool`. `keydown` sets `keys[key] = true`; `keyup` deletes the entry. All single-character keys are normalized to lowercase so Shift/CapsLock doesn't create duplicate entries.

The window `blur` event and `visibilitychange` both clear `e.keys` to prevent ghost key presses when the window loses focus.

---

## `game/render.go` — Canvas 2D rendering

### CRT visual style

The game uses three theme colors (amber / green / cyan), each defined as a `color(display-p3 …)` value that slightly exceeds sRGB for vivid bloom on P3 displays:

```go
func (e *Engine) crtColor() string {
    switch js.Global().Get("crtTheme").String() {
    case "theme-green": return "color(display-p3 0.50 1.00 0.50)"
    case "theme-cyan":  return "color(display-p3 0.48 0.92 1.00)"
    default:            return "color(display-p3 1.00 0.78 0.38)"  // amber
    }
}
```

The active theme is set by the `<select>` in `index.html` and exposed to WASM as `window.crtTheme`.

**Scanlines** — every frame, thin horizontal lines are drawn every 4px at 18% opacity to mimic a CRT screen:

```go
for y := 2.0; y < e.h; y += 4 {
    ctx.Call("moveTo", 0, y)
    ctx.Call("lineTo", e.w, y)
}
```

**Glow** — `shadowBlur` + `shadowColor` is set before drawing any element to produce the phosphor bloom effect. `noGlow()` resets `shadowBlur = 0` after each element to avoid bleed.

### HUD layout

| Mode | Left | Centre | Right |
|------|------|--------|-------|
| 1P | `SCORE: N` | level name | `TIME: M:SS` |
| 2P | `P1 N` | timer | `N P2` |

The time display turns red when `timeLeft < 10s`.
Streak multiplier (×1.5 or ×2.0) appears at the bottom when streak ≥ 3.

### Score flash
On every score, a full-canvas color overlay fades over 0.25s:
- **Green** (`rgba(0,255,80,α)`) — P1 scored (ball passed paddle)
- **Red** (`rgba(255,40,40,α)`) — P2 scored (paddle caught ball)

---

## `audio.js` — Web Audio API sounds

All sound effects are generated via the Web Audio API — no audio files needed for SFX.

```js
function beep({ type, freq, freqEnd, gain, duration }) {
    const osc = ctx.createOscillator();
    const env = ctx.createGain();
    osc.connect(env); env.connect(ctx.destination);

    osc.frequency.setValueAtTime(freq, ctx.currentTime);
    if (freqEnd !== null)
        osc.frequency.linearRampToValueAtTime(freqEnd, ctx.currentTime + duration);

    // fast attack (5ms), linear decay to 0
    env.gain.linearRampToValueAtTime(gain, ctx.currentTime + 0.005);
    env.gain.linearRampToValueAtTime(0, ctx.currentTime + duration);
    // ...
}
```

| Method | Sound | When |
|--------|-------|------|
| `bounce()` | sine 440Hz, 50ms | wall or left-edge bounce |
| `score()` | sine sweep 300→800Hz, 150ms | player scores |
| `hit()` | sawtooth 80Hz, 200ms | paddle catches ball |
| `countdown()` | square 600Hz, 30ms | each countdown digit |
| `go()` | sine chord 523+659+784Hz | "GO!" |
| `gameOver()` | sine sweep 400→80Hz, 500ms | game over |

**Background music** — looped `music.mp3`, played at 0.35 volume in the menu and 0.22 during play (quieter so SFX cuts through). Browsers require a user gesture before audio can start; `audio.js` starts music on the first `keydown`, `mousedown`, or `touchstart`.

WASM calls audio by name via `window.audioPlay`:

```go
func callAudio(method string) {
    ap := js.Global().Get("audioPlay")
    fn := ap.Get(method)
    fn.Call("call", ap)
}
```

---

## `touch.js` — touch input

Exposes `window.touchInput = { active: bool, deltaY: number }` for WASM to read.

On `touchmove`, computes instantaneous vertical velocity in px/s from the touch position delta:

```js
const velocity = (t.clientY - lastY) / dt;
window.touchInput.deltaY = Math.max(-600, Math.min(600, velocity));
```

Velocity is clamped to ±600 px/s. WASM multiplies `touchInput.deltaY * dt` to get the per-frame position delta — the same formula as keyboard input.

After `touchend`, a 100ms timer clears `touchInput` so the ball doesn't drift after the finger lifts.

---

## Server (`server/`)

### HTTP endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | static files (HTML, CSS, JS, WASM) embedded in binary |
| `GET` | `/api/config` | JSON game config (loaded from `config.yaml`) |
| `GET` | `/api/scores?level=&n=` | top-N scores, optional level filter (`easy`/`medium`/`hard`/`2p`) |
| `POST` | `/api/scores` | submit score (1P or 2P) |
| `GET` | `/api/stats` | aggregated match counts by mode and level |

All static files are embedded via `//go:embed web` so the server binary is self-contained.

The server listens on **HTTPS only** (TLS). The certificate is embedded in the binary via `internal/certdata` (encrypted at rest, decrypted at startup).

### Score submission — 1P

```json
POST /api/scores
{ "nick": "ABC", "score": 42, "level": "medium" }
```

Validation: nick must match `^[A-Za-z]{3}$`, score in `0–9999`, level one of `easy/medium/hard`.
Rate limit: one submission per IP per minute.

### Score submission — 2P

```json
POST /api/scores
{
  "p1_nick": "AAA", "p1_score": 30,
  "p2_nick": "BBB", "p2_score": 18,
  "p2_level": "hard"
}
```

Both entries are written in a single SQLite transaction. Winner (`p1`/`p2`/`draw`) is computed server-side and recorded in the `matches` table. If P2 skips their nick, `p2_nick` is `"---"` and only P1's score row is inserted.

### `scoreboard.go` — SQLite store

Two tables:

```sql
scores  (id, nick, score, level, timestamp)   -- individual score entries
matches (id, mode, level, winner, timestamp)  -- match result log
```

`ScoreStore` wraps a `*sql.DB` with:
- `sync.Mutex` for write serialization (SQLite single-writer)
- `lastIP map[string]time.Time` for rate limiting (in-memory, resets on restart)

`Top(level, n)` — returns the top-N scores ordered by score descending, optionally filtered by level.

`Stats()` — aggregates match counts from the `matches` table, grouped by mode and level. This powers the statistics shown on the main menu.

### `config.go` — game configuration

Viper loads `config.yaml` from the working directory. All fields have defaults so the server works without a config file.

| Key | Default | Description |
|-----|---------|-------------|
| `game.duration` | 60 | match length in seconds |
| `game.ball_speedup_rate` | 1.02 | ball speed multiplier per scored point |
| `game.player_speed` | 700 | P1 (ball) movement speed in px/s |
| `levels.easy.ball_speed` | 600 | starting ball speed (px/s) |
| `levels.easy.paddle_speed` | 750 | AI/P2 paddle speed (px/s) |
| `levels.medium.ball_speed` | 800 | — |
| `levels.medium.paddle_speed` | 1000 | — |
| `levels.hard.ball_speed` | 1000 | — |
| `levels.hard.paddle_speed` | 1250 | — |

The full config is served as JSON at `/api/config` and fetched by WASM at startup, so changing `config.yaml` and restarting the server immediately affects the running game for all clients.

---

## How to build and run

```bash
make web     # compile game/ → server/web/game.wasm
make server  # compile server/ → ./reverse-pong-server
make run     # web + server, then launch browser
```

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTPS listen port |
| `DB_PATH` | `$XDG_CONFIG_HOME/reverse-pong/scores.db` | SQLite database path |

The server always uses HTTPS. The embedded certificate is self-signed; browsers will show a security warning on first visit.

---

## Keys

| State | Key | Action |
|-------|-----|--------|
| Menu | `1` `2` `3` | select difficulty |
| Menu | `←` `→` | select difficulty |
| Menu | `Tab` | toggle 1P / 2P mode |
| Menu | `Enter` / `Space` | start game |
| Menu | `S` | open scoreboard |
| Menu | `M` | toggle music |
| Game (P1) | `↑` `↓` | move ball |
| Game (P2) | `W` `S` | move paddle |
| Game | `Esc` | pause |
| Pause | `Esc` / `P` | resume |
| Pause | `Q` | quit to menu |
| Game over | letters | type 3-letter nick |
| Game over | `Backspace` | delete character |
| Game over | `Enter` | save score (requires 3 letters) |
| Game over | `Esc` | skip saving |
| Scoreboard | `←` `→` | switch tab (ALL / EASY / MEDIUM / HARD / 2P) |
| Scoreboard | `Esc` / `R` | back to menu |

Gamepad: left stick Y-axis or D-pad up/down controls P1 (ball).
Touch: vertical swipe controls P1 (ball).

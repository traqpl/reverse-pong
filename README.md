# Reverse Pong

A twist on the classic Pong — you control the ball, not the paddle. The paddle is driven by AI (or a second player). Score points by slipping the ball past the paddle, not by bouncing off it.

Written in Go. Runs in the browser (WebAssembly + Web Audio) and in the terminal (TUI).

## Mechanics

- **Ball** — controlled with arrow keys (or gamepad / touch), bounces off walls
- **Paddle** — on the left side, controlled by AI or P2 (W/S keys)
- **Point** — scored when the ball slips past the paddle (AI misses)
- **Streak** — consecutive points without the paddle catching the ball give a multiplier: ×1.5 at 3, ×2.0 at 5
- **Time** — 60 seconds to score as many points as possible

## Modes

| Mode | Description |
|------|-------------|
| 1P | You control the ball, AI controls the paddle |
| 2P | P1 controls the ball (arrows), P2 controls the paddle (W/S) — both score points |

## Difficulty levels

| Level  | Ball speed | Points |
|--------|------------|--------|
| Easy   | 400 px/s   | 1 pt   |
| Medium | 533 px/s   | 2 pt   |
| Hard   | 667 px/s   | 3 pt   |

## Controls (browser)

| Key | Action |
|-----|--------|
| Arrow Up / Down | Move ball (P1) |
| W / S | Move paddle (P2 in 2P mode) |
| Enter / Space | Start game |
| Tab | Toggle 1P / 2P |
| 1 / 2 / 3 | Select difficulty |
| ← / → | Change difficulty |
| M | Toggle music on / off |
| S | Scoreboard |
| Esc | Pause / back |
| Q | Quit to menu (from pause) |

## Requirements

- Go 1.21+
- ffmpeg (for music compression in `make music` / `make deploy`)
- TLS certificates in `certs/` (`cert.pem`, `key.pem`) — not included in the repository

## Running locally

```bash
# Build WASM + generate certificates + start server
make all
```

Opens https://localhost:8080 automatically.

Or step by step:

```bash
make music     # compress music to 96 kb/s (requires ffmpeg)
make wasm      # compile game to WebAssembly
make certs     # encrypt certificates, generate internal/certdata/encrypted.go
make server    # start HTTPS server
```

## Terminal version (TUI)

```bash
make play      # build and open in Ghostty (220×50)
```

Or manually:

```bash
make tui
./reverse-pong-tui
```

## Deployment

Deploy via SSH to the host named `daemon`:

```bash
make deploy
```

Cross-compiles for Linux/amd64, copies binaries over SCP, kills the old process, and starts the new one via `start-server.sh` (uses `setcap cap_net_bind_service` to bind port 443 without root).

```bash
make logs      # tail -f server logs
```

## Configuration

`config.yaml` — game parameters loaded by the server at startup:

```yaml
game:
  duration: 60               # game duration in seconds
  ball_speedup_rate: 1.02    # ball speed multiplier after each point
  player_speed: 467          # player-controlled ball speed (px/s)

levels:
  easy:
    ball_speed: 400
    paddle_speed: 333
  medium:
    ball_speed: 533
    paddle_speed: 480
  hard:
    ball_speed: 667
    paddle_speed: 640
```

## Project structure

```
game/        — game logic (compiled to WASM, build tag: js && wasm)
server/      — HTTPS server, scoreboard API, static asset serving
server/web/  — frontend (HTML, CSS, JS, game.wasm, music.mp3)
cmd/certgen/ — TLS certificate encryption tool
cmd/tui/     — terminal version entry point
tui/         — TUI renderer (tcell)
internal/    — internal packages (certdata)
music/       — source audio files
config.yaml  — game configuration
```

## Scoreboard

Scores are stored in SQLite (`~/.config/reverse-pong/scores.db` or `$DB_PATH`). API:

- `GET /api/scores` — top 10 across all levels
- `GET /api/scores?level=easy|medium|hard|2p` — filter by level
- `POST /api/scores` — submit a score (rate limit: 1/min per IP)

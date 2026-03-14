# Reverse Pong TUI — jak działa kod

> Dokumentacja wersji terminalowej (`cmd/tui/`).
> Czytaj od góry — każdy rozdział buduje na poprzednim.

---

## Zanim zaczniesz — co to jest ta gra?

Odwrócony pong. **Ty jesteś piłką** (`●`). AI steruje paletką po lewej stronie.
Twoim celem jest *ominąć* paletkę — jeśli piłka prześlizgnie się obok, dostajesz punkt.
Jeśli paletka ją złapie, piłka odbija się z powrotem i tracisz streak.

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

## Pliki

```
cmd/tui/
  main.go        — silnik gry: pętla, fizyka, rysowanie
  audio.go       — generowanie dźwięków PCM w Go
  scoreboard.go  — Hall of Fame: zapis, odczyt, rysowanie
```

---

## `main.go` — serce gry

### Stałe wymiarów i integer scaling

Gra ma **stałą logiczną przestrzeń** (44×23) i renderuje ją w dowolnie dużym terminalu.

```go
const (
    logicCols = 44   // szerokość pola gry w komórkach logicznych
    logicRows = 23   // wysokość pola gry w komórkach logicznych
    hudRows   = 2    // wiersz 0: HUD, wiersz 1: separator ─────
    botWall   = 1    // wiersz na dolną ścianę ─────
)
```

**Dlaczego te liczby?** Terminal to siatka znaków, nie piksele.
Komórka terminala ma proporcje ok. 1:2 (szerokość:wysokość).
Pole 44×23 przy tym współczynniku wygląda podobnie do płótna webowego 800×480.

**Integer scaling** — `calcScale()` oblicza ile terminal-komórek odpowiada jednej logicznej komórce:

```go
func (e *Engine) calcScale() {
    e.termW, e.termH = e.scr.Size()
    avail := e.termH - hudRows - botWall
    byW := e.termW / logicCols   // ile razy zmieści się w poziomie
    byH := avail / logicRows     // ile razy zmieści się w pionie
    e.rScale = min(byW, byH)     // wybierz mniejszy (zachowaj proporcje)
    if e.rScale < 1 { e.rScale = 1 }

    fw := logicCols * e.rScale
    fh := logicRows * e.rScale
    e.fOffX = (e.termW - fw) / 2       // wyśrodkuj poziomo
    e.fOffY = hudRows + (avail-fh)/2   // wyśrodkuj pionowo (pod HUD)
}
```

Fizyka zawsze działa w przestrzeni 44×23. Rysowanie mnoży pozycje przez `rScale` i dodaje `fOffX/fOffY`. Przy `rScale=1` gra wygląda jak klasyczny terminal 59×26; przy `rScale=3` każda komórka to blok 3×3 terminala.

`calcScale()` jest wywoływane przy starcie i za każdym razem gdy terminal zmieni rozmiar (`EventResize`).

---

### Stany gry — automat skończony

Gra jest **maszyną stanów**. W każdej chwili jest dokładnie w jednym stanie:

```go
type GameState int

const (
    StateMenu        // ekran startowy z tickerem HOF
    StateCountdown   // 3… 2… 1… GO!
    StatePlaying     // właściwa gra
    StatePaused      // pauza
    StateNickInput   // wpisywanie inicjałów po game over
    StateScoreboard  // Hall of Fame
)
```

Przejścia stanów są **wyłącznie przez zdarzenia klawiatury lub koniec czasu**:

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

### Struktury danych

#### `Ball` — piłka (= gracz)

```go
type Ball struct{ X, Y, VX, VY float64 }
```

- `X, Y` — pozycja w **komórkach terminala** (float, zaokrąglana przy rysowaniu)
- `VX, VY` — prędkość w **komórkach na sekundę**
- Gracz zmienia tylko `Y` przez strzałki; `VX` i `VY` działają automatycznie

#### `Paddle` — paletka AI

```go
type Paddle struct {
    Y, H        float64   // pozycja górnej krawędzi i wysokość
    jitterTimer float64   // kiedy losować następne drżenie (easy)
    jitterOff   float64   // obecne przesunięcie drżenia
    errorTimer  float64   // kiedy losować następny błąd (medium/hard)
    errorOff    float64   // obecne przesunięcie błędu
    targetY     float64   // cel do którego zmierza paletka
}
```

#### `Engine` — wszystko razem

```go
type Engine struct {
    scr    tcell.Screen   // ekran terminala
    state  GameState
    level  int            // 1=easy, 2=medium, 3=hard
    ball   Ball
    paddle Paddle
    score, streak, bestStreak int
    timeLeft float64
    lastUp, lastDown time.Time   // czas ostatniego wciśnięcia strzałki
    nickBuf [3]rune              // bufor wpisywanego nicka
    nickLen int
    sb      *scoreboardState
}
```

---

### Pętla główna

```go
ticker := time.NewTicker(time.Second / 60)  // 60 FPS

for running {
    <-ticker.C            // czekaj na następną klatkę

    dt = now - lastTick   // czas który minął (w sekundach)

    // 1. pobierz wszystkie zdarzenia z bufora
    for { ev := <-evCh; e.processEvent(ev) }

    // 2. zaktualizuj stan gry
    e.update(dt)

    // 3. narysuj klatkę
    e.draw()
}
```

Dlaczego `dt` zamiast stałego kroku? Bo komputer może być wolny, okno nieaktywne, GC może zatrzymać program. `dt` sprawia że fizyka działa tak samo niezależnie od chwilowych opóźnień. Maksymalne `dt = 0.05s` chroni przed "teleportacją" piłki po długiej przerwie.

---

### Płynny ruch gracza — dlaczego nie `keydown/keyup`?

`tcell` **nie ma zdarzenia keyup** — terminal tego nie wspiera. Klasyczne podejście (`upHeld = true` na keydown, `upHeld = false` na keyup) nie działa.

Rozwiązanie: **timestamp ostatniego wciśnięcia**.

```go
// w processEvent:
if ev.Key() == tcell.KeyUp {
    e.lastUp = time.Now()
}

// w updatePlaying:
if time.Since(e.lastUp) < holdThreshold {  // 90ms
    e.ball.Y -= playerSpeed * dt
}
```

OS wysyła key-repeat (kolejne zdarzenia) póki trzymasz klawisz, zwykle co ~30ms. Jeśli od ostatniego zdarzenia minęło mniej niż 90ms — klawisz jest "wciśnięty". To daje płynny ruch przy 60 FPS.

---

### Fizyka piłki

```go
// ruch gracza
if time.Since(e.lastUp) < holdThreshold { e.ball.Y -= playerSpeed * dt }
if time.Since(e.lastDown) < holdThreshold { e.ball.Y += playerSpeed * dt }

// autonomiczny ruch
e.ball.X += e.ball.VX * dt
e.ball.Y += e.ball.VY * dt

// odbicia od ścian (top/bottom/right)
if e.ball.Y < 0       { e.ball.VY =  math.Abs(e.ball.VY); soundBounce() }
if e.ball.Y >= fieldRows { e.ball.VY = -math.Abs(e.ball.VY); soundBounce() }
if e.ball.X >= fieldCols-1 { e.ball.VX = -math.Abs(e.ball.VX); soundBounce() }
```

Oba ruchy (gracz i fizyka) dodają się do pozycji w tej samej klatce — gracz "nakłada" swój ruch na autonomiczny lot piłki.

---

### Wykrywanie kolizji z paletką

```go
if e.ball.X < float64(paddleWidth) {
    if e.ball.Y >= e.paddle.Y && e.ball.Y <= e.paddle.Y+e.paddle.H {
        // paletka złapała — odbicie
        e.ball.VX = math.Abs(e.ball.VX)       // leć w prawo
        e.ball.VY += rand * 0.15               // lekki nudge kąta
        e.streak = 0
    } else {
        // paletka chybiła — punkt!
        e.score += points * streakMult
        // piłka wraca na środek, trochę szybsza
        speed *= ballSpeedupRate  // ×1.05
    }
}
```

---

### AI paletki

Trzy poziomy:

#### Easy — drżenie losowe (jitter)
Co `jitterInterval` sekund losuje przesunięcie `±jitterRange`. Celuje w `ball.Y + jitterOffset`. Nigdy nie przewiduje — po prostu śledzi piłkę z losowym błędem.

#### Medium — przewidywanie + błędy
Kiedy piłka jedzie w lewo (`VX < 0`), wywołuje `predictY()` — symuluje tor piłki do lewej ściany uwzględniając odbicia. Z szansą `errChance` (30%) dodaje losowy błąd do celu.

#### Hard — to samo ale rzadsze i mniejsze błędy
`errChance = 8%`, `errRange` mały, `deadZone` prawie zerowa — paletka jest prawie idealna.

#### `predictY` — symulator toru

```go
func predictY(ball Ball, targetX, fw, fh float64) float64 {
    x, y, vx, vy := ball.X, ball.Y, ball.VX, ball.VY
    for i := 0; i < 5000; i++ {
        x += vx * 0.01  // kroczek 10ms
        y += vy * 0.01
        // obsługa odbić od ścian...
        if vx < 0 && x <= targetX { return y }  // dotarliśmy
    }
    return y
}
```

Symuluje lot piłki krokami 10ms i zwraca Y w miejscu docelowym. Paletka celuje w ten punkt zamiast bieżącej pozycji piłki — stąd AI wydaje się "czytać" twoje ruchy.

---

### Rysowanie

`tcell` pozwala ustawić dowolny znak w dowolnej komórce terminala:

```go
e.scr.SetContent(col, row, '●', nil, styleBall)
```

Piłka jest **jednym znakiem** `●` w zaokrąglonej pozycji:

```go
bx := int(math.Round(e.ball.X))
by := int(math.Round(e.ball.Y)) + hudRows
e.scr.SetContent(bx, by, '●', nil, styleBall)
```

Paletka to dwa znaki szerokości (`█▌`) rysowane w pętli po wierszach:

```go
for row := paddleTop; row <= paddleBot; row++ {
    e.scr.SetContent(0, row+hudRows, '█', nil, stylePaddle)
    e.scr.SetContent(1, row+hudRows, '▌', nil, stylePaddle)
}
```

Na końcu `e.scr.Show()` wysyła zmiany do terminala **w jednej operacji** (double buffering wbudowany w tcell — nie ma migotania).

---

## `audio.go` — dźwięki bez plików

Biblioteka `oto/v3` otwiera urządzenie audio i odtwarza surowe bajty PCM.
Wszystkie dźwięki są **generowane matematycznie** w locie.

### Sinusoida z fade-out

```go
func tone(freq, freqEnd float64, dur time.Duration, vol float64) []byte {
    n := sampleRate * dur.Seconds()   // liczba próbek
    for i := 0; i < n; i++ {
        t    := float64(i) / sampleRate
        f    := lerp(freq, freqEnd, i/n)      // sweep częstotliwości
        fade := sqrt(1 - i/n)                  // wygaszanie amplitudy
        s    := sin(2π * f * t) * vol * fade
        // zamień na signed int16 little-endian...
    }
}
```

**PCM** = próbki amplitudy fali dźwiękowej, 44100 na sekundę, 16-bit signed.
Każda próbka to 2 bajty (little-endian). Sinus daje czysty ton, `freqEnd != 0` robi sweep.

### Dźwięki w grze

| Funkcja | Dźwięk | Kiedy |
|---|---|---|
| `soundBounce()` | krótki 880Hz | odbicie od ściany |
| `soundHit()` | opadający sweep 220→110Hz | paletka złapała piłkę |
| `soundScore()` | 3 rosnące tony 660→880→1100Hz | punkt gracza |
| `soundCountdown()` | beep 440Hz | każda cyfra odliczania |
| `soundGo()` | chord 660+880Hz | "GO!" |
| `soundGameOver()` | opadające 440→330→220Hz | koniec gry |

Wszystko gra w osobnych goroutine (`go playBuf(...)`) żeby nie blokować pętli gry.

---

## `scoreboard.go` — Hall of Fame

### Gdzie są dane?

```
~/.config/reverse-pong-tui/hof.json   (macOS/Linux)
```

Format JSON, max 100 wpisów, posortowane malejąco po score.

### Cykl życia wpisu

```
koniec gry
    → StateNickInput  (gracz wpisuje 3 litery)
    → sb.add(nick, score, level)
        → append do s.all
        → sort malejąco
        → przytnij do 100
        → saveHOF() → zapis JSON
    → StateScoreboard (własny wynik podświetlony na zielono)
```

### Filtrowanie po poziomie

`visible()` filtruje `s.all` według aktywnego taba:

```go
func (s *scoreboardState) visible() []hofEntry {
    if tabLevels[s.tab] == "" { return s.all }   // ALL
    // filtruj po poziomie...
}
```

Taby: `ALL / EASY / MEDIUM / HARD` — przełączane `←→`.

### Ticker w menu

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

`offset` rośnie z czasem (`UnixMilli/80` = ~12 kroków/sekundę) i "przesuwa okno" po tekście. Modulo długości tekstu powoduje zapętlenie.

---

## Jak uruchomić

```bash
make play        # kompiluje + otwiera nowe okno Ghostty 59×26
make tui         # tylko kompilacja → ./reverse-pong-tui
```

Klawisze:

| Stan | Klawisz | Akcja |
|---|---|---|
| Menu | `↑↓` | zmiana poziomu |
| Menu | `1` `2` `3` | wybór poziomu |
| Menu | `ENTER` / dowolny | start |
| Menu | `S` | Hall of Fame |
| Gra | `↑↓` | ruch piłki |
| Gra | `ESC` / `P` | pauza |
| Pauza | `ESC` / `P` | wznów |
| Pauza | `Q` | menu |
| Nick | litery | wpisz inicjały |
| Nick | `Backspace` | kasuj |
| Nick | `ENTER` | zapisz (tylko przy 3 literach) |
| Nick | `ESC` | pomiń zapis |
| HOF | `←→` | zmień tab |
| HOF | `↑↓` | scroll |
| HOF | `R` / `ESC` | menu |
| Wszędzie | `Ctrl+C` | wyjście |

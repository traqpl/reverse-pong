.PHONY: all wasm wasm-exec server dev build tui play clean

WASM_OUT   = server/web/game.wasm
BINARY     = reverse-pong
TUI_BINARY = reverse-pong-tui

all: wasm
	@sleep 2 && open http://localhost:8080 &
	go run ./server/

wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $(WASM_OUT) ./game/

wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/

server:
	go run ./server/

dev: wasm
	go run ./server/

build: wasm
	go build -ldflags="-s -w" -o $(BINARY) ./server/

tui:
	go build -ldflags="-s -w" -o $(TUI_BINARY) ./cmd/tui/

play: tui
	open -na Ghostty.app --args --window-width=220 --window-height=50 -e $(abspath $(TUI_BINARY))

clean:
	rm -f $(WASM_OUT) $(BINARY) $(TUI_BINARY) server/web/wasm_exec.js

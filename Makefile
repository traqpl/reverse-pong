.PHONY: all wasm wasm-exec server dev build tui play certs clean

WASM_OUT   = server/web/game.wasm
BINARY     = reverse-pong
TUI_BINARY = reverse-pong-tui
CERTS_DIR  ?= certs

all: wasm certs
	@sleep 2 && open https://localhost:8080 &
	go run ./server/

wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $(WASM_OUT) ./game/

wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/

certs:
	go run ./cmd/certgen $(CERTS_DIR)

server:
	go run ./server/

dev: wasm certs
	go run ./server/

build: wasm certs
	go build -ldflags="-s -w" -o $(BINARY) ./server/

tui:
	go build -ldflags="-s -w" -o $(TUI_BINARY) ./cmd/tui/

play: tui
	open -na Ghostty.app --args --window-width=220 --window-height=50 -e $(abspath $(TUI_BINARY))

clean:
	rm -f $(WASM_OUT) $(BINARY) $(TUI_BINARY) server/web/wasm_exec.js internal/certdata/encrypted.go

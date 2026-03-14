.PHONY: all wasm wasm-exec server dev build tui play certs deploy logs clean

WASM_OUT   = server/web/game.wasm
BINARY     = reverse-pong
TUI_BINARY = reverse-pong-tui
CERTS_DIR  ?= certs

# ── deployment ────────────────────────────────────────────────────────────────
REMOTE_HOST = daemon
REMOTE_DIR  = ~/reverse-pong
REMOTE_BIN  = $(REMOTE_DIR)/$(BINARY)
REMOTE_LOG  = $(REMOTE_DIR)/server.log
PORT        = 443

all: wasm certs
	@sleep 2 && open https://localhost:8080 &
	go run ./server/

BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M')

wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w -X 'main.BuildTime=$(BUILD_TIME)'" -o $(WASM_OUT) ./game/

wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/

certs:
	go run ./cmd/certgen $(CERTS_DIR)

server: wasm certs
	go run ./server/

dev: wasm certs
	go run ./server/

build: wasm certs
	go build -ldflags="-s -w" -o $(BINARY) ./server/

tui:
	go build -ldflags="-s -w" -o $(TUI_BINARY) ./cmd/tui/

play: tui
	open -na Ghostty.app --args --window-width=220 --window-height=50 -e $(abspath $(TUI_BINARY))

deploy: wasm certs
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY) ./server/
	ssh $(REMOTE_HOST) "mkdir -p $(REMOTE_DIR) && pkill -x $(BINARY) || true; sleep 1; rm -f $(REMOTE_BIN)"
	scp $(BINARY) $(REMOTE_HOST):$(REMOTE_BIN)
	scp scripts/start-server.sh $(REMOTE_HOST):$(REMOTE_DIR)/start-server.sh
	scp config.yaml $(REMOTE_HOST):$(REMOTE_DIR)/config.yaml
	ssh $(REMOTE_HOST) "mkdir -p $(REMOTE_DIR)/music"
	scp -r music/* $(REMOTE_HOST):$(REMOTE_DIR)/music/
	ssh $(REMOTE_HOST) " \
		chmod +x $(REMOTE_DIR)/start-server.sh; \
		$(REMOTE_DIR)/start-server.sh"

logs:
	ssh $(REMOTE_HOST) "tail -f $(REMOTE_LOG)"

clean:
	rm -f $(WASM_OUT) $(BINARY) $(TUI_BINARY) server/web/wasm_exec.js internal/certdata/encrypted.go

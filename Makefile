.PHONY: all wasm wasm-exec server dev build tui play music lint sec test ci docker-build deploy deploy-agent logs clean

WASM_OUT   = server/web/game.wasm
MUSIC_SRC  = music/Serca w pikselach.mp3
MUSIC_OUT  = server/web/music.mp3
BINARY     = reverse-pong
TUI_BINARY = reverse-pong-tui
GOLANGCI_LINT_VERSION ?= v2.8.0
GOSEC_VERSION ?= v2.22.2

# ── deployment ────────────────────────────────────────────────────────────────
REMOTE_HOST ?= daemon
REMOTE_DIR  ?= ~/reverse-pong
REMOTE_BIN  := $(REMOTE_DIR)/$(BINARY)
REMOTE_LOG  := $(REMOTE_DIR)/server.log
PORT        ?= 8073
DOCKER_IMAGE ?= reverse-pong:latest

ifeq ($(shell uname -s),Darwin)
BROWSER_OPEN = open
else
BROWSER_OPEN = xdg-open
endif

BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M')

all: lint sec wasm
	@sleep 2 && $(BROWSER_OPEN) http://localhost:$(PORT) &
	go run ./server/

music:
	ffmpeg -i "$(MUSIC_SRC)" -codec:a libmp3lame -b:a 96k -ar 44100 $(MUSIC_OUT) -y

wasm: lint sec
	GOOS=js GOARCH=wasm go build -ldflags="-s -w -X 'main.BuildTime=$(BUILD_TIME)'" -o $(WASM_OUT) ./game/

wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/

server: lint sec wasm
	go run ./server/

dev: lint sec wasm
	go run ./server/

build: lint sec wasm
	go build -ldflags="-s -w" -o $(BINARY) ./server/

tui:
	go build -ldflags="-s -w" -o $(TUI_BINARY) ./cmd/tui/

play: tui
	$(BROWSER_OPEN) -na Ghostty.app --args --window-width=220 --window-height=50 -e $(abspath $(TUI_BINARY))

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

sec:
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) ./...

test:
	go test ./...

ci: lint sec test wasm build

docker-build: lint sec
	docker build -t $(DOCKER_IMAGE) .

deploy: lint sec music wasm
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY) ./server/
	ssh $(REMOTE_HOST) "mkdir -p $(REMOTE_DIR) && pkill -x $(BINARY) || true; sleep 1; rm -f $(REMOTE_BIN)"
	scp $(BINARY) $(REMOTE_HOST):$(REMOTE_BIN)
	scp scripts/start-server.sh $(REMOTE_HOST):$(REMOTE_DIR)/start-server.sh
	scp config.yaml $(REMOTE_HOST):$(REMOTE_DIR)/config.yaml
	ssh $(REMOTE_HOST) "chmod +x $(REMOTE_DIR)/start-server.sh && PORT=$(PORT) $(REMOTE_DIR)/start-server.sh"

deploy-agent: lint sec wasm
	DEPLOY_DIR="$${DEPLOY_DIR:-$$HOME/reverse-pong}"; \
	mkdir -p "$$DEPLOY_DIR" && \
	docker build -t $(DOCKER_IMAGE) . && \
	cp compose.yaml "$$DEPLOY_DIR/compose.yaml" && \
	cp config.yaml "$$DEPLOY_DIR/config.yaml" && \
	IMAGE_NAME=$(DOCKER_IMAGE) PORT=$(PORT) docker compose -f "$$DEPLOY_DIR/compose.yaml" up -d --force-recreate --remove-orphans

logs:
	ssh $(REMOTE_HOST) "tail -f $(REMOTE_LOG)"

clean:
	rm -f $(WASM_OUT) $(BINARY) $(TUI_BINARY) server/web/wasm_exec.js

.PHONY: all wasm wasm-exec server dev build clean

WASM_OUT = server/web/game.wasm
BINARY   = reverse-pong

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

clean:
	rm -f $(WASM_OUT) $(BINARY) server/web/wasm_exec.js

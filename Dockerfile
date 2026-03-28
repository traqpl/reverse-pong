FROM golang:1.26@sha256:f200f27a113fd26789f07ff95ec1f7e337e295ddb711c693cf5b18a6dc7e88f5 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o server/web/game.wasm ./game/
RUN cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" server/web/wasm_exec.js

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/reverse-pong ./server/

FROM scratch

WORKDIR /app

COPY --from=build /out/reverse-pong /app/reverse-pong
COPY --from=build /src/config.yaml /app/config.yaml

ENV PORT=8073
ENV DB_PATH=/data/reverse_pong_scores.db

EXPOSE 8073
VOLUME ["/data"]

ENTRYPOINT ["/app/reverse-pong"]

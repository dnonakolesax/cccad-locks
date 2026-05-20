# Установка модулей и тесты
FROM golang:1.25.5 AS modules

COPY go.mod go.sum /m/
RUN cd /m && go mod download

# Сборка приложения
FROM golang:1.25.5 AS builder

COPY --from=modules /go/pkg /go/pkg

# Пользователь без прав
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    useradd -u 10001 -m -d /sketch-runner sketch-runner

RUN mkdir -p /cccad-sketches
COPY . /cccad-sketches
WORKDIR /cccad-sketches
COPY db/requests ./db/requests

# Сборка
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -o ./bin/cccad-sketches ./cmd/api
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -o ./bin/cccad-migrate ./cmd/migrate

# Запуск приложения
FROM alpine:3.22

RUN apk add --no-cache ca-certificates && \
    addgroup -S sketch-runner && \
    adduser -S -D -h /sketch-runner -G sketch-runner -u 10001 sketch-runner

# Запускаем от имени этого пользователя
USER sketch-runner

COPY --from=builder /cccad-sketches/bin/cccad-sketches /cccad-sketches
COPY --from=builder /cccad-sketches/bin/cccad-migrate /cccad-migrate
COPY --from=builder /cccad-sketches/db/requests /db/requests
COPY --from=builder /cccad-sketches/migrations /migrations

CMD ["/bin/sh", "-c", "/cccad-migrate -configs /configs -dir /migrations up && exec /cccad-sketches -configs /configs"]

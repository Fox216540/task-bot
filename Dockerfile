FROM golang:1.26.2-alpine AS builder

WORKDIR /app

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/task-bot ./cmd/task-bot

FROM alpine:3.22

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app \
    && apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/task-bot /usr/local/bin/task-bot

USER app

ENV DB_PATH=/app/data/task_bot.sqlite

CMD ["task-bot"]

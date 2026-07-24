# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.3

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.22 AS runner

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S app -G app

COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/seed /usr/local/bin/seed
COPY migrations ./migrations
COPY .env.example ./.env.example

ENV APP_PORT=8080
EXPOSE 8080

USER app

CMD ["api"]

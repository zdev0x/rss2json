# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rss2json ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

COPY --from=builder /rss2json /usr/local/bin/rss2json

EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/usr/local/bin/rss2json"]

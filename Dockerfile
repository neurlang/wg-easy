FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o wg-easy-go

FROM alpine:latest

RUN apk add --no-cache wireguard-tools iptables ip6tables

WORKDIR /app
COPY --from=builder /app/wg-easy-go .
COPY config.example.json ./config.json

EXPOSE 8080
EXPOSE 51820/udp

CMD ["./wg-easy-go"]

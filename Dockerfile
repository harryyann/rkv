FROM golang:1.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o rkvd cmd/server/main.go
RUN groupadd -g 999 user && \
    useradd -r -u 999 -g user user

FROM ubuntu:latest

RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /app/rkvd .
USER user

ENTRYPOINT ["/app/rkvd"]

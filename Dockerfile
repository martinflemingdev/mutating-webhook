# Build stage
FROM golang:1.23 AS builder

WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -o webhook-server

# Final stage
FROM debian:stable-slim

WORKDIR /app
COPY --from=builder /app/webhook-server /app/webhook-server

CMD ["/app/webhook-server"]

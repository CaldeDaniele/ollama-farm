# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ollama-farm .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/ollama-farm .

EXPOSE 8080

# Server only, no TUI (headless in container)
ENTRYPOINT ["/app/ollama-farm", "server", "--no-tui"]
CMD ["--port", "8080"]

# Build stage: server binary + release artifacts for all platforms
# RUNTIME_ARCH: arch of /app/ollama-farm in the final image. Default amd64 = RunPod / most cloud GPUs.
# Mac ARM local docker build used to produce arm64 → exec format error on amd64 hosts. Override if needed:
#   docker buildx build --build-arg RUNTIME_ARCH=arm64   (e.g. ARM64 VPS)
FROM golang:1.25-alpine AS builder
ARG RUNTIME_ARCH=amd64
WORKDIR /app

RUN apk add --no-cache zip

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Release artifacts (for /download/ when no GitHub release)
RUN mkdir -p /app/releases

# Windows amd64 (zip must contain ollama-farm.exe)
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o /app/releases/ollama-farm.exe . \
  && cd /app/releases && zip ollama-farm_windows_amd64.zip ollama-farm.exe && rm ollama-farm.exe

# Linux amd64 tar.gz (archive must contain file named "ollama-farm")
RUN mkdir -p /app/out \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/out/ollama-farm . \
  && tar -C /app/out -czvf /app/releases/ollama-farm_linux_amd64.tar.gz ollama-farm

# Darwin amd64 + arm64 (for macOS install.sh)
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o /app/out/ollama-farm . \
  && tar -C /app/out -czvf /app/releases/ollama-farm_darwin_amd64.tar.gz ollama-farm
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o /app/out/ollama-farm . \
  && tar -C /app/out -czvf /app/releases/ollama-farm_darwin_arm64.tar.gz ollama-farm

# Linux binary for server — must match pod CPU (cross-compile from any builder arch)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${RUNTIME_ARCH} go build -ldflags="-s -w" -o ollama-farm .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/ollama-farm .
COPY --from=builder /app/releases /app/releases

EXPOSE 8080

# Server with --releases-dir so install.sh / install.ps1 get binaries from this server (no GitHub needed)
ENTRYPOINT ["/app/ollama-farm", "server", "--no-tui", "--releases-dir", "/app/releases"]
CMD ["--port", "8080"]

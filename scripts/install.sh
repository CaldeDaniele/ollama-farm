#!/bin/sh
set -e

REPO="danielecalderazzo/ollama-farm"
BINARY="ollama-farm"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Please download manually from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version"
  exit 1
fi

FILENAME="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${FILENAME}"

echo "Downloading ollama-farm $VERSION for $OS/$ARCH..."
curl -fsSL "$URL" | tar -xz "$BINARY"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)"
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "ollama-farm installed to $INSTALL_DIR/$BINARY"
echo "Run: ollama-farm --help"

#!/bin/sh
set -e

REPO="charkala/peth"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="peth"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    *)
        echo "Error: unsupported OS: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        exit 1
        ;;
esac

ASSET="${BIN_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

echo "Installing peth (${OS}/${ARCH})..."

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

if ! curl -fsSL "$URL" -o "$TMPFILE"; then
    echo "Error: failed to download ${ASSET}"
    echo "Check available assets at https://github.com/${REPO}/releases/latest"
    exit 1
fi

chmod +x "$TMPFILE"

if [ -w "$INSTALL_DIR" ]; then
    mv "$TMPFILE" "${INSTALL_DIR}/${BIN_NAME}"
else
    sudo mv "$TMPFILE" "${INSTALL_DIR}/${BIN_NAME}"
fi

echo "Installed peth to ${INSTALL_DIR}/${BIN_NAME}"
peth version

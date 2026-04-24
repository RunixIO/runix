#!/usr/bin/env bash
set -euo pipefail

REPO="runixio/runix"
BINARY="runix"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        echo "Error: unsupported OS: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Find the install directory.
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Determine the version to install.
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo "Looking up latest version..."
    VERSION=$(curl -sfL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Error: could not determine latest version"
        exit 1
    fi
fi

echo "Installing ${BINARY} ${VERSION} for ${OS}/${ARCH}..."

# Build the download URL.
FILENAME="${BINARY}_${OS}_${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}.tar.gz"

# Create a temp directory for the download.
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download.
echo "Downloading ${URL}..."
if ! curl -sfL "$URL" -o "${TMPDIR}/${BINARY}.tar.gz"; then
    echo "Error: download failed"
    echo "The release for ${OS}/${ARCH} may not exist yet."
    exit 1
fi

# Extract.
tar -xzf "${TMPDIR}/${BINARY}.tar.gz" -C "${TMPDIR}"

# Install.
if [ ! -f "${TMPDIR}/${BINARY}" ]; then
    echo "Error: binary not found in archive"
    exit 1
fi

chmod +x "${TMPDIR}/${BINARY}"

if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
    echo "Requires sudo to install to ${INSTALL_DIR}..."
    sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" version

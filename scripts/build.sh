#!/usr/bin/env bash
set -euo pipefail

# Build script for Runix.
# Usage: ./scripts/build.sh [version]
#   version: tag or commit-ish to embed (default: git describe)

BINARY="runix"
BUILD_DIR="./bin"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

LDFLAGS="-s -w -X github.com/runixio/runix/internal/version.Version=${VERSION} -X github.com/runixio/runix/internal/version.BuildTime=${BUILD_TIME}"

echo "Building ${BINARY} ${VERSION}..."

mkdir -p "${BUILD_DIR}"

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

echo "  OS:      ${GOOS}"
echo "  Arch:    ${GOARCH}"
echo "  Version: ${VERSION}"
echo "  Time:    ${BUILD_TIME}"

CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "${LDFLAGS}" \
    -o "${BUILD_DIR}/${BINARY}" \
    ./cmd/runix

echo "Built: ${BUILD_DIR}/${BINARY}"
"${BUILD_DIR}/${BINARY}" version

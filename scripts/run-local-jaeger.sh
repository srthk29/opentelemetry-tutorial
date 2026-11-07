#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${DIR}/bin"
JAEGER_BIN="${BIN_DIR}/jaeger-all-in-one"
CONFIG="${DIR}/jaeger-v2-local.yaml"

mkdir -p "${BIN_DIR}"

if ! command -v "${JAEGER_BIN}" >/dev/null 2>&1; then
    echo "▶ Jaeger binary not found at ${JAEGER_BIN}."
    echo "▶ Detecting macOS architecture to download native standalone binary..."

    ARCH="$(uname -m)"
    if [ "${ARCH}" = "x86_64" ]; then
        JAEGER_ARCH="amd64"
    elif [ "${ARCH}" = "arm64" ]; then
        JAEGER_ARCH="arm64"
    else
        echo "Unsupported architecture: ${ARCH}"
        exit 1
    fi

    VERSION="2.23.0"
    TAR_NAME="jaeger-${VERSION}-darwin-${JAEGER_ARCH}.tar.gz"
    URL="https://github.com/jaegertracing/jaeger/releases/download/v${VERSION}/${TAR_NAME}"

    echo "▶ Downloading Jaeger v${VERSION} for macOS (${JAEGER_ARCH})..."
    TMP_DIR=$(mktemp -d)
    curl -sL "${URL}" | tar -xz -C "${TMP_DIR}"
    if [ -f "${TMP_DIR}/jaeger-${VERSION}-darwin-${JAEGER_ARCH}/jaeger" ]; then
        mv "${TMP_DIR}/jaeger-${VERSION}-darwin-${JAEGER_ARCH}/jaeger" "${JAEGER_BIN}"
    elif [ -f "${TMP_DIR}/jaeger-${VERSION}-darwin-${JAEGER_ARCH}/jaeger-all-in-one" ]; then
        mv "${TMP_DIR}/jaeger-${VERSION}-darwin-${JAEGER_ARCH}/jaeger-all-in-one" "${JAEGER_BIN}"
    fi
    rm -rf "${TMP_DIR}"
    chmod +x "${JAEGER_BIN}"

    # Remove macOS quarantine attribute if set
    xattr -d com.apple.quarantine "${JAEGER_BIN}" 2>/dev/null || true
    echo "✓ Downloaded ${JAEGER_BIN} successfully."
fi

echo "=========================================================="
echo " Starting Jaeger All-In-One (Native macOS Standalone Mode)"
echo " Web UI             : http://localhost:16686"
echo " OTLP gRPC Receiver : localhost:4317 (or localhost:4319 if port busy)"
echo " OTLP HTTP Receiver : localhost:4318"
echo "=========================================================="

if [ -f "${CONFIG}" ]; then
    echo " Config             : ${CONFIG}"
    echo "=========================================================="
    exec "${JAEGER_BIN}" --config="${CONFIG}"
else
    echo "=========================================================="
    exec "${JAEGER_BIN}"
fi

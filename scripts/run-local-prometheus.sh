#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${DIR}/bin"
PROM_BIN="${BIN_DIR}/prometheus"
CONFIG="${DIR}/prometheus-local.yaml"

mkdir -p "${BIN_DIR}"

# 1. Prefer Homebrew prometheus if installed
if command -v prometheus >/dev/null 2>&1; then
    EXEC_CMD="prometheus"
elif [ -f "${PROM_BIN}" ]; then
    EXEC_CMD="${PROM_BIN}"
else
    echo "▶ Prometheus not found on system."
    echo "▶ Detecting macOS architecture to download native standalone binary..."

    ARCH="$(uname -m)"
    if [ "${ARCH}" = "x86_64" ]; then
        PROM_ARCH="amd64"
    elif [ "${ARCH}" = "arm64" ]; then
        PROM_ARCH="arm64"
    else
        echo "Unsupported architecture: ${ARCH}"
        exit 1
    fi

    VERSION="3.1.0"
    TAR_NAME="prometheus-${VERSION}.darwin-${PROM_ARCH}.tar.gz"
    URL="https://github.com/prometheus/prometheus/releases/download/v${VERSION}/${TAR_NAME}"

    echo "▶ Downloading Prometheus v${VERSION} for macOS (${PROM_ARCH})..."
    TMP_DIR=$(mktemp -d)
    curl -sL "${URL}" | tar -xz -C "${TMP_DIR}"
    mv "${TMP_DIR}/prometheus-${VERSION}.darwin-${PROM_ARCH}/prometheus" "${PROM_BIN}"
    rm -rf "${TMP_DIR}"
    chmod +x "${PROM_BIN}"

    # Remove macOS quarantine attribute if set
    xattr -d com.apple.quarantine "${PROM_BIN}" 2>/dev/null || true
    echo "✓ Downloaded ${PROM_BIN} successfully."
    EXEC_CMD="${PROM_BIN}"
fi

echo "=========================================================="
echo " Starting Prometheus (Native macOS Standalone Mode)"
echo " Web UI             : http://localhost:9090"
echo " Scrape Target      : localhost:8889 (Local OTel Collector)"
echo " Config             : ${CONFIG}"
echo "=========================================================="

exec "${EXEC_CMD}" --config.file="${CONFIG}" --storage.tsdb.path="${BIN_DIR}/prom_data"

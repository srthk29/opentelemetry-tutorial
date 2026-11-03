#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${DIR}/bin"
OTELCOL="${BIN_DIR}/otelcol"
CONFIG="${DIR}/otel-collector-local.yaml"

mkdir -p "${BIN_DIR}"

if ! command -v "${OTELCOL}" >/dev/null 2>&1; then
    echo "▶ OpenTelemetry Collector binary not found at ${OTELCOL}."
    echo "▶ Detecting platform to download standalone binary..."

    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    if [ "${ARCH}" = "x86_64" ]; then
        ARCH="amd64"
    elif [ "${ARCH}" = "aarch64" ] || [ "${ARCH}" = "arm64" ]; then
        ARCH="arm64"
    fi

    VERSION="0.164.0"
    TAR_NAME="otelcol_${VERSION}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${VERSION}/${TAR_NAME}"

    echo "▶ Downloading ${URL}..."
    curl -sL "${URL}" | tar -xz -C "${BIN_DIR}"
    echo "✓ Downloaded ${OTELCOL} successfully."
fi

echo "=========================================================="
echo " Starting OpenTelemetry Collector (Local Standalone Mode)"
echo " OTLP gRPC Receiver : localhost:4317"
echo " OTLP HTTP Receiver : localhost:4318"
echo " Prometheus Metrics : http://localhost:8889/metrics"
echo " Config             : ${CONFIG}"
echo "=========================================================="

exec "${OTELCOL}" --config="${CONFIG}"

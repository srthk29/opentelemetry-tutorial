# Monitoring Infrastructure on macOS (Prometheus & Jaeger)

This directory contains configuration files, scripts, and guides to run the local monitoring stack on macOS.

---

## 🚀 Quick Launch Cheat Sheet

### Method 1: Docker Compose (All-in-One ⭐ Recommended)
```bash
# Start Prometheus, Jaeger, OTel Collector, and PostgreSQL in Docker
make docker-up

# Stop all containers
make docker-down
```
* **Jaeger Web UI**: [http://localhost:16686](http://localhost:16686)
* **Prometheus Web UI**: [http://localhost:9090](http://localhost:9090)

---

### Method 2: Native macOS Standalone (Zero Docker)
Run each in a separate terminal tab:

```bash
# Terminal 1: Run native Jaeger All-in-One
make run-jaeger

# Terminal 2: Run native Prometheus (scrapes localhost:8889)
make run-prometheus

# Terminal 3: Run standalone OpenTelemetry Collector
make run-collector

# Terminal 4: Run application with Collector exporter
make run-localai-collector
```

---

## 📁 Monitoring Files in This Project

| File | Purpose |
| :--- | :--- |
| [`docker-compose.yaml`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/docker-compose.yaml) | Docker Compose setup for Postgres, Collector, Jaeger, and Prometheus |
| [`prometheus.yaml`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/prometheus.yaml) | Prometheus scrape config inside Docker (`targets: ["otel-collector:8889"]`) |
| [`prometheus-local.yaml`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/prometheus-local.yaml) | Prometheus scrape config on native macOS (`targets: ["localhost:8889"]`) |
| [`otel-collector-config.yaml`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/otel-collector-config.yaml) | Collector config for Docker Compose (exports to Jaeger container) |
| [`otel-collector-local.yaml`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/otel-collector-local.yaml) | Collector config for local standalone execution |
| [`scripts/run-local-jaeger.sh`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/scripts/run-local-jaeger.sh) | Helper script: downloads & starts native Darwin binary for Apple Silicon / Intel |
| [`scripts/run-local-prometheus.sh`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/scripts/run-local-prometheus.sh) | Helper script: starts Homebrew or standalone Prometheus binary |
| [`docs/PROMETHEUS_AND_JAEGER_MAC_GUIDE.md`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/docs/PROMETHEUS_AND_JAEGER_MAC_GUIDE.md) | Full in-depth macOS installation, architecture, and troubleshooting guide |

---

## 📋 Standard macOS Port Mappings

* **`16686`**: Jaeger Web UI
* **`9090`**: Prometheus Web UI
* **`4317`**: OTLP gRPC receiver (OTel Collector)
* **`4318`**: OTLP HTTP receiver (OTel Collector)
* **`8889`**: Collector Prometheus metrics scrape endpoint
* **`5433`**: PostgreSQL container mapped port (avoids macOS local Postgres on 5432)

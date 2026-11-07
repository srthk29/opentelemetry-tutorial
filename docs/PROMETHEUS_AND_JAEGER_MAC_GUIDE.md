# Complete Guide: Installing and Running Prometheus and Jaeger on macOS

This guide walks you through setting up and running **Prometheus** (Metrics) and **Jaeger** (Distributed Tracing) on macOS. It covers both the **Docker / Docker Compose** method (fastest) and the **Native Standalone macOS** method (zero Docker required).

---

## 🎯 Architecture on macOS

Whether running via containers or native Darwin processes, telemetry flows through this local topology:

```mermaid
flowchart TD
    App["Go Application / Pipeline<br/>(SDK: TracerProvider & MeterProvider)"]

    subgraph OTelCollector ["OpenTelemetry Collector"]
        Recv["OTLP Receivers<br/>(gRPC :4317, HTTP :4318)"]
        Proc["Processors<br/>(batch, memory_limiter)"]
        Exp["Exporters<br/>(debug, otlp/jaeger, prometheus)"]
        Recv --> Proc --> Exp
    end

    App -->|OTLP gRPC (:4317)| Recv

    subgraph Backends ["Monitoring & Tracing Backends"]
        Jaeger["Jaeger All-in-One<br/>• Traces Receiver: :4317 (internal)<br/>• Web UI: http://localhost:16686"]
        Prometheus["Prometheus Server<br/>• Scrapes: :8889/metrics<br/>• Web UI: http://localhost:9090"]
    end

    Exp -->|OTLP Traces| Jaeger
    Exp -.->|Scrapes :8889/metrics| Prometheus
```

---

## 🚀 Method 1: Docker / Docker Compose (Recommended & Fastest)

If you have **Docker Desktop**, **OrbStack**, or **Colima** installed on your Mac, this is the quickest way to spin up the entire observability stack in one command.

### Step 1: Start the Observability Stack
Run from the project root:
```bash
make docker-up
```
*(Or manually: `docker compose up -d`)*

This starts 4 containers in the background:
* **Jaeger Web UI**: [http://localhost:16686](http://localhost:16686)
* **Prometheus Web UI**: [http://localhost:9090](http://localhost:9090)
* **OpenTelemetry Collector**: listening on `localhost:4317` (gRPC) and `localhost:4318` (HTTP)
* **PostgreSQL**: available on `localhost:5433` (mapped to 5433 to prevent conflict with local macOS Postgres)

### Step 2: Send Telemetry Traffic
Run any of the project's applications:
```bash
# Option A: Run the Order API Server + Client Simulator
make run-server &
make run-client

# Option B: Run the Local AI Batch Pipeline
make run-localai-collector
```

### Step 3: View the Telemetry in macOS Browser
* **Traces in Jaeger**: Open [http://localhost:16686](http://localhost:16686), select service `local-ai-batch-service` or `order-service`, and click **Find Traces**.
* **Metrics in Prometheus**: Open [http://localhost:9090](http://localhost:9090), enter query `gen_ai_client_token_usage` or `app_orders_total`, and click **Execute**.

### Step 4: Stop Containers
When finished:
```bash
make docker-down
```

---

## 🍏 Method 2: Native macOS Standalone (Zero Docker Required)

If you do not have Docker installed or prefer native macOS Darwin processes running directly on Apple Silicon or Intel, follow these steps.

### A. Installing & Running Jaeger on macOS

Jaeger provides a standalone **`jaeger-all-in-one`** binary containing the in-memory collector, query engine, and React UI.

#### Option 1: Using the Automated Makefile (Easiest)
```bash
make run-jaeger
```
*(The helper script [`scripts/run-local-jaeger.sh`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/scripts/run-local-jaeger.sh) detects whether your Mac is Apple Silicon (`arm64`) or Intel (`amd64`), downloads the official binary to `./bin/jaeger-all-in-one`, clears macOS quarantine attributes, and launches it).*

#### Option 2: Manual Installation via Terminal
```bash
# 1. Detect architecture
ARCH="$(uname -m | sed 's/x86_64/amd64/')"

# 2. Download official Jaeger v1.66.0 release for Darwin
curl -sL "https://github.com/jaegertracing/jaeger/releases/download/v1.66.0/jaeger-1.66.0-darwin-${ARCH}.tar.gz" | tar -xz

# 3. Clear macOS Gatekeeper quarantine (if blocked)
xattr -d com.apple.quarantine jaeger-1.66.0-darwin-${ARCH}/jaeger-all-in-one 2>/dev/null || true

# 4. Run Jaeger
./jaeger-1.66.0-darwin-${ARCH}/jaeger-all-in-one
```

* **Jaeger Web UI**: [http://localhost:16686](http://localhost:16686)

---

### B. Installing & Running Prometheus on macOS

Prometheus can be installed either via Homebrew or downloaded as a standalone binary.

#### Option 1: Via Homebrew (Standard for Mac developers)
```bash
# 1. Install Prometheus using Homebrew
brew install prometheus

# 2. Run Prometheus using our pre-configured local config:
prometheus --config.file=prometheus-local.yaml
```

#### Option 2: Using the Automated Makefile
```bash
make run-prometheus
```
*(The helper script [`scripts/run-local-prometheus.sh`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/scripts/run-local-prometheus.sh) checks for Homebrew; if absent, it downloads the native Darwin standalone binary into `./bin/prometheus` and starts it).*

#### Option 3: Manual Download without Homebrew
```bash
ARCH="$(uname -m | sed 's/x86_64/amd64/')"
curl -sL "https://github.com/prometheus/prometheus/releases/download/v3.1.0/prometheus-3.1.0.darwin-${ARCH}.tar.gz" | tar -xz
./prometheus-3.1.0.darwin-${ARCH}/prometheus --config.file=prometheus-local.yaml
```

* **Prometheus Web UI**: [http://localhost:9090](http://localhost:9090)

---

### C. Running the OpenTelemetry Collector on macOS

In native standalone mode, run the local collector in another terminal:
```bash
make run-collector
```
* **Collector Receiver**: `localhost:4317` (gRPC), `localhost:4318` (HTTP)
* **Prometheus Scrape Endpoint**: `http://localhost:8889/metrics`

---

## 🛠️ Complete Native 4-Terminal Workflow on macOS

To run the complete system natively on macOS without Docker:

```
Terminal 1 (Jaeger):     make run-jaeger        -> UI at http://localhost:16686
Terminal 2 (Prometheus): make run-prometheus    -> UI at http://localhost:9090
Terminal 3 (Collector):  make run-collector     -> Metrics at http://localhost:8889/metrics
Terminal 4 (App):        make run-localai-collector (or make run-server)
```

---

## ⚠️ macOS Gotchas & Troubleshooting

### 1. Port 5432 Conflict (Local PostgreSQL)
* **Issue**: Many macOS developers have PostgreSQL running locally via Homebrew (`brew services list`), which locks port `5432`.
* **Solution**: In `docker-compose.yaml`, PostgreSQL is mapped to host port **`5433:5432`**. Always connect to port `5433` for the tutorial database:
  `postgres://postgres:postgrespassword@localhost:5433/ecommerce_db?sslmode=disable`

### 2. Port 5000 / 7000 (macOS AirPlay Receiver)
* **Issue**: macOS Monterey, Ventura, Sonoma, and Sequoia bind the AirPlay Receiver to port `5000` and `7000` by default.
* **Solution**: Neither Jaeger (16686), Prometheus (9090), nor OTel (4317/4318/8889) use these ports, so there is no conflict. If you ever use a custom exporter on port 5000, disable AirPlay in **System Settings ➔ General ➔ AirDrop & AirPlay ➔ AirPlay Receiver**.

### 3. Port 4317 Conflict (Collector vs Jaeger)
* **Issue**: Both the OpenTelemetry Collector and `jaeger-all-in-one` listen on port `4317` (OTLP gRPC) by default.
* **Solution**:
  * In **Docker Compose**, Docker networks isolate container ports; only the Collector binds to host `4317` and forwards to Jaeger internally.
  * In **Native macOS mode**, [`scripts/run-local-jaeger.sh`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/scripts/run-local-jaeger.sh) automatically checks if port `4317` is already taken by `otelcol`. If busy, it runs Jaeger's internal OTLP listener on port `4319` so both run seamlessly side by side.

### 4. macOS Gatekeeper Quarantine
* **Issue**: macOS displays a dialog: *"jaeger-all-in-one cannot be opened because the developer cannot be verified"*.
* **Solution**: Remove the macOS quarantine flag using `xattr`:
  ```bash
  xattr -d com.apple.quarantine bin/* 2>/dev/null || true
  ```
  *(Our helper scripts run this automatically).*

### 5. Apple Silicon Architecture (M1/M2/M3/M4)
* Always ensure binaries are compiled for `darwin-arm64`. Rosetta 2 translation works for `amd64`, but native `arm64` consumes significantly less CPU and memory. Our automated scripts check `uname -m` to select the native architecture.

---

## 📋 Port Reference Table

| Service | Port | Protocol | Purpose / URL |
| :--- | :--- | :--- | :--- |
| **Jaeger UI** | `16686` | HTTP | Web visualization: [http://localhost:16686](http://localhost:16686) |
| **Prometheus UI** | `9090` | HTTP | Time-series query & graph: [http://localhost:9090](http://localhost:9090) |
| **OTel Collector** | `4317` | gRPC | Standard OTLP trace and metric ingestion |
| **OTel Collector** | `4318` | HTTP | Standard OTLP HTTP ingestion |
| **Collector Metrics** | `8889` | HTTP | Prometheus scrape target: [http://localhost:8889/metrics](http://localhost:8889/metrics) |
| **Order API Server**| `8080` | HTTP | Application API: [http://localhost:8080](http://localhost:8080) |
| **Docker PostgreSQL**| `5433` | TCP | Host-mapped database port |

---

## 🔍 Verification Commands

Test your macOS endpoints directly in Terminal:

```bash
# 1. Verify Prometheus Health
curl -s http://localhost:9090/-/healthy
# Response: Prometheus Server is Healthy.

# 2. Verify Jaeger Health
curl -s http://localhost:16686/ | head -n 5
# Response: HTML document title "Jaeger UI"

# 3. Verify OpenTelemetry Collector Metrics Scrape
curl -s http://localhost:8889/metrics | grep -E "app_orders|gen_ai"
```

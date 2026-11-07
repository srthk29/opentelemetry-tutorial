# OpenTelemetry Go Tutorial & Complete Reference

A comprehensive, production-grade tutorial project demonstrating **OpenTelemetry (OTel)** integration in Golang.

---

## 🎯 What This Project Covers

1. **OpenTelemetry Go SDK Integration**:
   - Setting up `Resource` attributes (`service.name`, `service.version`, `deployment.environment`).
   - Configuring `TracerProvider` with `BatchSpanProcessor` or `SimpleSpanProcessor`.
   - Configuring `MeterProvider` with `PeriodicReader` and custom export intervals.
   - Configuring W3C Context Propagators (`TraceContext` and `Baggage`).
   - Proper lifecycle management with graceful shutdown to flush queued telemetry.

2. **Traces Collection**:
   - Inbound HTTP server auto-instrumentation using `otelhttp.NewHandler`.
   - Outbound HTTP client auto-instrumentation with header propagation using `otelhttp.NewTransport`.
   - Manual child spans (`tracer.Start(ctx, ...)`).
   - Custom span attributes (`attribute.String`, `attribute.Float64`, `attribute.Int`).
   - In-span events with timestamps and attributes (`span.AddEvent(...)`).
   - Error recording and status setting (`span.RecordError(err)` and `span.SetStatus(codes.Error, ...)`).
   - Cross-boundary Baggage extraction and propagation (`baggage.FromContext(ctx)`).

3. **Metrics Collection**:
   - **Counter** (`orders.total`): Monotonically increasing counter with dimensions (`status`, `payment_method`, `customer_tier`).
   - **UpDownCounter** (`orders.active`): Tracks in-flight / active processing jobs.
   - **Histogram** (`orders.amount.usd`, `orders.processing_time_ms`): Records value distributions and latency histograms with buckets.
   - **Observable Gauge** (`system.memory.heap_alloc_bytes`, `system.goroutines`): Asynchronous instruments queried periodically via callback.

4. **Collector Integration**:
   - OTLP gRPC (`:4317`) and OTLP HTTP (`:4318`) receivers.
   - Processors: `memory_limiter` and `batch`.
   - Exporters: `debug` (collector console), `otlp/jaeger` (Jaeger tracing backend), and `prometheus` (`:8889/metrics`).
   - Ready-to-use configurations for **both** standalone local execution (no Docker required) and full **Docker Compose** (Jaeger + Prometheus).

---

## 📐 Architecture Overview

```
                      +-----------------------------------+
                      |   Client Simulator                |
                      |   - Service: client-simulator     |
                      |   - otelhttp.Transport (TraceId)  |
                      |   - W3C Baggage ("customer.tier") |
                      +-----------------+-----------------+
                                        | HTTP Request
                                        v
                      +-----------------------------------+
                      |   Order API Server (Golang)       |
                      |   - Service: order-service        |
                      |   - otelhttp.Handler              |
                      |   - Spans: order.process, etc.    |
                      |   - Metrics: Counter, Histograms  |
                      +-----------------+-----------------+
                                        | OTLP (gRPC / :4317)
                                        v
                      +-----------------------------------+
                      |   OpenTelemetry Collector         |
                      |   - Receiver: OTLP gRPC/HTTP      |
                      |   - Processors: memory_limiter,   |
                      |                 batch             |
                      |   - Exporters: debug, jaeger,     |
                      |                prometheus         |
                      +--------+-----------------+--------+
                               |                 |
                   OTLP Traces |                 | Prometheus Scrape
                               v                 v
                 +-------------------+     +-------------------+
                 | Jaeger UI         |     | Prometheus UI     |
                 | (localhost:16686) |     | (localhost:9090)  |
                 +-------------------+     +-------------------+
```

---

## 📂 Project Structure

```
├── cmd/
│   ├── server/main.go               # Instrumented HTTP Server (Order API)
│   ├── client/main.go               # Instrumented Client Simulator (Sends test traffic)
│   ├── pipeline/main.go             # ⚡ Single-Item AI Pipeline (Transcript->Chunking->Embeddings->Summarizer)
│   └── localai/main.go              # 🧠 Local AI Staged Batch Pipeline (Memory-constrained single-model swapping)
├── internal/
│   ├── domain/                      # 🛡️ PURE BUSINESS DOMAIN (ZERO OpenTelemetry imports!)
│   │   ├── order.go                 # Pure domain models, validation, and business logic
│   │   └── order_test.go            # Ultra-fast pure domain unit tests (no OTel setup)
│   ├── pipeline/                    # ⚡ MULTI-STAGE PIPELINE OTel INSTRUMENTATION
│   │   ├── types.go                 # Pipeline interface definition
│   │   ├── request/request.go       # FilterParams & PipelineStage state container
│   │   ├── metrics.go               # Standard Pipeline & GenAI token metrics
│   │   ├── decorator.go             # Generic Telemetry Decorator wrapping ANY Pipeline stage
│   │   ├── orchestrator.go          # Pipeline Orchestrator running stages under parent span
│   │   ├── stages/                  # Pure Stages (ZERO OTel code!)
│   │   │   ├── transcript.go        # Speech-to-text stage
│   │   │   ├── chunking.go          # Text chunking stage
│   │   │   ├── embeddings.go        # Vector embeddings stage
│   │   │   └── summarizer.go        # LLM summarization stage
│   │   ├── modelmanager/            # 🧠 Model Lifecycle & VRAM Resource Management
│   │   │   ├── modelmanager.go      # Single-model residency manager with load/unload spans & VRAM gauge
│   │   │   └── modelmanager_test.go # ModelManager lifecycle tests
│   │   ├── batch/                   # 📦 Memory-Constrained Staged Batch Pipeline
│   │   │   ├── types.go             # DocumentItem, ChunkItem, BatchFilterParams
│   │   │   ├── engines.go           # Pure inference engines (Llama, BGE)
│   │   │   ├── engine_decorators.go # GenAI spans (pipeline.chunk.summarize, pipeline.chunk.embed)
│   │   │   ├── metrics.go           # Batch metrics (stage durations, item counts, token usage)
│   │   │   ├── decorator.go         # BatchTelemetryDecorator (pipeline.stage.<name>.batch parent spans)
│   │   │   ├── orchestrator.go      # Staged Batch Orchestrator
│   │   │   ├── batch_test.go        # Batch pipeline integration tests
│   │   │   └── stages/              # Pure batch stages (chunking, summarizer, embeddings)
│   │   └── pipeline_test.go         # Pipeline integration & unit tests
│   ├── storage/                     # 🗄️ DATABASE IMPLEMENTATIONS & OTel INTEGRATIONS
│   │   ├── sqlite/                  # SQLite (Pure-Go modernc.org/sqlite + otelsql)
│   │   │   ├── sqlite.go
│   │   │   └── sqlite_test.go
│   │   └── postgres/                # PostgreSQL (Both otelsql and native otelpgx!)
│   │       ├── otelsql.go           # Approach A: Unified database/sql + otelsql
│   │       └── otelpgx.go           # Approach B: Native pgxpool.Pool + otelpgx
│   ├── telemetry/                   # 🔌 INFRASTRUCTURE & TELEMETRY ADAPTERS
│   │   ├── telemetry.go             # OTel SDK bootstrap (TracerProvider, MeterProvider, Resource, Shutdown)
│   │   ├── metrics_adapter.go       # Adapter implementing domain.MetricsRecorder port via OTel
│   │   ├── order_decorator.go       # Decorator wrapping pure domain service with tracing & latencies
│   │   └── order_decorator_test.go  # Decorator integration tests
│   └── service/                     # 📚 Alternative Direct-instrumentation reference
│       ├── order.go
│       └── order_test.go
├── scripts/
│   └── run-local-collector.sh       # Script to download & run standalone OTel Collector
├── otel-collector-local.yaml     # Collector configuration for local standalone running
├── otel-collector-config.yaml    # Collector configuration for Docker Compose (with Jaeger & Prometheus)
├── docker-compose.yaml           # Docker Compose orchestration (Collector + Jaeger + Prometheus)
├── prometheus.yaml               # Prometheus scraper configuration
├── Makefile                      # Easy workflow shortcuts
└── go.mod
```

---

## 🚀 Quick Start Guide

You can run this project in **3 different ways** depending on your environment.

### Option A: Standalone Local Collector (No Docker Required! ⭐ Recommended)

This mode runs the official OpenTelemetry Collector binary directly on your machine.

#### Step 1: Start the OpenTelemetry Collector
In Terminal 1:
```bash
make run-collector
```
*(The script will automatically download the standalone `otelcol` binary if not already present in `./bin` and start listening on ports `4317` gRPC and `8889` Prometheus).*

#### Step 2: Start the Go Order API Server
In Terminal 2:
```bash
make run-server
```
*The server connects to `localhost:4317` (Collector) and listens on `http://localhost:8080`.*

#### Step 3: Run the Client Traffic Simulator
In Terminal 3:
```bash
make run-client
```

#### Step 4: Observe the Telemetry!
- **Collector Terminal (Terminal 1)**: You will see formatted spans and metric records printed live by the collector's `debug` exporter.
- **Prometheus Metrics**: Scrape the Collector's Prometheus metrics endpoint:
  ```bash
  curl -s http://localhost:8889/metrics | grep -E "app_orders|app_system"
  ```

---

### Option B: Zero-Dependency Console Mode (Inspect Spans & Metrics directly in Terminal)

If you don't want to run any collector and just want to inspect the raw JSON traces and metric data structures directly in your terminal:

#### Step 1: Start Server in STDOUT mode
In Terminal 1:
```bash
make run-server-stdout
```

#### Step 2: Run Client
In Terminal 2:
```bash
make run-client
```
*Both the client and server will output colorized JSON representations of completed spans and metric points.*

---

### Option C: Full Docker Compose (With Jaeger UI & Prometheus UI)

If you have Docker installed and want to visualize traces in Jaeger UI and metrics in Prometheus:

#### Step 1: Spin up the observability stack
```bash
make docker-up
```
This starts:
- **Jaeger Web UI**: [http://localhost:16686](http://localhost:16686)
- **Prometheus Web UI**: [http://localhost:9090](http://localhost:9090)
- **OTel Collector**: listening on `localhost:4317` (gRPC) & `localhost:4318` (HTTP)

#### Step 2: Start the Go Server
```bash
make run-server
```

#### Step 3: Send Traffic
```bash
make run-client
```

#### Step 4: Explore the UIs
- Open **Jaeger** at [http://localhost:16686](http://localhost:16686):
  - Select Service: `order-service` or `client-simulator`
  - Click **Find Traces** to see end-to-end distributed waterfall traces!
  - Click into an individual trace to view child spans (`order.process` -> `payment.authorize` -> `db.insert_order`) and their span events and attributes.
- Open **Prometheus** at [http://localhost:9090](http://localhost:9090):
  - Query: `app_orders_total`
  - Query: `app_orders_amount_usd_USD_bucket`
  - Query: `app_system_memory_heap_alloc_bytes`

To tear down Docker services:
```bash
make docker-down
```

---

### Option D: Native Standalone on macOS (Zero Docker: Jaeger + Prometheus)

For macOS users who do not have Docker installed or prefer native Apple Silicon / Intel Darwin binaries:

* **Detailed Guide**: [`monitoring/README.md`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/monitoring/README.md) & [`docs/PROMETHEUS_AND_JAEGER_MAC_GUIDE.md`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/docs/PROMETHEUS_AND_JAEGER_MAC_GUIDE.md)

```bash
# Terminal 1: Run native Jaeger All-in-One (UI at http://localhost:16686)
make run-jaeger

# Terminal 2: Run native Prometheus (UI at http://localhost:9090)
make run-prometheus

# Terminal 3: Run OpenTelemetry Collector (:4317 gRPC, :8889 Prometheus metrics)
make run-collector

# Terminal 4: Run application
make run-localai-collector
```

---

## 🛡️ Solving Telemetry Pollution in Business Logic

A common concern when adopting OpenTelemetry is **code pollution**: `tracer.Start()`, `span.SetAttributes()`, and `counter.Add()` calls polluting pure business logic, violating the **Single Responsibility Principle** and making unit testing noisy.

This project demonstrates the industry-standard solution using **Clean / Hexagonal Architecture** and the **Decorator Pattern**:

```
                       HTTP Request
                            │
                            ▼
           ┌───────────────────────────────────┐
           │     otelhttp.NewHandler           │  (Boundary Tracing)
           └────────────────┬──────────────────┘
                            │
                            ▼
           ┌───────────────────────────────────┐
           │    InstrumentedOrderService       │  (Decorator Pattern)
           │    - Starts spans (order.process) │  (Lives in internal/telemetry)
           │    - Injects span attributes      │
           │    - Measures latency histograms  │
           └────────────────┬──────────────────┘
                            │ Calls pure interface
                            ▼
           ┌───────────────────────────────────┐
           │        PureOrderService           │  (Pure Domain Logic)
           │    - Validates order items        │  (Lives in internal/domain)
           │    - Checks card payment limits   │  (ZERO OTel IMPORTS!)
           │    - Saves to repository          │
           └────────────────┬──────────────────┘
                            │ Emits domain metrics via Port
                            ▼
           ┌───────────────────────────────────┐
           │      OTelMetricsAdapter           │  (Ports & Adapters)
           │  Implements MetricsRecorder port  │  (Lives in internal/telemetry)
           └───────────────────────────────────┘
```

### 1. Pure Domain Logic (`internal/domain/order.go`)
Notice: **Zero OpenTelemetry imports**. Pure business rules, zero observability coupling:
```go
type PureOrderService struct {
    repo    OrderRepository
    metrics MetricsRecorder // Domain Port interface
}

func (s *PureOrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    total, err := s.validateItems(req.Items)
    if err != nil {
        s.metrics.RecordOrderFailed(ctx, req.PaymentMethod, req.CustomerTier, "validation_error")
        return nil, err
    }
    // ... pure business rules ...
    s.metrics.RecordOrderCreated(ctx, total, req.PaymentMethod, req.CustomerTier)
    return order, nil
}
```

### 2. Domain Metrics Port (`internal/domain/order.go`)
The domain defines *what* it wants to measure, without knowing *how* it is measured:
```go
type MetricsRecorder interface {
    RecordOrderCreated(ctx context.Context, amount float64, paymentMethod, customerTier string)
    RecordOrderFailed(ctx context.Context, paymentMethod, customerTier, reason string)
}
```

### 3. Telemetry Decorator (`internal/telemetry/order_decorator.go`)
The decorator wraps any `domain.OrderService`, attaching spans and timing without changing one line of domain code:
```go
type InstrumentedOrderService struct {
    next           domain.OrderService
    tracer         trace.Tracer
    metricsAdapter *OTelMetricsAdapter
}

func (s *InstrumentedOrderService) CreateOrder(ctx context.Context, req domain.CreateOrderRequest) (*domain.Order, error) {
    ctx, span := s.tracer.Start(ctx, "order.process",
        trace.WithAttributes(attribute.String("customer.id", req.CustomerID)),
    )
    defer span.End()

    order, err := s.next.CreateOrder(ctx, req) // Calls pure business logic!
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    span.SetStatus(codes.Ok, "order created")
    return order, nil
}
```

### 4. Frictionless Unit Testing (`internal/domain/order_test.go`)
Domain tests run in milliseconds without needing to initialize tracers, meters, exporters, or background collectors:
```go
func TestPureOrderService_Unit(t *testing.T) {
    repo := domain.NewInMemoryOrderRepository()
    // Pass domain.NoopMetricsRecorder{} - no mocks or OTel SDK needed!
    svc := domain.NewPureOrderService(repo, domain.NoopMetricsRecorder{})

    order, err := svc.CreateOrder(context.Background(), validReq)
    assert.NoError(t, err)
    assert.Equal(t, 60.0, order.TotalAmount)
}
```

---

## 🗄️ Database OpenTelemetry Instrumentation (SQLite & PostgreSQL)

The project provides three database implementations for `domain.OrderRepository`, demonstrating how OpenTelemetry traces database calls, multi-statement transactions, and connection pool metrics.

### 1. SQLite (`modernc.org/sqlite` + `otelsql`)
* **Pure-Go Driver**: `modernc.org/sqlite` requires **no CGO** or C compiler.
* **Unified `otelsql` Wrapper**: Wraps the standard Go `database/sql` driver.
* **Normalized Tables**: `orders` and `order_items` tables with foreign keys and multi-statement transaction tracing.
* **Trace Output**: Spans for `sql.tx.begin`, `sql.stmt.exec` (`INSERT INTO orders`), `sql.stmt.exec` (`INSERT INTO order_items`), and `sql.tx.commit`.
* **Connection Pool Metrics**: Tracks open, idle, in-use connections, and wait latencies.

```go
// Open SQLite with OpenTelemetry tracing
db, err := otelsql.Open("sqlite", dsn,
    otelsql.WithAttributes(semconv.DBSystemSqlite, semconv.DBNamespace(dsn)),
    otelsql.WithSQLCommenter(true), // Injects W3C tracecontext into SQL comments!
)
// Register DBStats connection pool metrics with OpenTelemetry
metricReg, err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(semconv.DBSystemSqlite))
```

---

### 2. PostgreSQL Approach A: Unified `database/sql` + `otelsql` (`internal/storage/postgres/otelsql.go`)
* Uses `github.com/jackc/pgx/v5/stdlib` with `otelsql`.
* Identical API experience to SQLite, making multi-database codebases uniform.
* Automatic statement sanitization and parameter logging.

```go
db, err := otelsql.Open("pgx", dsn,
    otelsql.WithAttributes(semconv.DBSystemPostgreSQL, semconv.DBNamespace(dsn)),
    otelsql.WithSQLCommenter(true),
)
```

---

### 3. PostgreSQL Approach B: Native `pgxpool.Pool` + `otelpgx` (`internal/storage/postgres/otelpgx.go`)
* Uses high-performance native `pgxpool.Pool` with `github.com/exaring/otelpgx`.
* Preferred for high-throughput production Go services using native `pgx` features (batching, binary protocol).
* Attaches `otelpgx.NewTracer()` directly to the pgx connection configuration:

```go
config, err := pgxpool.ParseConfig(connString)
// Attach OTel Tracer directly to pgxpool
config.ConnConfig.Tracer = otelpgx.NewTracer(
    otelpgx.WithQuerySpanNamePrefix(), // Formats spans as "DB: SELECT orders", etc.
)
pool, err := pgxpool.NewWithConfig(ctx, config)

// Record native connection pool stats as OpenTelemetry metrics
otelpgx.RecordStats(pool)
```

---

### 🔄 Switching Storage Backends

Switch seamlessly using the `-db` flag or `DB_TYPE` environment variable:

```bash
# In-Memory
make run-server DB_TYPE=memory

# SQLite (default, zero dependencies)
make run-server-sqlite

# PostgreSQL via unified otelsql (Approach A) - connects to Docker on localhost:5433
make run-server-postgres-sql

# PostgreSQL via native otelpgx (Approach B) - connects to Docker on localhost:5433
make run-server-postgres-pgx
```

> [!NOTE]
> **Why Port 5433?**
> On macOS, if you have Homebrew PostgreSQL installed (`brew services start postgresql`), it listens on port `5432` with your Mac username (e.g. `sarthak`) and does not have the `postgres` user.
> To prevent port conflicts, Docker Compose maps container PostgreSQL to **host port 5433** (`5433:5432`) with credentials:
> - **Host**: `localhost:5433`
> - **User**: `postgres`
> - **Password**: `postgrespassword`
> - **Database**: `ecommerce_db`
>
> If you prefer using your local Homebrew PostgreSQL on port 5432 instead, pass your local DSN:
> ```bash
> go run ./cmd/server -db=postgres-sql -db-dsn="postgres://$(whoami)@localhost:5432/postgres?sslmode=disable"
> ```

---

## ⚡ Multi-Stage Pipelines & GenAI Semantic Conventions (`internal/pipeline`)

For multi-stage data processing systems (such as AI, NLP, Audio/Video, or RAG pipelines), each stage typically implements a standard interface:

```go
type Pipeline interface {
    Name() string
    Stage() request.PipelineStage
    Run(context.Context, *request.FilterParams) error
}
```

### 1. Standard OpenTelemetry Signals for Pipelines

| Category | Signal Name | Type | Description / Attributes |
| :--- | :--- | :--- | :--- |
| **Workflow Trace** | `pipeline.workflow: <name>` | Parent Span | Encompasses the entire multi-stage pipeline run. Attributes: `pipeline.workflow.name`, `stage_count`, `request_id`, `document_id`. |
| **Stage Trace** | `pipeline.stage.<stage>` | Child Span | Nested child span for each stage (`pipeline.stage.transcript`, `pipeline.stage.chunking`, etc.). |
| **Workflow Runs** | `pipeline.workflow.runs_total` | Counter | Total workflow executions (`status=success\|failed`, `workflow_name`). |
| **Workflow Latency**| `pipeline.workflow.duration_ms` | Histogram | End-to-end processing latency. |
| **Workflow In-Flight**| `pipeline.workflow.active` | UpDownCounter | Current number of concurrently running pipelines. |
| **Stage Runs** | `pipeline.stage.runs_total` | Counter | Executions per stage (`pipeline.stage`, `status`, `error.type`). |
| **Stage Latency** | `pipeline.stage.duration_ms` | Histogram | Latency distribution per stage (identifies bottlenecks). |
| **Data Volume** | `pipeline.items.processed_total`| Counter | Data items produced (`item.type=words\|chunks\|vectors`). |
| **GenAI Tokens** | `gen_ai.client.token.usage` | Counter | **OTel GenAI Standard**: Token counts (`gen_ai.token.type=input\|output`, `gen_ai.request.model`). |

---

### 2. The Generic Decorator: Zero OTel Code in Stages

Instead of adding OpenTelemetry code into `TranscriptPipeline`, `ChunkingPipeline`, `EmbeddingPipeline`, or `SummarizerPipeline`, we wrap each stage with [`pipeline.Wrap`](file:///Users/sarthak/Documents/code/golang/OpenTelemeteryTutorial/internal/pipeline/decorator.go#L33):

```go
// internal/pipeline/decorator.go
type TelemetryDecorator struct {
    next    Pipeline
    tracer  trace.Tracer
    metrics *Metrics
}

func (d *TelemetryDecorator) Run(ctx context.Context, params *request.FilterParams) error {
    ctx, span := d.tracer.Start(ctx, fmt.Sprintf("pipeline.stage.%s", d.Stage()), ...)
    defer span.End()

    // 1. Execute pure stage business logic
    err := d.next.Run(ctx, params)
    
    // 2. Automatically record errors, durations, and GenAI semantic conventions
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }
    
    // 3. Extract metadata (model, tokens, chunks) and enrich OTel attributes
    d.enrichTelemetry(ctx, span, params)
    return nil
}
```

---

### 3. OpenTelemetry GenAI Semantic Conventions

For embedding and LLM generation stages, the decorator translates stage metadata into **official OpenTelemetry GenAI attributes**:

* **Embeddings Stage (`pipeline.stage.embeddings`)**:
  * `gen_ai.operation.name = "embeddings"`
  * `gen_ai.system = "openai"`
  * `gen_ai.request.model = "text-embedding-3-small"`
  * `gen_ai.usage.input_tokens = 82`
  * `embedding.dimensions = 1536`
  * `embedding.vector_count = 3`
* **Summarizer / Chat Stage (`pipeline.stage.summarizer`)**:
  * `gen_ai.operation.name = "chat"`
  * `gen_ai.system = "google"`
  * `gen_ai.request.model = "gemini-1.5-pro"`
  * `gen_ai.usage.input_tokens = 84`
  * `gen_ai.usage.output_tokens = 46`

---

### 4. Running the Pipeline Demo

```bash
# Option 1: Run directly in terminal (inspect JSON spans & GenAI token metrics)
make run-pipeline

# Option 2: Run with OpenTelemetry Collector
make run-pipeline-collector
```

---

## 🧠 Local AI Memory-Constrained Staged Batch Pipeline

When deploying AI applications on edge hardware or local workstations (using **llama.cpp**, **whisper.cpp**, or local embeddings), system memory and VRAM are strictly limited:

1. **Memory Invariant**: Only **one** heavy model can reside in VRAM/unified memory at a time.
2. **Problem with Item-by-Item Pipelines**: Processing item-by-item forces swapping models back and forth for every single document (Whisper ➔ Llama ➔ BGE ➔ Whisper ➔ Llama ➔ BGE), incurring seconds of SSD-to-VRAM disk I/O latency for each document.
3. **The Solution (Bulk Staged Processing)**:
   - **Chunking Stage**: Runs pure CPU segmentation across **all** documents.
   - **Summarizer Stage**: Loads the 8B LLM (`llama-3-8b-instruct.gguf`, 4.5 GB) into VRAM **once**, summarizes **all** chunks across all documents, and unloads the model.
   - **Embeddings Stage**: Loads the embedding model (`bge-large-en-v1.5.gguf`, 1.3 GB) into VRAM **once**, vectorizes **all** chunks across all documents, and unloads the model.

### 📐 Local AI Tracing Hierarchy

```
pipeline.workflow: local-ai-staged-workflow (Root Trace)
  ├── pipeline.stage.chunking.batch
  │     (pure CPU string segmentation across 3 docs -> 5 chunks)
  ├── pipeline.stage.summarizer.batch
  │     ├── ai.model.load (llama-3-8b-instruct.gguf, 4.5 GB, metal)
  │     ├── pipeline.chunk.summarize [doc-001, chunk-0] (tokens: 38 in, 24 out)
  │     ├── pipeline.chunk.summarize [doc-001, chunk-1] (tokens: 42 in, 28 out)
  │     ├── pipeline.chunk.summarize [doc-002, chunk-0] (tokens: 35 in, 22 out)
  │     ├── ... (all chunks processed in bulk)
  │     └── ai.model.unload (llama-3-8b-instruct.gguf, freed 4.5 GB)
  └── pipeline.stage.embeddings.batch
        ├── ai.model.load (bge-large-en-v1.5.gguf, 1.3 GB, metal)
        ├── pipeline.chunk.embed [doc-001, chunk-0] (dims: 768, input_tokens: 38)
        ├── pipeline.chunk.embed [doc-001, chunk-1] (dims: 768, input_tokens: 42)
        ├── pipeline.chunk.embed [doc-002, chunk-0] (dims: 768, input_tokens: 35)
        ├── ... (all chunks embedded in bulk)
        └── ai.model.unload (bge-large-en-v1.5.gguf, freed 1.3 GB)
```

### 📊 Standard Metrics for Local AI Pipelines

| Metric Name | Type | Description | Key Attributes / Labels |
| :--- | :--- | :--- | :--- |
| `ai.model.vram_allocated_bytes` | UpDownCounter | Current memory/VRAM occupied by active model | `model.name`, `device.type` (metal/cuda/cpu) |
| `ai.model.load_duration_ms` | Histogram | Latency to load model weights into VRAM | `model.name`, `device.type` |
| `ai.model.loads_total` | Counter | Total model load / swap operations | `model.name`, `device.type` |
| `ai.model.unloads_total` | Counter | Total model eviction operations | `model.name`, `device.type` |
| `pipeline.batch.stage.duration_ms` | Histogram | Latency of the bulk batch stage | `pipeline.stage`, `status` |
| `pipeline.batch.chunk.duration_ms` | Histogram | Latency of individual chunk inference | `gen_ai.request.model`, `gen_ai.operation.name` |
| `gen_ai.client.token.usage` | Counter | Input and output token counts | `gen_ai.request.model`, `gen_ai.token.type` (input/output) |

### 🚀 Running the Local AI Demo

```bash
# Run standalone with console STDOUT exporter:
make run-localai

# Run with OpenTelemetry Collector + Jaeger:
make run-localai-collector
```

---

## 🔍 Key Code Walkthrough

### 1. Initializing OpenTelemetry (`internal/telemetry/telemetry.go`)

```go
// 1. Create Resource attributes
res, err := resource.Merge(
    resource.Default(),
    resource.NewSchemaless(
        semconv.ServiceName(cfg.ServiceName),
        semconv.ServiceVersion(cfg.ServiceVersion),
        semconv.DeploymentEnvironment(cfg.Environment),
    ),
)

// 2. Set up W3C Context Propagator
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{}, // W3C traceparent header
    propagation.Baggage{},      // W3C baggage header
))

// 3. Initialize OTLP gRPC Trace Exporter & BatchSpanProcessor
spanExporter, err := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(spanExporter),
    sdktrace.WithResource(res),
)
otel.SetTracerProvider(tp)

// 4. Initialize OTLP gRPC Metric Exporter & PeriodicReader
metricExporter, err := otlpmetricgrpc.New(ctx,
    otlpmetricgrpc.WithEndpoint("localhost:4317"),
    otlpmetricgrpc.WithInsecure(),
)
reader := sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(3*time.Second))
mp := sdkmetric.NewMeterProvider(
    sdkmetric.WithResource(res),
    sdkmetric.WithReader(reader),
)
otel.SetMeterProvider(mp)
```

---

### 2. Tracing: Child Spans, Attributes & Events (`internal/service/order.go`)

```go
// Start a root child span
ctx, span := s.tracer.Start(ctx, "order.process",
    trace.WithAttributes(
        attribute.String("customer.id", req.CustomerID),
        attribute.String("payment.method", req.PaymentMethod),
    ),
)
defer span.End()

// Add child span for payment
ctx, paymentSpan := s.tracer.Start(ctx, "payment.authorize")
defer paymentSpan.End()

// Add Span Event (milestone inside a span)
paymentSpan.AddEvent("payment_authorized", trace.WithAttributes(
    attribute.String("payment.transaction_id", "tx-12345"),
))

// Handle errors
if err != nil {
    paymentSpan.RecordError(err)
    paymentSpan.SetStatus(codes.Error, "payment gateway declined transaction")
    return err
}
paymentSpan.SetStatus(codes.Ok, "payment authorized")
```

---

### 3. Metrics: Synchronous & Asynchronous Instruments (`internal/service/order.go`)

```go
meter := otel.Meter("order-service")

// Synchronous Counter
counter, _ := meter.Int64Counter("orders.total", metric.WithDescription("Total orders created"))
counter.Add(ctx, 1, metric.WithAttributes(
    attribute.String("status", "success"),
    attribute.String("payment_method", "credit_card"),
))

// Synchronous UpDownCounter (active tasks)
inFlight, _ := meter.Int64UpDownCounter("orders.active")
inFlight.Add(ctx, 1)
defer inFlight.Add(ctx, -1)

// Synchronous Histogram (latency / distribution)
hist, _ := meter.Float64Histogram("orders.amount.usd")
hist.Record(ctx, totalAmount)

// Asynchronous Observable Gauge (runtime stats)
meter.Int64ObservableGauge("system.memory.heap_alloc_bytes",
    metric.WithInt64Callback(func(ctx context.Context, obs metric.Int64Observer) error {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        obs.Observe(int64(m.HeapAlloc))
        return nil
    }),
)
```

---

### 4. OpenTelemetry Collector Pipeline (`otel-collector-config.yaml`)

```yaml
receivers:
  otlp:
    protocols:
      grpc: { endpoint: 0.0.0.0:4317 }
      http: { endpoint: 0.0.0.0:4318 }

processors:
  batch: { timeout: 1s, send_batch_size: 256 }
  memory_limiter: { check_interval: 1s, limit_percentage: 75 }

exporters:
  debug: { verbosity: detailed }
  otlp/jaeger: { endpoint: jaeger:4317, tls: { insecure: true } }
  prometheus: { endpoint: "0.0.0.0:8889", namespace: "app" }

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug, otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug, prometheus]
```

---

## 🧪 Testing and Verification

To verify that the entire pipeline is working:
```bash
# 1. Build project
make build

# 2. Check health endpoint
curl http://localhost:8080/health

# 3. Create a manual order
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "cust-demo-1",
    "payment_method": "credit_card",
    "items": [{"sku": "KEYBOARD-RGB", "quantity": 1, "price": 120.00}]
  }'
```

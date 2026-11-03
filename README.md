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
│   └── client/main.go               # Instrumented Client Simulator (Sends test traffic)
├── internal/
│   ├── domain/                      # 🛡️ PURE BUSINESS DOMAIN (ZERO OpenTelemetry imports!)
│   │   ├── order.go                 # Pure domain models, validation, and business logic
│   │   └── order_test.go            # Ultra-fast pure domain unit tests (no OTel setup)
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

# SQLite (default)
make run-server-sqlite

# PostgreSQL via unified otelsql (Approach A)
make run-server-postgres-sql

# PostgreSQL via native otelpgx (Approach B)
make run-server-postgres-pgx
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

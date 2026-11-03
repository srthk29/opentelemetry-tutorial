package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	"github.com/sarthak/opentelemetry-tutorial/internal/storage/postgres"
	"github.com/sarthak/opentelemetry-tutorial/internal/storage/sqlite"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// 1. Command-line flags and environment defaults
	port := flag.Int("port", 8080, "HTTP server port")
	exporterType := flag.String("exporter", "otlp", "Telemetry exporter: 'otlp' (Collector) or 'stdout' (Console)")
	collectorEndpoint := flag.String("collector-addr", "localhost:4317", "OTLP gRPC collector address")
	serviceName := flag.String("service-name", "order-service", "Service name for OpenTelemetry resource")
	dbType := flag.String("db", "sqlite", "Storage backend: 'sqlite', 'postgres-sql', 'postgres-pgx', or 'memory'")
	dbDSN := flag.String("db-dsn", "", "Database DSN/connection string")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", port)
	}
	if envExp := os.Getenv("OTEL_EXPORTER_TYPE"); envExp != "" {
		*exporterType = envExp
	}
	if envCollector := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); envCollector != "" {
		*collectorEndpoint = envCollector
	}
	if envSvc := os.Getenv("OTEL_SERVICE_NAME"); envSvc != "" {
		*serviceName = envSvc
	}
	if envDB := os.Getenv("DB_TYPE"); envDB != "" {
		*dbType = envDB
	}
	if envDSN := os.Getenv("DB_DSN"); envDSN != "" {
		*dbDSN = envDSN
	}

	log.Printf("Starting %s with exporter=%s, collector=%s, port=%d, db=%s\n",
		*serviceName, *exporterType, *collectorEndpoint, *port, *dbType)

	// 2. Initialize OpenTelemetry SDK (TracerProvider, MeterProvider, Propagators)
	ctx := context.Background()
	cfg := telemetry.DefaultConfig(*serviceName)
	cfg.ExporterType = *exporterType
	cfg.OTLPEndpoint = *collectorEndpoint

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}

	// 3. Initialize Storage Backend (with OpenTelemetry instrumentation)
	var orderRepo domain.OrderRepository
	switch *dbType {
	case "sqlite":
		dsn := *dbDSN
		if dsn == "" {
			dsn = "orders.db"
		}
		repo, err := sqlite.New(ctx, dsn)
		if err != nil {
			log.Fatalf("Failed to initialize SQLite repository: %v", err)
		}
		defer repo.Close()
		orderRepo = repo
		log.Printf("✓ Storage: SQLite (otelsql) connected -> %s\n", dsn)

	case "postgres-sql":
		dsn := *dbDSN
		if dsn == "" {
			dsn = "postgres://postgres:postgrespassword@localhost:5433/ecommerce_db?sslmode=disable"
		}
		repo, err := postgres.NewSQLRepository(ctx, dsn)
		if err != nil {
			log.Fatalf("Failed to initialize Postgres SQL repository: %v", err)
		}
		defer repo.Close()
		orderRepo = repo
		log.Printf("✓ Storage: PostgreSQL (Approach A: otelsql + pgx/stdlib) connected -> %s\n", dsn)

	case "postgres-pgx":
		dsn := *dbDSN
		if dsn == "" {
			dsn = "postgres://postgres:postgrespassword@localhost:5433/ecommerce_db?sslmode=disable"
		}
		repo, err := postgres.NewPGXPoolRepository(ctx, dsn)
		if err != nil {
			log.Fatalf("Failed to initialize Postgres pgxpool repository: %v", err)
		}
		defer repo.Close()
		orderRepo = repo
		log.Printf("✓ Storage: PostgreSQL (Approach B: native pgxpool + otelpgx) connected -> %s\n", dsn)

	case "memory":
		fallthrough
	default:
		orderRepo = domain.NewInMemoryOrderRepository()
		log.Println("✓ Storage: In-Memory repository")
	}

	// 4. Clean Architecture Composition:
	// A. Create Metrics Adapter (implements domain.MetricsRecorder port)
	metricsAdapter, err := telemetry.NewOTelMetricsAdapter()
	if err != nil {
		log.Fatalf("Failed to create metrics adapter: %v", err)
	}

	// B. Create Pure Domain Service (100% Free of OpenTelemetry imports!)
	pureOrderService := domain.NewPureOrderService(orderRepo, metricsAdapter)

	// C. Wrap with Telemetry Decorator (Decorator Pattern)
	var orderService domain.OrderService = telemetry.NewInstrumentedOrderService(pureOrderService, metricsAdapter)

	// 5. Configure HTTP Routes
	mux := http.NewServeMux()

	// Health Check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})

	// Create Order: POST /orders
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("http.route", "/orders"))

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed. Use POST /orders", http.StatusMethodNotAllowed)
			return
		}

		var req domain.CreateOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			span.RecordError(err)
			http.Error(w, fmt.Sprintf("Invalid JSON body: %v", err), http.StatusBadRequest)
			return
		}

		order, err := orderService.CreateOrder(r.Context(), req)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, domain.ErrInvalidOrderAmount) || errors.Is(err, domain.ErrEmptyOrderItems) {
				status = http.StatusBadRequest
			} else if errors.Is(err, domain.ErrPaymentFailed) {
				status = http.StatusPaymentRequired
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(order)
	})

	// Get Order: GET /orders/{id}
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(attribute.String("http.route", "/orders/{id}"))

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed. Use GET /orders/{id}", http.StatusMethodNotAllowed)
			return
		}

		orderID := strings.TrimPrefix(r.URL.Path, "/orders/")
		if orderID == "" {
			http.Error(w, "Missing order ID in path", http.StatusBadRequest)
			return
		}

		order, err := orderService.GetOrder(r.Context(), orderID)
		if err != nil {
			if errors.Is(err, domain.ErrOrderNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(order)
	})

	// 6. Wrap HTTP Handler with otelhttp to automatically trace inbound HTTP requests
	otelHandler := otelhttp.NewHandler(mux, "http.server",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      otelHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 7. Graceful Server Lifecycle Management
	serverErrChan := make(chan error, 1)
	go func() {
		log.Printf("Order API server listening on http://localhost:%d\n", *port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- err
		}
	}()

	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrChan:
		log.Printf("Server failed: %v", err)
	case sig := <-stopSig:
		log.Printf("Received signal '%v', initiating graceful shutdown...", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("HTTP server stopped gracefully.")
	}

	telemetryShutdownCtx, cancelTele := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTele()

	if err := shutdownTelemetry(telemetryShutdownCtx); err != nil {
		log.Printf("OpenTelemetry shutdown error: %v", err)
	} else {
		log.Println("OpenTelemetry telemetry flushed successfully.")
	}

	log.Println("Service exited.")
}

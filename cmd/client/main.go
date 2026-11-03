package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/service"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Target server base URL")
	exporterType := flag.String("exporter", "otlp", "Telemetry exporter: 'otlp' or 'stdout'")
	collectorEndpoint := flag.String("collector-addr", "localhost:4317", "OTLP collector address")
	flag.Parse()

	ctx := context.Background()

	// Initialize OpenTelemetry for client to demonstrate distributed cross-service tracing
	cfg := telemetry.DefaultConfig("client-simulator")
	cfg.ExporterType = *exporterType
	cfg.OTLPEndpoint = *collectorEndpoint

	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	// HTTP Client wrapped with OpenTelemetry Transport.
	// This automatically propagates traceparent and baggage HTTP headers!
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	tracer := otel.Tracer("client-simulator")

	log.Println("==================================================")
	log.Println("Starting OpenTelemetry Client Traffic Simulator")
	log.Println("==================================================")

	// Test 1: Successful Order with Gold Customer Tier Baggage
	runScenario(ctx, tracer, client, *serverURL, scenario{
		name:         "Scenario 1: Standard Order (Tier: Gold)",
		customerID:   "cust-gold-001",
		customerTier: "gold",
		method:       "credit_card",
		items: []service.Item{
			{SKU: "LAPTOP-PRO", Quantity: 1, Price: 1200.00},
			{SKU: "USB-HUB", Quantity: 2, Price: 45.00},
		},
	})

	time.Sleep(500 * time.Millisecond)

	// Test 2: Successful Small Order (Cash)
	runScenario(ctx, tracer, client, *serverURL, scenario{
		name:         "Scenario 2: Quick Coffee Order (Cash)",
		customerID:   "cust-walkin-99",
		customerTier: "standard",
		method:       "cash",
		items: []service.Item{
			{SKU: "COFFEE-ESPRESSO", Quantity: 2, Price: 4.50},
			{SKU: "CROISSANT", Quantity: 1, Price: 3.50},
		},
	})

	time.Sleep(500 * time.Millisecond)

	// Test 3: Large Order that triggers Payment Decline Error in Service
	runScenario(ctx, tracer, client, *serverURL, scenario{
		name:         "Scenario 3: Over-limit Order (Simulates Error Tracing)",
		customerID:   "cust-fraud-risk",
		customerTier: "standard",
		method:       "credit_card",
		items: []service.Item{
			{SKU: "SERVER-RACK-ENTERPRISE", Quantity: 1, Price: 9500.00},
		},
	})

	time.Sleep(500 * time.Millisecond)

	// Test 4: Order Not Found (Simulates 404 Error Trace)
	runScenarioGetNotFound(ctx, tracer, client, *serverURL)

	log.Println("\nSimulation completed! Telemetry is flushing to Collector/Console...")
}

type scenario struct {
	name         string
	customerID   string
	customerTier string
	method       string
	items        []service.Item
}

func runScenario(ctx context.Context, tracer trace.Tracer, client *http.Client, serverURL string, sc scenario) {
	// Create W3C Baggage member
	tierMember, _ := baggage.NewMember("customer.tier", sc.customerTier)
	bag, _ := baggage.New(tierMember)
	ctx = baggage.ContextWithBaggage(ctx, bag)

	// Start Client Root Span
	ctx, span := tracer.Start(ctx, fmt.Sprintf("client_scenario: %s", sc.name))
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	log.Printf("\n▶ %s [TraceID: %s]\n", sc.name, traceID)

	reqPayload := service.CreateOrderRequest{
		CustomerID:    sc.customerID,
		Items:         sc.items,
		PaymentMethod: sc.method,
	}
	body, _ := json.Marshal(reqPayload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/orders", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("  Error creating request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	log.Printf("  Server HTTP Status: %d\n", resp.StatusCode)
	log.Printf("  Server Response: %s\n", string(respBytes))

	// If successful, test retrieving the created order via GET /orders/{id}
	if resp.StatusCode == http.StatusCreated {
		var created service.Order
		if err := json.Unmarshal(respBytes, &created); err == nil {
			getOrderReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/orders/%s", serverURL, created.ID), nil)
			getResp, err := client.Do(getOrderReq)
			if err == nil {
				defer getResp.Body.Close()
				log.Printf("  Verified via GET /orders/%s -> Status: %d\n", created.ID, getResp.StatusCode)
			}
		}
	}
}

func runScenarioGetNotFound(ctx context.Context, tracer trace.Tracer, client *http.Client, serverURL string) {
	ctx, span := tracer.Start(ctx, "client_scenario: Fetch Unknown Order")
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	log.Printf("\n▶ Scenario 4: Request Non-Existent Order [TraceID: %s]\n", traceID)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/orders/unknown-order-9999", nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("  Server HTTP Status: %d (Expected 404)\n", resp.StatusCode)
	log.Printf("  Server Response: %s\n", string(body))
}

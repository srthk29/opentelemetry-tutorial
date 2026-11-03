package telemetry_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
	"go.opentelemetry.io/otel/baggage"
)

func TestInstrumentedOrderService_Decorator(t *testing.T) {
	ctx := context.Background()

	// Initialize stdout telemetry for test
	cfg := telemetry.DefaultConfig("test-decorator-service")
	cfg.ExporterType = "stdout"
	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to setup telemetry: %v", err)
	}
	defer func() {
		_ = shutdown(ctx)
	}()

	// 1. Create Metrics Adapter (implements domain.MetricsRecorder port)
	metricsAdapter, err := telemetry.NewOTelMetricsAdapter()
	if err != nil {
		t.Fatalf("Failed to create metrics adapter: %v", err)
	}

	// 2. Create Pure Domain Service (100% telemetry free!)
	repo := domain.NewInMemoryOrderRepository()
	pureService := domain.NewPureOrderService(repo, metricsAdapter)

	// 3. Wrap Pure Domain Service with Telemetry Decorator
	instrumentedService := telemetry.NewInstrumentedOrderService(pureService, metricsAdapter)

	t.Run("Create valid order with baggage propagation", func(t *testing.T) {
		// Inject baggage
		member, _ := baggage.NewMember("customer.tier", "platinum")
		bag, _ := baggage.New(member)
		reqCtx := baggage.ContextWithBaggage(ctx, bag)

		req := domain.CreateOrderRequest{
			CustomerID:    "cust-decor-1",
			PaymentMethod: "credit_card",
			Items: []domain.Item{
				{SKU: "HEADPHONES", Quantity: 1, Price: 150.0},
			},
		}

		order, err := instrumentedService.CreateOrder(reqCtx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if order.CustomerTier != "platinum" {
			t.Errorf("Expected tier 'platinum' from baggage, got %s", order.CustomerTier)
		}
		if order.TotalAmount != 150.0 {
			t.Errorf("Expected total 150.0, got %f", order.TotalAmount)
		}

		// Verify retrieval through decorator
		fetched, err := instrumentedService.GetOrder(ctx, order.ID)
		if err != nil {
			t.Fatalf("Failed to get order: %v", err)
		}
		if fetched.ID != order.ID {
			t.Errorf("Expected ID %s, got %s", order.ID, fetched.ID)
		}
	})

	t.Run("Decorator handles error from pure domain service", func(t *testing.T) {
		req := domain.CreateOrderRequest{
			CustomerID:    "cust-decor-2",
			PaymentMethod: "credit_card",
			Items: []domain.Item{
				{SKU: "VERY-EXPENSIVE", Quantity: 1, Price: 8000.0},
			},
		}

		_, err := instrumentedService.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected error for amount > 5000, got nil")
		}
	})
}

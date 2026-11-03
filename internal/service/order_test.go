package service_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/service"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
)

func TestOrderService(t *testing.T) {
	ctx := context.Background()

	// Initialize stdout telemetry for test
	cfg := telemetry.DefaultConfig("test-order-service")
	cfg.ExporterType = "stdout"
	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to setup telemetry: %v", err)
	}
	defer func() {
		_ = shutdown(ctx)
	}()

	svc, err := service.NewOrderService()
	if err != nil {
		t.Fatalf("Failed to create OrderService: %v", err)
	}

	t.Run("Create valid order", func(t *testing.T) {
		req := service.CreateOrderRequest{
			CustomerID:    "cust-test-1",
			PaymentMethod: "credit_card",
			Items: []service.Item{
				{SKU: "TEST-ITEM", Quantity: 2, Price: 25.0},
			},
		}

		order, err := svc.CreateOrder(ctx, req)
		if err != nil {
			t.Fatalf("Expected successful order, got error: %v", err)
		}
		if order.TotalAmount != 50.0 {
			t.Errorf("Expected total amount 50.0, got %f", order.TotalAmount)
		}
		if order.Status != "COMPLETED" {
			t.Errorf("Expected COMPLETED status, got %s", order.Status)
		}

		fetched, err := svc.GetOrder(ctx, order.ID)
		if err != nil {
			t.Fatalf("Failed to fetch order: %v", err)
		}
		if fetched.ID != order.ID {
			t.Errorf("Expected fetched ID %s, got %s", order.ID, fetched.ID)
		}
	})

	t.Run("Invalid order item price", func(t *testing.T) {
		req := service.CreateOrderRequest{
			CustomerID:    "cust-test-2",
			PaymentMethod: "cash",
			Items: []service.Item{
				{SKU: "BAD-ITEM", Quantity: 1, Price: -10.0},
			},
		}

		_, err := svc.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected error for negative price, got nil")
		}
	})

	t.Run("Payment declined simulation for order > 5000", func(t *testing.T) {
		req := service.CreateOrderRequest{
			CustomerID:    "cust-test-3",
			PaymentMethod: "credit_card",
			Items: []service.Item{
				{SKU: "EXPENSIVE-ITEM", Quantity: 1, Price: 6000.0},
			},
		}

		_, err := svc.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected payment declined error, got nil")
		}
	})
}

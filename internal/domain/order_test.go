package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
)

func TestPureOrderService_Unit(t *testing.T) {
	ctx := context.Background()

	// Notice: ZERO OpenTelemetry setup required!
	// We can pass domain.NoopMetricsRecorder{} directly.
	repo := domain.NewInMemoryOrderRepository()
	svc := domain.NewPureOrderService(repo, domain.NoopMetricsRecorder{})

	t.Run("Successfully creates order", func(t *testing.T) {
		req := domain.CreateOrderRequest{
			CustomerID:    "cust-123",
			PaymentMethod: "credit_card",
			Items: []domain.Item{
				{SKU: "BOOK-GO", Quantity: 2, Price: 30.0},
			},
		}

		order, err := svc.CreateOrder(ctx, req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if order.TotalAmount != 60.0 {
			t.Errorf("Expected 60.0, got %f", order.TotalAmount)
		}
		if order.Status != "COMPLETED" {
			t.Errorf("Expected COMPLETED, got %s", order.Status)
		}
	})

	t.Run("Rejects empty items", func(t *testing.T) {
		req := domain.CreateOrderRequest{
			CustomerID:    "cust-123",
			PaymentMethod: "cash",
			Items:         []domain.Item{},
		}

		_, err := svc.CreateOrder(ctx, req)
		if !errors.Is(err, domain.ErrEmptyOrderItems) {
			t.Fatalf("Expected ErrEmptyOrderItems, got: %v", err)
		}
	})

	t.Run("Rejects negative price", func(t *testing.T) {
		req := domain.CreateOrderRequest{
			CustomerID:    "cust-123",
			PaymentMethod: "cash",
			Items: []domain.Item{
				{SKU: "BAD-ITEM", Quantity: 1, Price: -5.0},
			},
		}

		_, err := svc.CreateOrder(ctx, req)
		if !errors.Is(err, domain.ErrInvalidOrderAmount) {
			t.Fatalf("Expected ErrInvalidOrderAmount, got: %v", err)
		}
	})

	t.Run("Declines over-limit payment", func(t *testing.T) {
		req := domain.CreateOrderRequest{
			CustomerID:    "cust-123",
			PaymentMethod: "credit_card",
			Items: []domain.Item{
				{SKU: "EXPENSIVE-MACBOOK", Quantity: 2, Price: 3000.0}, // total 6000 > 5000
			},
		}

		_, err := svc.CreateOrder(ctx, req)
		if !errors.Is(err, domain.ErrPaymentFailed) {
			t.Fatalf("Expected ErrPaymentFailed, got: %v", err)
		}
	})
}

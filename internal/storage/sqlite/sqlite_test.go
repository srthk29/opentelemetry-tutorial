package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	"github.com/sarthak/opentelemetry-tutorial/internal/storage/sqlite"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
)

func TestSQLiteRepository(t *testing.T) {
	ctx := context.Background()

	// Initialize stdout telemetry for test
	cfg := telemetry.DefaultConfig("test-sqlite")
	cfg.ExporterType = "stdout"
	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to setup telemetry: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	// Use in-memory SQLite database
	repo, err := sqlite.New(ctx, "file:testorders?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to initialize sqlite repo: %v", err)
	}
	defer repo.Close()

	now := time.Now().Truncate(time.Second)
	testOrder := &domain.Order{
		ID:            "ord-sql-001",
		CustomerID:    "cust-sqlite",
		CustomerTier:  "gold",
		TotalAmount:   299.99,
		PaymentMethod: "credit_card",
		Status:        "COMPLETED",
		CreatedAt:     now,
		Items: []domain.Item{
			{SKU: "KEYBOARD-MECH", Quantity: 1, Price: 199.99},
			{SKU: "WRIST-REST", Quantity: 2, Price: 50.00},
		},
	}

	t.Run("Save and FindByID", func(t *testing.T) {
		if err := repo.Save(ctx, testOrder); err != nil {
			t.Fatalf("Failed to save order: %v", err)
		}

		fetched, err := repo.FindByID(ctx, testOrder.ID)
		if err != nil {
			t.Fatalf("Failed to fetch order: %v", err)
		}

		if fetched.ID != testOrder.ID {
			t.Errorf("Expected ID %s, got %s", testOrder.ID, fetched.ID)
		}
		if fetched.TotalAmount != testOrder.TotalAmount {
			t.Errorf("Expected total %f, got %f", testOrder.TotalAmount, fetched.TotalAmount)
		}
		if len(fetched.Items) != 2 {
			t.Fatalf("Expected 2 items, got %d", len(fetched.Items))
		}
		if fetched.Items[0].SKU != "KEYBOARD-MECH" {
			t.Errorf("Expected first item SKU KEYBOARD-MECH, got %s", fetched.Items[0].SKU)
		}
	})

	t.Run("FindByID returns ErrOrderNotFound", func(t *testing.T) {
		_, err := repo.FindByID(ctx, "non-existent-order")
		if !errors.Is(err, domain.ErrOrderNotFound) {
			t.Fatalf("Expected ErrOrderNotFound, got: %v", err)
		}
	})
}

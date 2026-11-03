package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
)

// PGXPoolRepository implements domain.OrderRepository using native pgxpool.Pool
// instrumented via the otelpgx tracer plugin.
//
// LEARNING FOCUS (Approach B):
// Many high-throughput Go services prefer the native pgx driver over database/sql
// for its advanced PostgreSQL features (batching, binary format, copy protocol).
// This demonstrates how to instrument native pgx connections using otelpgx.NewTracer().
type PGXPoolRepository struct {
	pool *pgxpool.Pool
}

// Compile-time check that PGXPoolRepository satisfies domain.OrderRepository.
var _ domain.OrderRepository = (*PGXPoolRepository)(nil)

// NewPGXPoolRepository connects to PostgreSQL using pgxpool instrumented with otelpgx.
func NewPGXPoolRepository(ctx context.Context, connString string) (*PGXPoolRepository, error) {
	// 1. Parse connection string
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	// 2. Attach OpenTelemetry Tracer to pgx connection configuration
	// otelpgx automatically intercepts Query, Exec, Prepare, Batch, and Transactions!
	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithQuerySpanNamePrefix(), // Formats spans as "DB: SELECT orders", etc.
	)

	// 3. Create Connection Pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	// 4. Ping to verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping pgxpool: %w", err)
	}

	// 5. Register native connection pool metrics (acquiring, idle, total conns) with OpenTelemetry
	if err := otelpgx.RecordStats(pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to record pgx stats: %w", err)
	}

	repo := &PGXPoolRepository{
		pool: pool,
	}

	// 6. Initialize schema
	if err := repo.migrate(ctx); err != nil {
		repo.Close()
		return nil, fmt.Errorf("failed to migrate pgx schema: %w", err)
	}

	return repo, nil
}

// Close closes the pgx connection pool.
func (r *PGXPoolRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

func (r *PGXPoolRepository) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(64) PRIMARY KEY,
			customer_id VARCHAR(64) NOT NULL,
			customer_tier VARCHAR(32) NOT NULL,
			total_amount NUMERIC(12, 2) NOT NULL,
			payment_method VARCHAR(32) NOT NULL,
			status VARCHAR(32) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS order_items (
			order_id VARCHAR(64) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			sku VARCHAR(64) NOT NULL,
			quantity INT NOT NULL,
			price NUMERIC(10, 2) NOT NULL,
			PRIMARY KEY (order_id, sku)
		);`,
	}

	for _, query := range queries {
		if _, err := r.pool.Exec(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// Save persists an order and items using native pgx transaction.
func (r *PGXPoolRepository) Save(ctx context.Context, order *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin pgx tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert Order
	orderQuery := `
		INSERT INTO orders (id, customer_id, customer_tier, total_amount, payment_method, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, orderQuery,
		order.ID,
		order.CustomerID,
		order.CustomerTier,
		order.TotalAmount,
		order.PaymentMethod,
		order.Status,
		order.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// 2. Insert Items
	itemQuery := `
		INSERT INTO order_items (order_id, sku, quantity, price)
		VALUES ($1, $2, $3, $4)
	`
	for _, it := range order.Items {
		if _, err := tx.Exec(ctx, itemQuery, order.ID, it.SKU, it.Quantity, it.Price); err != nil {
			return fmt.Errorf("failed to insert item %s: %w", it.SKU, err)
		}
	}

	// 3. Commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit pgx tx: %w", err)
	}

	return nil
}

// FindByID retrieves an order and items by ID.
func (r *PGXPoolRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, customer_id, customer_tier, total_amount, payment_method, status, created_at
		FROM orders WHERE id = $1
	`, id)

	var o domain.Order
	err := row.Scan(
		&o.ID,
		&o.CustomerID,
		&o.CustomerTier,
		&o.TotalAmount,
		&o.PaymentMethod,
		&o.Status,
		&o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT sku, quantity, price
		FROM order_items WHERE order_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.SKU, &it.Quantity, &it.Price); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		o.Items = append(o.Items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading item rows: %w", err)
	}

	return &o, nil
}

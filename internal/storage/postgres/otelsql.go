package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib" // Registers "pgx" driver for database/sql
	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SQLRepository implements domain.OrderRepository for PostgreSQL using the
// standard database/sql package instrumented via otelsql.
//
// LEARNING FOCUS (Approach A):
// This demonstrates how to instrument PostgreSQL identically to SQLite using the
// unified otelsql wrapper. Any standard database/sql driver (pgx/stdlib, lib/pq)
// benefits from automatic query spans, transaction spans, and DB connection pool metrics.
type SQLRepository struct {
	db     *sql.DB
	metric interface{ Unregister() error }
}

// Ensure SQLRepository implements domain.OrderRepository at compile time.
var _ domain.OrderRepository = (*SQLRepository)(nil)

// NewSQLRepository connects to PostgreSQL using pgx/stdlib wrapped with otelsql.
func NewSQLRepository(ctx context.Context, dsn string) (*SQLRepository, error) {
	// 1. Open PostgreSQL database wrapped with OpenTelemetry driver
	db, err := otelsql.Open("pgx", dsn,
		otelsql.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBNamespace(dsn),
		),
		otelsql.WithSQLCommenter(true), // Injects W3C tracecontext into SQL query comments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open otelsql postgres: %w", err)
	}

	// 2. Ping to verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	// 3. Register database/sql connection pool metrics with OpenTelemetry
	metricReg, err := otelsql.RegisterDBStatsMetrics(db,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to register db stats metrics: %w", err)
	}

	repo := &SQLRepository{
		db:     db,
		metric: metricReg,
	}

	// 4. Initialize schema
	if err := repo.migrate(ctx); err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to migrate postgres schema: %w", err)
	}

	return repo, nil
}

// Close closes the database connection and unregisters OTel metrics.
func (r *SQLRepository) Close() error {
	var errs []error
	if r.metric != nil {
		if err := r.metric.Unregister(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *SQLRepository) migrate(ctx context.Context) error {
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
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// Save persists an order and items using a multi-statement transaction.
func (r *SQLRepository) Save(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert order record (Postgres uses $1, $2... parameter placeholders)
	orderQuery := `
		INSERT INTO orders (id, customer_id, customer_tier, total_amount, payment_method, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, orderQuery,
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

	// 2. Insert items
	itemStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO order_items (order_id, sku, quantity, price)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare items stmt: %w", err)
	}
	defer itemStmt.Close()

	for _, it := range order.Items {
		if _, err := itemStmt.ExecContext(ctx, order.ID, it.SKU, it.Quantity, it.Price); err != nil {
			return fmt.Errorf("failed to insert item %s: %w", it.SKU, err)
		}
	}

	// 3. Commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}

// FindByID retrieves an order and its items by ID.
func (r *SQLRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
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

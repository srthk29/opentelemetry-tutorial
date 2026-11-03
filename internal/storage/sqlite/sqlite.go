package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	_ "modernc.org/sqlite" // Pure-Go SQLite driver (no CGO required)
)

// Repository implements domain.OrderRepository backed by SQLite with OpenTelemetry tracing.
type Repository struct {
	db     *sql.DB
	metric interface{ Unregister() error }
}

// Compile-time assertion that Repository satisfies domain.OrderRepository.
var _ domain.OrderRepository = (*Repository)(nil)

// New creates a new SQLite-backed order repository instrumented with OpenTelemetry.
// It automatically records query spans, transaction spans, and connection pool metrics.
func New(ctx context.Context, dsn string) (*Repository, error) {
	// 1. Open SQLite database wrapped with OpenTelemetry SQL driver
	db, err := otelsql.Open("sqlite", dsn,
		otelsql.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBNamespace(dsn),
		),
		otelsql.WithSQLCommenter(true), // Appends W3C trace context into SQL query comments!
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open otelsql sqlite: %w", err)
	}

	// 2. Ping database to verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// 3. Register DB connection pool stats as OpenTelemetry metrics
	// (tracks open connections, idle connections, in-use connections, and wait duration)
	metricReg, err := otelsql.RegisterDBStatsMetrics(db,
		otelsql.WithAttributes(semconv.DBSystemSqlite),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to register db stats metrics: %w", err)
	}

	repo := &Repository{
		db:     db,
		metric: metricReg,
	}

	// 4. Initialize schema (orders & normalized order_items)
	if err := repo.migrate(ctx); err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to migrate sqlite schema: %w", err)
	}

	return repo, nil
}

// Close closes the database connection and unregisters connection pool metrics.
func (r *Repository) Close() error {
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

func (r *Repository) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			customer_id TEXT NOT NULL,
			customer_tier TEXT NOT NULL,
			total_amount REAL NOT NULL,
			payment_method TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS order_items (
			order_id TEXT NOT NULL,
			sku TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			price REAL NOT NULL,
			PRIMARY KEY (order_id, sku),
			FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
		);`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// Save persists an order across a multi-statement transaction (orders + order_items).
// otelsql automatically creates spans for:
//   - sql.tx.begin
//   - sql.stmt.exec (INSERT INTO orders)
//   - sql.stmt.exec (INSERT INTO order_items)
//   - sql.tx.commit
func (r *Repository) Save(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert order record
	orderQuery := `
		INSERT INTO orders (id, customer_id, customer_tier, total_amount, payment_method, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
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

	// 2. Insert order items
	itemStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO order_items (order_id, sku, quantity, price)
		VALUES (?, ?, ?, ?)
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

	// 3. Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FindByID retrieves an order and its items by ID.
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, customer_id, customer_tier, total_amount, payment_method, status, created_at
		FROM orders WHERE id = ?
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

	// Fetch items
	rows, err := r.db.QueryContext(ctx, `
		SELECT sku, quantity, price
		FROM order_items WHERE order_id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query order items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.SKU, &it.Quantity, &it.Price); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		o.Items = append(o.Items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading item rows: %w", err)
	}

	return &o, nil
}

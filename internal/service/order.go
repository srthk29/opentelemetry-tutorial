package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrInvalidOrderAmount = errors.New("order amount must be greater than zero")
	ErrEmptyOrderItems   = errors.New("order must contain at least one item")
	ErrPaymentFailed     = errors.New("payment declined: card limit exceeded")
	ErrOrderNotFound     = errors.New("order not found")
)

// Item represents a single line item in an order.
type Item struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// Order represents an e-commerce order.
type Order struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	CustomerTier  string    `json:"customer_tier"`
	Items         []Item    `json:"items"`
	TotalAmount   float64   `json:"total_amount"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateOrderRequest is the payload for creating a new order.
type CreateOrderRequest struct {
	CustomerID    string  `json:"customer_id"`
	Items         []Item  `json:"items"`
	PaymentMethod string  `json:"payment_method"`
}

// OrderService manages orders with full OpenTelemetry instrumentation.
type OrderService struct {
	mu     sync.RWMutex
	orders map[string]*Order

	// OpenTelemetry Tracing
	tracer trace.Tracer

	// OpenTelemetry Metrics
	ordersCounter       metric.Int64Counter
	activeOrdersGauge   metric.Int64UpDownCounter
	orderAmountHist     metric.Float64Histogram
	processingTimeHist  metric.Float64Histogram
}

// NewOrderService initializes the OrderService and registers metrics instruments.
func NewOrderService() (*OrderService, error) {
	// 1. Obtain Tracer & Meter from global providers
	tracer := otel.Tracer("order-service")
	meter := otel.Meter("order-service")

	// 2. Synchronous Metric: Counter (Monotonically increasing)
	ordersCounter, err := meter.Int64Counter(
		"orders.total",
		metric.WithDescription("Total number of orders submitted to the service"),
		metric.WithUnit("{order}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create orders counter: %w", err)
	}

	// 3. Synchronous Metric: UpDownCounter (Can go up and down to track active/in-flight jobs)
	activeOrdersGauge, err := meter.Int64UpDownCounter(
		"orders.active",
		metric.WithDescription("Number of orders currently being processed"),
		metric.WithUnit("{order}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active orders gauge: %w", err)
	}

	// 4. Synchronous Metric: Histogram (Measures distribution of order amounts in USD)
	orderAmountHist, err := meter.Float64Histogram(
		"orders.amount.usd",
		metric.WithDescription("Distribution of processed order amounts in USD"),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create order amount histogram: %w", err)
	}

	// 5. Synchronous Metric: Histogram (Measures latency of business logic in milliseconds)
	processingTimeHist, err := meter.Float64Histogram(
		"orders.processing_time_ms",
		metric.WithDescription("Order processing execution latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing time histogram: %w", err)
	}

	// 6. Asynchronous Metric: Observable Gauge (Polled periodically by OTel SDK Reader)
	_, err = meter.Int64ObservableGauge(
		"system.memory.heap_alloc_bytes",
		metric.WithDescription("Current heap memory allocated by runtime in bytes"),
		metric.WithUnit("By"),
		metric.WithInt64Callback(func(ctx context.Context, obs metric.Int64Observer) error {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			obs.Observe(int64(m.HeapAlloc))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory observable gauge: %w", err)
	}

	_, err = meter.Int64ObservableGauge(
		"system.goroutines",
		metric.WithDescription("Current number of active goroutines"),
		metric.WithUnit("{goroutine}"),
		metric.WithInt64Callback(func(ctx context.Context, obs metric.Int64Observer) error {
			obs.Observe(int64(runtime.NumGoroutine()))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create goroutines observable gauge: %w", err)
	}

	return &OrderService{
		orders:              make(map[string]*Order),
		tracer:              tracer,
		ordersCounter:       ordersCounter,
		activeOrdersGauge:   activeOrdersGauge,
		orderAmountHist:     orderAmountHist,
		processingTimeHist:  processingTimeHist,
	}, nil
}

// CreateOrder processes an order with child spans, events, attributes, and metric recordings.
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	startTime := time.Now()

	// Track in-flight orders with UpDownCounter
	s.activeOrdersGauge.Add(ctx, 1)
	defer s.activeOrdersGauge.Add(ctx, -1)

	// 1. Read Baggage propagated from caller (e.g. client set "customer.tier")
	bag := baggage.FromContext(ctx)
	customerTier := bag.Member("customer.tier").Value()
	if customerTier == "" {
		customerTier = "standard"
	}

	// 2. Start parent span for the order business transaction
	ctx, span := s.tracer.Start(ctx, "order.process",
		trace.WithAttributes(
			attribute.String("customer.id", req.CustomerID),
			attribute.String("customer.tier", customerTier),
			attribute.String("payment.method", req.PaymentMethod),
			attribute.Int("item.count", len(req.Items)),
		),
	)
	defer span.End()

	orderID := fmt.Sprintf("ord-%d-%d", time.Now().UnixNano(), rand.Intn(1000))
	span.SetAttributes(attribute.String("order.id", orderID))

	// 3. Child Span: Validation
	totalAmount, err := s.validateOrder(ctx, req)
	if err != nil {
		s.recordFailureMetrics(ctx, req.PaymentMethod, customerTier, "validation_error")
		return nil, err
	}
	span.SetAttributes(attribute.Float64("order.amount", totalAmount))

	// 4. Child Span: Payment Authorization
	if err := s.authorizePayment(ctx, orderID, req.CustomerID, totalAmount, req.PaymentMethod); err != nil {
		s.recordFailureMetrics(ctx, req.PaymentMethod, customerTier, "payment_failed")
		return nil, err
	}

	// 5. Child Span: Database Persistence
	order := &Order{
		ID:            orderID,
		CustomerID:    req.CustomerID,
		CustomerTier:  customerTier,
		Items:         req.Items,
		TotalAmount:   totalAmount,
		PaymentMethod: req.PaymentMethod,
		Status:        "COMPLETED",
		CreatedAt:     time.Now(),
	}

	if err := s.saveOrder(ctx, order); err != nil {
		s.recordFailureMetrics(ctx, req.PaymentMethod, customerTier, "database_error")
		return nil, err
	}

	// 6. Record Success Metrics
	durationMs := float64(time.Since(startTime).Milliseconds())
	commonAttrs := metric.WithAttributes(
		attribute.String("status", "success"),
		attribute.String("payment_method", req.PaymentMethod),
		attribute.String("customer_tier", customerTier),
	)

	s.ordersCounter.Add(ctx, 1, commonAttrs)
	s.orderAmountHist.Record(ctx, totalAmount, commonAttrs)
	s.processingTimeHist.Record(ctx, durationMs, commonAttrs)

	span.SetStatus(codes.Ok, "order processed successfully")
	return order, nil
}

// validateOrder validates the order items and calculates the total amount.
func (s *OrderService) validateOrder(ctx context.Context, req CreateOrderRequest) (float64, error) {
	_, span := s.tracer.Start(ctx, "order.validate")
	defer span.End()

	if len(req.Items) == 0 {
		err := ErrEmptyOrderItems
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	var total float64
	for _, item := range req.Items {
		if item.Price <= 0 || item.Quantity <= 0 {
			err := ErrInvalidOrderAmount
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return 0, err
		}
		total += item.Price * float64(item.Quantity)
	}

	span.SetAttributes(attribute.Float64("validation.calculated_total", total))
	span.SetStatus(codes.Ok, "order validated")
	return total, nil
}

// authorizePayment simulates calling a payment gateway with latency, events, and simulated errors.
func (s *OrderService) authorizePayment(ctx context.Context, orderID, customerID string, amount float64, method string) error {
	ctx, span := s.tracer.Start(ctx, "payment.authorize",
		trace.WithAttributes(
			attribute.String("order.id", orderID),
			attribute.Float64("payment.amount", amount),
			attribute.String("payment.gateway", "stripe-mock"),
		),
	)
	defer span.End()

	// Simulate payment processing network latency (30-80ms)
	latency := time.Duration(30+rand.Intn(50)) * time.Millisecond
	time.Sleep(latency)

	// Simulated business rule: orders over $5000 trigger a simulated card decline
	if amount > 5000.0 {
		err := ErrPaymentFailed
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment gateway declined transaction")
		span.AddEvent("payment_declined", trace.WithAttributes(
			attribute.String("decline_code", "insufficient_limit"),
			attribute.Float64("amount_attempted", amount),
		))
		return err
	}

	// Add an OpenTelemetry Span Event (in-span milestone with timestamp & metadata)
	txID := fmt.Sprintf("tx-%d", time.Now().UnixNano())
	span.AddEvent("payment_authorized", trace.WithAttributes(
		attribute.String("payment.transaction_id", txID),
		attribute.String("payment.auth_code", "AUTH-9921"),
	))

	span.SetStatus(codes.Ok, "payment authorized")
	return nil
}

// saveOrder simulates saving the order to a database with DB semantic attributes.
func (s *OrderService) saveOrder(ctx context.Context, order *Order) error {
	_, span := s.tracer.Start(ctx, "db.insert_order",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.name", "ecommerce_db"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.sql.table", "orders"),
			attribute.String("order.id", order.ID),
		),
	)
	defer span.End()

	// Simulate DB write latency (5-15ms)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)

	s.mu.Lock()
	s.orders[order.ID] = order
	s.mu.Unlock()

	span.SetStatus(codes.Ok, "order persisted to database")
	return nil
}

// GetOrder retrieves an order by its ID.
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	_, span := s.tracer.Start(ctx, "db.get_order",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.name", "ecommerce_db"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "orders"),
			attribute.String("order.id", orderID),
		),
	)
	defer span.End()

	s.mu.RLock()
	order, exists := s.orders[orderID]
	s.mu.RUnlock()

	if !exists {
		err := ErrOrderNotFound
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("order %s not found", orderID))
		return nil, err
	}

	span.SetStatus(codes.Ok, "order found")
	return order, nil
}

func (s *OrderService) recordFailureMetrics(ctx context.Context, method, tier, reason string) {
	attrs := metric.WithAttributes(
		attribute.String("status", "failed"),
		attribute.String("payment_method", method),
		attribute.String("customer_tier", tier),
		attribute.String("failure_reason", reason),
	)
	s.ordersCounter.Add(ctx, 1, attrs)
}

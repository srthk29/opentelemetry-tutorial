package domain

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
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
	CustomerTier  string  `json:"customer_tier"`
	Items         []Item  `json:"items"`
	PaymentMethod string  `json:"payment_method"`
}

// MetricsRecorder is an OUTBOUND PORT (Hexagonal Architecture).
// The domain declares what metrics it cares about, without knowing how
// they are recorded (OpenTelemetry, Prometheus, Datadog, or Noop).
// Notice: ZERO OpenTelemetry imports here!
type MetricsRecorder interface {
	RecordOrderCreated(ctx context.Context, amount float64, paymentMethod, customerTier string)
	RecordOrderFailed(ctx context.Context, paymentMethod, customerTier, reason string)
}

// NoopMetricsRecorder is a default no-op implementation, making unit tests effortless.
type NoopMetricsRecorder struct{}

func (NoopMetricsRecorder) RecordOrderCreated(ctx context.Context, amount float64, paymentMethod, customerTier string) {}
func (NoopMetricsRecorder) RecordOrderFailed(ctx context.Context, paymentMethod, customerTier, reason string)           {}

// OrderRepository is the storage PORT for persistence.
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
}

// OrderService is the core BUSINESS SERVICE PORT.
type OrderService interface {
	CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error)
	GetOrder(ctx context.Context, id string) (*Order, error)
}

// InMemoryOrderRepository is a simple thread-safe repository implementation.
type InMemoryOrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

func NewInMemoryOrderRepository() *InMemoryOrderRepository {
	return &InMemoryOrderRepository{
		orders: make(map[string]*Order),
	}
}

func (r *InMemoryOrderRepository) Save(ctx context.Context, order *Order) error {
	// Simulate minor storage latency (5-15ms)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)

	r.mu.Lock()
	r.orders[order.ID] = order
	r.mu.Unlock()
	return nil
}

func (r *InMemoryOrderRepository) FindByID(ctx context.Context, id string) (*Order, error) {
	r.mu.RLock()
	order, exists := r.orders[id]
	r.mu.RUnlock()

	if !exists {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// PureOrderService implements OrderService with PURE DOMAIN LOGIC.
// There is NOT A SINGLE LINE OF TELEMETRY here!
type PureOrderService struct {
	repo    OrderRepository
	metrics MetricsRecorder
}

func NewPureOrderService(repo OrderRepository, metrics MetricsRecorder) *PureOrderService {
	if metrics == nil {
		metrics = NoopMetricsRecorder{}
	}
	return &PureOrderService{
		repo:    repo,
		metrics: metrics,
	}
}

func (s *PureOrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	customerTier := req.CustomerTier
	if customerTier == "" {
		customerTier = "standard"
	}

	// 1. Domain Validation
	totalAmount, err := s.validateItems(req.Items)
	if err != nil {
		s.metrics.RecordOrderFailed(ctx, req.PaymentMethod, customerTier, "validation_error")
		return nil, err
	}

	// 2. Business Logic: Payment Authorization
	if err := s.authorizePayment(totalAmount, req.PaymentMethod); err != nil {
		s.metrics.RecordOrderFailed(ctx, req.PaymentMethod, customerTier, "payment_failed")
		return nil, err
	}

	// 3. Entity Creation
	orderID := fmt.Sprintf("ord-%d-%d", time.Now().UnixNano(), rand.Intn(1000))
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

	// 4. Persistence via repository port
	if err := s.repo.Save(ctx, order); err != nil {
		s.metrics.RecordOrderFailed(ctx, req.PaymentMethod, customerTier, "storage_error")
		return nil, err
	}

	// 5. Notify metrics port
	s.metrics.RecordOrderCreated(ctx, totalAmount, req.PaymentMethod, customerTier)

	return order, nil
}

func (s *PureOrderService) GetOrder(ctx context.Context, id string) (*Order, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *PureOrderService) validateItems(items []Item) (float64, error) {
	if len(items) == 0 {
		return 0, ErrEmptyOrderItems
	}

	var total float64
	for _, item := range items {
		if item.Price <= 0 || item.Quantity <= 0 {
			return 0, ErrInvalidOrderAmount
		}
		total += item.Price * float64(item.Quantity)
	}
	return total, nil
}

func (s *PureOrderService) authorizePayment(amount float64, method string) error {
	// Simulate payment gateway latency (30-80ms)
	time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)

	// Simulated business rule: orders over $5000 decline
	if amount > 5000.0 {
		return ErrPaymentFailed
	}
	return nil
}

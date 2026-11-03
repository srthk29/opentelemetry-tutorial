package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedOrderService is a DECORATOR (Decorator Pattern) that wraps
// any pure domain.OrderService with OpenTelemetry tracing and latency metrics.
//
// Key Architecture Benefit:
// The underlying domain.OrderService remains 100% clean of telemetry code!
type InstrumentedOrderService struct {
	next           domain.OrderService
	tracer         trace.Tracer
	metricsAdapter *OTelMetricsAdapter
}

// Compile-time check that InstrumentedOrderService implements domain.OrderService.
var _ domain.OrderService = (*InstrumentedOrderService)(nil)

// NewInstrumentedOrderService wraps an existing domain.OrderService with telemetry.
func NewInstrumentedOrderService(next domain.OrderService, adapter *OTelMetricsAdapter) *InstrumentedOrderService {
	return &InstrumentedOrderService{
		next:           next,
		tracer:         otel.Tracer("order-service"),
		metricsAdapter: adapter,
	}
}

// CreateOrder decorates the domain call with spans, attributes, events, and metrics.
func (s *InstrumentedOrderService) CreateOrder(ctx context.Context, req domain.CreateOrderRequest) (*domain.Order, error) {
	startTime := time.Now()

	// 1. Track in-flight count via metrics adapter
	if s.metricsAdapter != nil {
		done := s.metricsAdapter.TrackInFlight(ctx)
		defer done()
	}

	// 2. Read W3C Baggage from caller context (e.g. "customer.tier" injected by HTTP client)
	bag := baggage.FromContext(ctx)
	tier := bag.Member("customer.tier").Value()
	if tier != "" && req.CustomerTier == "" {
		req.CustomerTier = tier
	}
	if req.CustomerTier == "" {
		req.CustomerTier = "standard"
	}

	// 3. Start business transaction span
	ctx, span := s.tracer.Start(ctx, "order.process",
		trace.WithAttributes(
			attribute.String("customer.id", req.CustomerID),
			attribute.String("customer.tier", req.CustomerTier),
			attribute.String("payment.method", req.PaymentMethod),
			attribute.Int("item.count", len(req.Items)),
		),
	)
	defer span.End()

	// 4. Call inner pure domain service!
	order, err := s.next.CreateOrder(ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		// Record error in span
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		if s.metricsAdapter != nil {
			s.metricsAdapter.RecordLatency(ctx, duration, req.PaymentMethod, req.CustomerTier, "failed")
		}
		return nil, err
	}

	// 5. Enrich span on success
	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.Float64("order.amount", order.TotalAmount),
	)
	span.AddEvent("order_processed_successfully", trace.WithAttributes(
		attribute.String("order.id", order.ID),
		attribute.Float64("order.amount", order.TotalAmount),
	))
	span.SetStatus(codes.Ok, "order created successfully")

	if s.metricsAdapter != nil {
		s.metricsAdapter.RecordLatency(ctx, duration, req.PaymentMethod, req.CustomerTier, "success")
	}

	return order, nil
}

// GetOrder decorates retrieval with a trace span and database attributes.
func (s *InstrumentedOrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	ctx, span := s.tracer.Start(ctx, "order.get_by_id",
		trace.WithAttributes(
			attribute.String("order.id", id),
			attribute.String("db.system", "in-memory"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()

	order, err := s.next.GetOrder(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("order %s not found", id))
		return nil, err
	}

	span.SetStatus(codes.Ok, "order found")
	return order, nil
}

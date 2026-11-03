package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetricsAdapter is an ADAPTER (Hexagonal Architecture) that implements
// domain.MetricsRecorder using the OpenTelemetry Metrics API.
//
// The domain only knows about domain.MetricsRecorder; it never knows that
// OpenTelemetry is what receives the metrics behind the scenes!
type OTelMetricsAdapter struct {
	ordersCounter      metric.Int64Counter
	activeOrdersGauge  metric.Int64UpDownCounter
	orderAmountHist    metric.Float64Histogram
	processingTimeHist metric.Float64Histogram
}

// Ensure OTelMetricsAdapter satisfies domain.MetricsRecorder at compile-time.
var _ domain.MetricsRecorder = (*OTelMetricsAdapter)(nil)

// NewOTelMetricsAdapter creates the metrics instruments and registers observable gauges.
func NewOTelMetricsAdapter() (*OTelMetricsAdapter, error) {
	meter := otel.Meter("order-service")

	// 1. Monotonic Counter
	ordersCounter, err := meter.Int64Counter(
		"orders.total",
		metric.WithDescription("Total number of orders submitted to the service"),
		metric.WithUnit("{order}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create orders counter: %w", err)
	}

	// 2. UpDownCounter for in-flight tasks
	activeOrdersGauge, err := meter.Int64UpDownCounter(
		"orders.active",
		metric.WithDescription("Number of orders currently being processed"),
		metric.WithUnit("{order}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active orders gauge: %w", err)
	}

	// 3. Amount Histogram
	orderAmountHist, err := meter.Float64Histogram(
		"orders.amount.usd",
		metric.WithDescription("Distribution of processed order amounts in USD"),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create order amount histogram: %w", err)
	}

	// 4. Latency Histogram
	processingTimeHist, err := meter.Float64Histogram(
		"orders.processing_time_ms",
		metric.WithDescription("Order processing execution latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing time histogram: %w", err)
	}

	// 5. Asynchronous / Observable Gauges (Polled by OTel PeriodicReader)
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

	return &OTelMetricsAdapter{
		ordersCounter:      ordersCounter,
		activeOrdersGauge:  activeOrdersGauge,
		orderAmountHist:    orderAmountHist,
		processingTimeHist: processingTimeHist,
	}, nil
}

// RecordOrderCreated implements domain.MetricsRecorder.
func (a *OTelMetricsAdapter) RecordOrderCreated(ctx context.Context, amount float64, paymentMethod, customerTier string) {
	attrs := metric.WithAttributes(
		attribute.String("status", "success"),
		attribute.String("payment_method", paymentMethod),
		attribute.String("customer_tier", customerTier),
	)

	a.ordersCounter.Add(ctx, 1, attrs)
	a.orderAmountHist.Record(ctx, amount, attrs)
}

// RecordOrderFailed implements domain.MetricsRecorder.
func (a *OTelMetricsAdapter) RecordOrderFailed(ctx context.Context, paymentMethod, customerTier, reason string) {
	attrs := metric.WithAttributes(
		attribute.String("status", "failed"),
		attribute.String("payment_method", paymentMethod),
		attribute.String("customer_tier", customerTier),
		attribute.String("failure_reason", reason),
	)

	a.ordersCounter.Add(ctx, 1, attrs)
}

// TrackInFlight returns a cleanup function to decrement active order count.
func (a *OTelMetricsAdapter) TrackInFlight(ctx context.Context) func() {
	a.activeOrdersGauge.Add(ctx, 1)
	return func() {
		a.activeOrdersGauge.Add(ctx, -1)
	}
}

// RecordLatency records duration in milliseconds.
func (a *OTelMetricsAdapter) RecordLatency(ctx context.Context, duration time.Duration, paymentMethod, customerTier, status string) {
	attrs := metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("payment_method", paymentMethod),
		attribute.String("customer_tier", customerTier),
	)
	a.processingTimeHist.Record(ctx, float64(duration.Milliseconds()), attrs)
}

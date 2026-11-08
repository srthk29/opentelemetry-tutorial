package instrumented

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds standard OpenTelemetry GenAI metric instruments for embedding operations.
type Metrics struct {
	// gen_ai.client.token.usage measures prompt/input tokens used to generate embeddings.
	TokenUsage metric.Int64Counter

	// gen_ai.client.operation.duration measures the latency of embedding inference calls.
	OperationDuration metric.Float64Histogram

	// gen_ai.client.embeddings.batch_size measures the number of texts processed in a batch call.
	BatchSize metric.Int64Histogram

	// gen_ai.client.active_requests tracks in-flight embedding requests.
	ActiveRequests metric.Int64UpDownCounter
}

// NewMetrics initializes the standard OpenTelemetry GenAI embedding metrics.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	if meter == nil {
		meter = otel.Meter("gen_ai.client.embeddings")
	}

	tokenUsage, err := meter.Int64Counter(
		"gen_ai.client.token.usage",
		metric.WithDescription("Number of input tokens consumed by the embedding model"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gen_ai.client.token.usage metric: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithDescription("Latency of GenAI embedding operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gen_ai.client.operation.duration metric: %w", err)
	}

	batchSize, err := meter.Int64Histogram(
		"gen_ai.client.embeddings.batch_size",
		metric.WithDescription("Distribution of text count per batch embedding request"),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gen_ai.client.embeddings.batch_size metric: %w", err)
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"gen_ai.client.active_requests",
		metric.WithDescription("Number of active in-flight embedding requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gen_ai.client.active_requests metric: %w", err)
	}

	return &Metrics{
		TokenUsage:        tokenUsage,
		OperationDuration: operationDuration,
		BatchSize:         batchSize,
		ActiveRequests:    activeRequests,
	}, nil
}

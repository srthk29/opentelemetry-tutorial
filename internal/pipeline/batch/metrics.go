package batch

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// BatchMetrics holds OpenTelemetry metrics for batch processing and GenAI chunk execution.
type BatchMetrics struct {
	StageActive     metric.Int64UpDownCounter
	StageDuration   metric.Float64Histogram
	StageRuns       metric.Int64Counter
	ItemsProcessed  metric.Int64Counter
	ChunksProcessed metric.Int64Counter

	TokenUsage    metric.Int64Counter
	ChunkDuration metric.Float64Histogram
}

// NewBatchMetrics initializes all instruments for batch pipelines.
func NewBatchMetrics() (*BatchMetrics, error) {
	meter := otel.Meter("pipeline-batch")

	stageActive, err := meter.Int64UpDownCounter(
		"pipeline.batch.stage.active",
		metric.WithDescription("Number of batch stages currently executing"),
		metric.WithUnit("{stages}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.stage.active metric: %w", err)
	}

	stageDuration, err := meter.Float64Histogram(
		"pipeline.batch.stage.duration_ms",
		metric.WithDescription("Execution time per batch stage in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.stage.duration_ms metric: %w", err)
	}

	stageRuns, err := meter.Int64Counter(
		"pipeline.batch.stage.runs_total",
		metric.WithDescription("Total number of batch stage executions"),
		metric.WithUnit("{runs}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.stage.runs_total metric: %w", err)
	}

	itemsProcessed, err := meter.Int64Counter(
		"pipeline.batch.documents_total",
		metric.WithDescription("Total number of raw documents processed through batch stages"),
		metric.WithUnit("{documents}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.documents_total metric: %w", err)
	}

	chunksProcessed, err := meter.Int64Counter(
		"pipeline.batch.chunks_total",
		metric.WithDescription("Total number of individual chunks processed"),
		metric.WithUnit("{chunks}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.chunks_total metric: %w", err)
	}

	tokenUsage, err := meter.Int64Counter(
		"gen_ai.client.token.usage",
		metric.WithDescription("Number of input or output tokens consumed by local AI models"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gen_ai.client.token.usage metric: %w", err)
	}

	chunkDuration, err := meter.Float64Histogram(
		"pipeline.batch.chunk.duration_ms",
		metric.WithDescription("Inference time per chunk in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline.batch.chunk.duration_ms metric: %w", err)
	}

	return &BatchMetrics{
		StageActive:     stageActive,
		StageDuration:   stageDuration,
		StageRuns:       stageRuns,
		ItemsProcessed:  itemsProcessed,
		ChunksProcessed: chunksProcessed,
		TokenUsage:      tokenUsage,
		ChunkDuration:   chunkDuration,
	}, nil
}

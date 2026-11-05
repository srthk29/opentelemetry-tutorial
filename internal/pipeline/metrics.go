package pipeline

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics encapsulates standard OpenTelemetry metrics for pipeline workflows.
type Metrics struct {
	// Workflow level metrics
	WorkflowRuns     metric.Int64Counter
	WorkflowDuration metric.Float64Histogram
	WorkflowActive   metric.Int64UpDownCounter

	// Stage level metrics
	StageRuns     metric.Int64Counter
	StageDuration metric.Float64Histogram
	StageActive   metric.Int64UpDownCounter

	// Pipeline domain & GenAI metrics (OTel GenAI conventions)
	ItemsProcessed metric.Int64Counter
	TokenUsage     metric.Int64Counter
}

// NewMetrics initializes all pipeline metrics using the global or passed MeterProvider.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("pipeline-orchestrator")

	workflowRuns, err := meter.Int64Counter(
		"pipeline.workflow.runs_total",
		metric.WithDescription("Total number of full pipeline workflow executions"),
		metric.WithUnit("{run}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow runs counter: %w", err)
	}

	workflowDuration, err := meter.Float64Histogram(
		"pipeline.workflow.duration_ms",
		metric.WithDescription("Total end-to-end pipeline execution latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow duration histogram: %w", err)
	}

	workflowActive, err := meter.Int64UpDownCounter(
		"pipeline.workflow.active",
		metric.WithDescription("Number of pipeline workflows currently running"),
		metric.WithUnit("{workflow}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow active gauge: %w", err)
	}

	stageRuns, err := meter.Int64Counter(
		"pipeline.stage.runs_total",
		metric.WithDescription("Total executions of individual pipeline stages"),
		metric.WithUnit("{execution}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stage runs counter: %w", err)
	}

	stageDuration, err := meter.Float64Histogram(
		"pipeline.stage.duration_ms",
		metric.WithDescription("Execution latency of individual pipeline stages in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stage duration histogram: %w", err)
	}

	stageActive, err := meter.Int64UpDownCounter(
		"pipeline.stage.active",
		metric.WithDescription("Number of executions currently in flight per stage"),
		metric.WithUnit("{stage}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stage active gauge: %w", err)
	}

	itemsProcessed, err := meter.Int64Counter(
		"pipeline.items.processed_total",
		metric.WithDescription("Total volume of data items processed (chunks, words, vectors)"),
		metric.WithUnit("{item}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create items processed counter: %w", err)
	}

	tokenUsage, err := meter.Int64Counter(
		"gen_ai.client.token.usage",
		metric.WithDescription("Total GenAI tokens consumed by models (OTel GenAI convention)"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token usage counter: %w", err)
	}

	return &Metrics{
		WorkflowRuns:     workflowRuns,
		WorkflowDuration: workflowDuration,
		WorkflowActive:   workflowActive,
		StageRuns:        stageRuns,
		StageDuration:    stageDuration,
		StageActive:      stageActive,
		ItemsProcessed:   itemsProcessed,
		TokenUsage:       tokenUsage,
	}, nil
}

package batch

import (
	"context"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// BatchOrchestrator coordinates sequential execution of batch pipeline stages
// under a unified root OpenTelemetry workflow trace.
type BatchOrchestrator struct {
	name    string
	stages  []pipeline.Pipeline
	tracer  trace.Tracer
	metrics *BatchMetrics
}

// NewBatchOrchestrator wraps each stage with BatchTelemetryDecorator and builds the coordinator.
func NewBatchOrchestrator(name string, stages []pipeline.Pipeline, metrics *BatchMetrics) *BatchOrchestrator {
	decorated := make([]pipeline.Pipeline, len(stages))
	for i, s := range stages {
		decorated[i] = WrapBatchStage(s, metrics)
	}

	return &BatchOrchestrator{
		name:    name,
		stages:  decorated,
		tracer:  otel.Tracer("pipeline-batch-orchestrator"),
		metrics: metrics,
	}
}

// Execute runs the staged batch workflow sequentially across all documents.
func (o *BatchOrchestrator) Execute(ctx context.Context, params *request.FilterParams) error {
	startTime := time.Now()
	docs, _ := GetDocuments(params)
	docCount := len(docs)

	// 1. Root Parent Span for Entire Batch Workflow
	ctx, span := o.tracer.Start(ctx, fmt.Sprintf("pipeline.workflow: %s", o.name),
		trace.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
			attribute.Int("pipeline.workflow.stage_count", len(o.stages)),
			attribute.Int("pipeline.batch.document_count", docCount),
			attribute.String("pipeline.request_id", params.RequestID),
			attribute.String("pipeline.tenant_id", params.TenantID),
		),
	)
	defer span.End()

	// 2. Execute each batch stage sequentially
	for _, stage := range o.stages {
		stageName := string(stage.Stage())

		span.AddEvent(fmt.Sprintf("starting_batch_stage: %s", stageName), trace.WithAttributes(
			attribute.String("pipeline.stage", stageName),
		))

		if err := stage.Run(ctx, params); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("batch pipeline failed at stage %s: %v", stageName, err))
			return fmt.Errorf("batch stage %s failed: %w", stageName, err)
		}
	}

	// 3. Mark Workflow Success
	durationMs := float64(time.Since(startTime).Milliseconds())
	totalChunks := TotalChunks(docs)
	span.SetAttributes(
		attribute.Int("pipeline.batch.total_chunks", totalChunks),
		attribute.Float64("pipeline.workflow.total_duration_ms", durationMs),
	)
	span.SetStatus(codes.Ok, "batch workflow completed successfully")

	if o.metrics != nil {
		attrs := metric.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
			attribute.String("status", "success"),
		)
		o.metrics.StageRuns.Add(ctx, 1, attrs)
	}

	return nil
}

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

// BatchTelemetryDecorator wraps any batch stage implementing pipeline.Pipeline
// with OpenTelemetry tracing, batch metadata metrics, and error recording.
type BatchTelemetryDecorator struct {
	next    pipeline.Pipeline
	tracer  trace.Tracer
	metrics *BatchMetrics
}

var _ pipeline.Pipeline = (*BatchTelemetryDecorator)(nil)

// WrapBatchStage decorates a batch pipeline stage with OpenTelemetry instruments.
func WrapBatchStage(stage pipeline.Pipeline, metrics *BatchMetrics) pipeline.Pipeline {
	return &BatchTelemetryDecorator{
		next:    stage,
		tracer:  otel.Tracer("pipeline-batch-stage"),
		metrics: metrics,
	}
}

func (d *BatchTelemetryDecorator) Name() string {
	return d.next.Name()
}

func (d *BatchTelemetryDecorator) Stage() request.PipelineStage {
	return d.next.Stage()
}

func (d *BatchTelemetryDecorator) Run(ctx context.Context, params *request.FilterParams) error {
	stageStr := string(d.Stage())
	startTime := time.Now()

	// 1. In-flight active batch stage tracking
	if d.metrics != nil {
		d.metrics.StageActive.Add(ctx, 1, metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
		))
		defer d.metrics.StageActive.Add(ctx, -1, metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
		))
	}

	// 2. Extract batch metadata for tracing context
	docs, _ := GetDocuments(params)
	docCount := len(docs)
	initialChunks := TotalChunks(docs)

	// 3. Start overarching Batch Stage Span
	ctx, span := d.tracer.Start(ctx, fmt.Sprintf("pipeline.stage.%s.batch", stageStr),
		trace.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
			attribute.String("pipeline.name", d.Name()),
			attribute.String("pipeline.request_id", params.RequestID),
			attribute.String("pipeline.tenant_id", params.TenantID),
			attribute.Int("pipeline.batch.document_count", docCount),
			attribute.Int("pipeline.batch.initial_chunk_count", initialChunks),
		),
	)
	defer span.End()

	// 4. Run Pure Business Stage Logic
	// Any child spans created inside (model load/unload, per-chunk inference) will
	// naturally be parented by this batch stage span!
	err := d.next.Run(ctx, params)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 5. Handle Error
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		if d.metrics != nil {
			errAttrs := metric.WithAttributes(
				attribute.String("pipeline.stage", stageStr),
				attribute.String("status", "failed"),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			d.metrics.StageRuns.Add(ctx, 1, errAttrs)
			d.metrics.StageDuration.Record(ctx, durationMs, errAttrs)
		}
		return err
	}

	// 6. Record Success
	finalChunks := TotalChunks(docs)
	span.SetAttributes(
		attribute.Int("pipeline.batch.final_chunk_count", finalChunks),
	)
	span.SetStatus(codes.Ok, fmt.Sprintf("batch stage %s processed %d documents (%d chunks)",
		stageStr, docCount, finalChunks))

	if d.metrics != nil {
		successAttrs := metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
			attribute.String("status", "success"),
		)
		d.metrics.StageRuns.Add(ctx, 1, successAttrs)
		d.metrics.StageDuration.Record(ctx, durationMs, successAttrs)
		d.metrics.ItemsProcessed.Add(ctx, int64(docCount), successAttrs)
	}

	return nil
}

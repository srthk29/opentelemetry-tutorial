package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// TelemetryDecorator wraps ANY implementation of Pipeline with standard
// OpenTelemetry spans, latency histograms, error recording, and GenAI conventions.
//
// Key Clean Architecture Benefit:
// The actual pipeline stage code remains 100% pure Go with ZERO OTel imports!
type TelemetryDecorator struct {
	next    Pipeline
	tracer  trace.Tracer
	metrics *Metrics
}

// Compile-time assertion that TelemetryDecorator implements Pipeline.
var _ Pipeline = (*TelemetryDecorator)(nil)

// Wrap returns a new Pipeline that decorates the given stage with telemetry.
func Wrap(stage Pipeline, metrics *Metrics) Pipeline {
	return &TelemetryDecorator{
		next:    stage,
		tracer:  otel.Tracer("pipeline-stage"),
		metrics: metrics,
	}
}

func (d *TelemetryDecorator) Name() string {
	return d.next.Name()
}

func (d *TelemetryDecorator) Stage() request.PipelineStage {
	return d.next.Stage()
}

func (d *TelemetryDecorator) Run(ctx context.Context, params *request.FilterParams) error {
	stageStr := string(d.Stage())
	startTime := time.Now()

	// 1. In-flight active jobs tracking
	if d.metrics != nil {
		d.metrics.StageActive.Add(ctx, 1, metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
		))
		defer d.metrics.StageActive.Add(ctx, -1, metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
		))
	}

	// 2. Start child span for this stage
	ctx, span := d.tracer.Start(ctx, fmt.Sprintf("pipeline.stage.%s", stageStr),
		trace.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
			attribute.String("pipeline.name", d.Name()),
			attribute.String("pipeline.request_id", params.RequestID),
			attribute.String("pipeline.document_id", params.DocumentID),
			attribute.String("pipeline.tenant_id", params.TenantID),
		),
	)
	defer span.End()

	// 3. Execute pure pipeline stage business logic
	err := d.next.Run(ctx, params)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
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

	// 5. Enrich Span and Record GenAI Semantic Conventions from stage outputs
	d.enrichTelemetry(ctx, span, params)

	// 6. Record Success
	span.SetStatus(codes.Ok, fmt.Sprintf("stage %s completed successfully", stageStr))

	if d.metrics != nil {
		successAttrs := metric.WithAttributes(
			attribute.String("pipeline.stage", stageStr),
			attribute.String("status", "success"),
		)
		d.metrics.StageRuns.Add(ctx, 1, successAttrs)
		d.metrics.StageDuration.Record(ctx, durationMs, successAttrs)
	}

	return nil
}

// enrichTelemetry reads stage metadata and attaches standard OTel & GenAI attributes and metrics.
func (d *TelemetryDecorator) enrichTelemetry(ctx context.Context, span trace.Span, params *request.FilterParams) {
	stage := d.Stage()

	switch stage {
	case request.StageTranscript:
		if words := params.GetInt("transcript.word_count"); words > 0 {
			span.SetAttributes(
				attribute.Int("transcript.word_count", words),
				attribute.Float64("transcript.audio_duration_sec", params.Metadata["audio.duration_sec"].(float64)),
			)
			if d.metrics != nil {
				d.metrics.ItemsProcessed.Add(ctx, int64(words), metric.WithAttributes(
					attribute.String("pipeline.stage", "transcript"),
					attribute.String("item.type", "words"),
				))
			}
		}

	case request.StageChunking:
		chunkCount := len(params.Chunks)
		span.SetAttributes(
			attribute.Int("chunk.count", chunkCount),
			attribute.String("chunk.strategy", params.GetString("chunk.strategy")),
			attribute.Int("chunk.size_chars", params.GetInt("chunk.size_chars")),
		)
		if d.metrics != nil {
			d.metrics.ItemsProcessed.Add(ctx, int64(chunkCount), metric.WithAttributes(
				attribute.String("pipeline.stage", "chunking"),
				attribute.String("item.type", "chunks"),
			))
		}

	case request.StageEmbeddings:
		model := params.GetString("gen_ai.request.model")
		tokens := params.GetInt("gen_ai.usage.input_tokens")
		dim := params.GetInt("embedding.dimensions")

		// Standard OTel GenAI Semantic Conventions
		span.SetAttributes(
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", "openai"),
			attribute.String("gen_ai.request.model", model),
			attribute.Int("gen_ai.usage.input_tokens", tokens),
			attribute.Int("embedding.vector_count", len(params.Embeddings)),
			attribute.Int("embedding.dimensions", dim),
		)

		if d.metrics != nil && tokens > 0 {
			d.metrics.TokenUsage.Add(ctx, int64(tokens), metric.WithAttributes(
				attribute.String("gen_ai.operation.name", "embeddings"),
				attribute.String("gen_ai.request.model", model),
				attribute.String("gen_ai.token.type", "input"),
			))
		}

	case request.StageSummarizer:
		model := params.GetString("gen_ai.request.model")
		inputTokens := params.GetInt("gen_ai.usage.input_tokens")
		outputTokens := params.GetInt("gen_ai.usage.output_tokens")

		// Standard OTel GenAI Semantic Conventions for Chat/Completion
		span.SetAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", "google"),
			attribute.String("gen_ai.request.model", model),
			attribute.Int("gen_ai.usage.input_tokens", inputTokens),
			attribute.Int("gen_ai.usage.output_tokens", outputTokens),
		)

		if d.metrics != nil {
			if inputTokens > 0 {
				d.metrics.TokenUsage.Add(ctx, int64(inputTokens), metric.WithAttributes(
					attribute.String("gen_ai.operation.name", "chat"),
					attribute.String("gen_ai.request.model", model),
					attribute.String("gen_ai.token.type", "input"),
				))
			}
			if outputTokens > 0 {
				d.metrics.TokenUsage.Add(ctx, int64(outputTokens), metric.WithAttributes(
					attribute.String("gen_ai.operation.name", "chat"),
					attribute.String("gen_ai.request.model", model),
					attribute.String("gen_ai.token.type", "output"),
				))
			}
		}
	}
}

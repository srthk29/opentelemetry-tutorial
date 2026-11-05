package batch

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedChunkSummarizer decorates ChunkSummarizer with GenAI child spans and token metrics.
type InstrumentedChunkSummarizer struct {
	next    ChunkSummarizer
	tracer  trace.Tracer
	metrics *BatchMetrics
}

var _ ChunkSummarizer = (*InstrumentedChunkSummarizer)(nil)

func WrapSummarizer(s ChunkSummarizer, metrics *BatchMetrics) ChunkSummarizer {
	return &InstrumentedChunkSummarizer{
		next:    s,
		tracer:  otel.Tracer("pipeline-batch-chunk"),
		metrics: metrics,
	}
}

func (s *InstrumentedChunkSummarizer) ModelName() string {
	return s.next.ModelName()
}

func (s *InstrumentedChunkSummarizer) SummarizeChunk(ctx context.Context, chunk *ChunkItem) error {
	startTime := time.Now()

	// Child span nested inside the batch stage span
	ctx, span := s.tracer.Start(ctx, "pipeline.chunk.summarize",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", "llama.cpp"),
			attribute.String("gen_ai.request.model", s.next.ModelName()),
			attribute.String("pipeline.document_id", chunk.DocumentID),
			attribute.String("pipeline.chunk_id", chunk.ID),
			attribute.Int("pipeline.chunk_index", chunk.Index),
		),
	)
	defer span.End()

	err := s.next.SummarizeChunk(ctx, chunk)
	durationMs := float64(time.Since(startTime).Milliseconds())

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Enrich span with token attributes
	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", chunk.InputTokens),
		attribute.Int("gen_ai.usage.output_tokens", chunk.OutputTokens),
	)
	span.SetStatus(codes.Ok, "chunk summarized")

	// Record OpenTelemetry GenAI token metrics
	if s.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", "llama.cpp"),
			attribute.String("gen_ai.request.model", s.next.ModelName()),
		}

		inAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		outAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "output"))

		s.metrics.TokenUsage.Add(ctx, int64(chunk.InputTokens), metric.WithAttributes(inAttrs...))
		s.metrics.TokenUsage.Add(ctx, int64(chunk.OutputTokens), metric.WithAttributes(outAttrs...))
		s.metrics.ChunkDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))
		s.metrics.ChunksProcessed.Add(ctx, 1, metric.WithAttributes(baseAttrs...))
	}

	return nil
}

// InstrumentedChunkEmbedder decorates ChunkEmbedder with GenAI child spans and token metrics.
type InstrumentedChunkEmbedder struct {
	next    ChunkEmbedder
	tracer  trace.Tracer
	metrics *BatchMetrics
}

var _ ChunkEmbedder = (*InstrumentedChunkEmbedder)(nil)

func WrapEmbedder(e ChunkEmbedder, metrics *BatchMetrics) ChunkEmbedder {
	return &InstrumentedChunkEmbedder{
		next:    e,
		tracer:  otel.Tracer("pipeline-batch-chunk"),
		metrics: metrics,
	}
}

func (e *InstrumentedChunkEmbedder) ModelName() string {
	return e.next.ModelName()
}

func (e *InstrumentedChunkEmbedder) Dimensions() int {
	return e.next.Dimensions()
}

func (e *InstrumentedChunkEmbedder) EmbedChunk(ctx context.Context, chunk *ChunkItem) error {
	startTime := time.Now()

	ctx, span := e.tracer.Start(ctx, "pipeline.chunk.embed",
		trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", "llama.cpp"),
			attribute.String("gen_ai.request.model", e.next.ModelName()),
			attribute.Int("gen_ai.response.dimensions", e.next.Dimensions()),
			attribute.String("pipeline.document_id", chunk.DocumentID),
			attribute.String("pipeline.chunk_id", chunk.ID),
			attribute.Int("pipeline.chunk_index", chunk.Index),
		),
	)
	defer span.End()

	err := e.next.EmbedChunk(ctx, chunk)
	durationMs := float64(time.Since(startTime).Milliseconds())

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", chunk.InputTokens),
	)
	span.SetStatus(codes.Ok, "chunk embedded")

	if e.metrics != nil {
		attrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", "llama.cpp"),
			attribute.String("gen_ai.request.model", e.next.ModelName()),
		}

		inAttrs := append(attrs, attribute.String("gen_ai.token.type", "input"))
		e.metrics.TokenUsage.Add(ctx, int64(chunk.InputTokens), metric.WithAttributes(inAttrs...))
		e.metrics.ChunkDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
		e.metrics.ChunksProcessed.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	return nil
}

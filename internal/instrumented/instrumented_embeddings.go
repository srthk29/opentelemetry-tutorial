package instrumented

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedGoogleGemmaEmbeddingClient decorates an Embedder with official
// OpenTelemetry GenAI Semantic Conventions for traces and metrics.
type InstrumentedGoogleGemmaEmbeddingClient struct {
	embedder EmbedderWithEmbeddingResponse
	tracer   trace.Tracer
	metrics  *Metrics
	system   string
}

// Compile-time assertion that InstrumentedGoogleGemmaEmbeddingClient implements Embedder.
var _ Embedder = (*InstrumentedGoogleGemmaEmbeddingClient)(nil)

// NewInstrumentedGoogleGemmaEmbeddingClient initializes the instrumented Gemma embedding client.
func NewInstrumentedGoogleGemmaEmbeddingClient(ctx context.Context, metrics *Metrics) (Embedder, error) {
	ec, err := NewOpenAIEmbeddingClient(ctx, "google/embeddinggemma-300m", 768, false)
	if err != nil {
		return nil, err
	}

	if metrics == nil {
		var metricErr error
		metrics, metricErr = NewMetrics(nil)
		if metricErr != nil {
			return nil, fmt.Errorf("failed to initialize embedding metrics: %w", metricErr)
		}
	}

	return &InstrumentedGoogleGemmaEmbeddingClient{
		embedder: ec,
		tracer:   otel.Tracer("gemma-embedder"),
		metrics:  metrics,
		system:   "google",
	}, nil
}

// GetEmbedding generates an embedding vector for a single text input with OpenTelemetry instrumentation.
func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	model := gc.embedder.GetModel()
	dims := gc.embedder.GetDimensionality()
	startTime := time.Now()

	// 1. In-flight active request tracking
	if gc.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", model),
		)
		gc.metrics.ActiveRequests.Add(ctx, 1, activeAttrs)
		defer gc.metrics.ActiveRequests.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry GenAI Client Span
	// Standard span naming convention: "embeddings {model_name}"
	ctx, span := gc.tracer.Start(ctx, fmt.Sprintf("embeddings %s", model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.request.model", model),
			attribute.Int("gen_ai.request.batch_size", 1),
			attribute.Int("gen_ai.response.dimensions", dims),
			attribute.Bool("gen_ai.embedding.normalized", gc.embedder.IsNormalized()),
		),
	)
	defer span.End()

	// 3. Execute underlying embedding API call
	respJSON200, err := gc.embedder.GetEmbedding(ctx, text)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)

		if gc.metrics != nil {
			errAttrs := metric.WithAttributes(
				attribute.String("gen_ai.operation.name", "embeddings"),
				attribute.String("gen_ai.system", gc.system),
				attribute.String("gen_ai.request.model", model),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			gc.metrics.OperationDuration.Record(ctx, durationMs, errAttrs)
		}
		return nil, err
	}

	if len(respJSON200.Data) == 0 {
		emptyErr := fmt.Errorf("embedding provider returned empty data array")
		span.RecordError(emptyErr)
		span.SetStatus(codes.Error, emptyErr.Error())
		return nil, emptyErr
	}

	// 5. Enrich Span with Token Usage & Actual Dimensions
	promptTokens := respJSON200.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = respJSON200.Usage.TotalTokens
	}
	actualDims := len(respJSON200.Data[0].Embedding)

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.response.dimensions", actualDims),
	)
	span.SetStatus(codes.Ok, "embedding generated successfully")

	// 6. Record Industry Standard Prometheus Metrics
	if gc.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", model),
		}

		// Token usage counter with token.type="input"
		tokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		gc.metrics.TokenUsage.Add(ctx, int64(promptTokens), metric.WithAttributes(tokenAttrs...))

		// Latency histogram
		gc.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))

		// Batch size distribution (1 for single item)
		gc.metrics.BatchSize.Record(ctx, 1, metric.WithAttributes(baseAttrs...))
	}

	return respJSON200.Data[0].Embedding, nil
}

// GetEmbeddings generates embedding vectors for a batch of text inputs with OpenTelemetry instrumentation.
func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	model := gc.embedder.GetModel()
	dims := gc.embedder.GetDimensionality()
	batchCount := len(texts)
	startTime := time.Now()

	// 1. In-flight active request tracking
	if gc.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", model),
		)
		gc.metrics.ActiveRequests.Add(ctx, 1, activeAttrs)
		defer gc.metrics.ActiveRequests.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry GenAI Client Span
	ctx, span := gc.tracer.Start(ctx, fmt.Sprintf("embeddings %s", model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.request.model", model),
			attribute.Int("gen_ai.request.batch_size", batchCount),
			attribute.Int("gen_ai.response.dimensions", dims),
			attribute.Bool("gen_ai.embedding.normalized", gc.embedder.IsNormalized()),
		),
	)
	defer span.End()

	// 3. Execute underlying embedding API call
	respJSON200, err := gc.embedder.GetEmbeddings(ctx, texts)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)

		if gc.metrics != nil {
			errAttrs := metric.WithAttributes(
				attribute.String("gen_ai.operation.name", "embeddings"),
				attribute.String("gen_ai.system", gc.system),
				attribute.String("gen_ai.request.model", model),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			gc.metrics.OperationDuration.Record(ctx, durationMs, errAttrs)
		}
		return nil, err
	}

	// 5. Enrich Span with Output Results & Tokens
	promptTokens := respJSON200.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = respJSON200.Usage.TotalTokens
	}

	embeddings := make([][]float32, len(respJSON200.Data))
	for i := range respJSON200.Data {
		embeddings[i] = respJSON200.Data[i].Embedding
	}

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.response.item_count", len(embeddings)),
	)
	span.SetStatus(codes.Ok, fmt.Sprintf("batch of %d embeddings generated successfully", len(embeddings)))

	// 6. Record Industry Standard Prometheus Metrics
	if gc.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", model),
		}

		tokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		gc.metrics.TokenUsage.Add(ctx, int64(promptTokens), metric.WithAttributes(tokenAttrs...))
		gc.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))
		gc.metrics.BatchSize.Record(ctx, int64(batchCount), metric.WithAttributes(baseAttrs...))
	}

	return embeddings, nil
}

func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetModel() string {
	return gc.embedder.GetModel()
}

func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetDimensionality() int {
	return gc.embedder.GetDimensionality()
}

func (gc *InstrumentedGoogleGemmaEmbeddingClient) IsNormalized() bool {
	return gc.embedder.IsNormalized()
}

// GetQueryEmbedding generates an embedding for retrieval search queries.
func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetQueryEmbedding(ctx context.Context, text string, taskType TaskType) ([]float32, error) {
	ctx, span := gc.tracer.Start(ctx, fmt.Sprintf("embedding.query: %s", taskType),
		trace.WithAttributes(
			attribute.String("gen_ai.embedding.task_type", string(taskType)),
		),
	)
	defer span.End()

	return gc.GetEmbedding(ctx, PrepareQuery(taskType, text))
}

// GetDocumentEmbedding generates an embedding for a structured document.
func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetDocumentEmbedding(ctx context.Context, content *Document) ([]float32, error) {
	ctx, span := gc.tracer.Start(ctx, "embedding.document",
		trace.WithAttributes(
			attribute.String("document.title", content.Title),
			attribute.Int("document.content_length", len(content.Content)),
		),
	)
	defer span.End()

	return gc.GetEmbedding(ctx, PrepareDocument(content.Title, content.Content))
}

// GetDocumentEmbeddings generates embeddings for a batch of structured documents.
func (gc *InstrumentedGoogleGemmaEmbeddingClient) GetDocumentEmbeddings(ctx context.Context, contents []*Document) ([][]float32, error) {
	ctx, span := gc.tracer.Start(ctx, "embedding.documents.batch",
		trace.WithAttributes(
			attribute.Int("batch.document_count", len(contents)),
		),
	)
	defer span.End()

	texts := make([]string, 0, len(contents))
	for _, content := range contents {
		texts = append(texts, PrepareDocument(content.Title, content.Content))
	}

	return gc.GetEmbeddings(ctx, texts)
}

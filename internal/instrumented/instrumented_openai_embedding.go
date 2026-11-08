package instrumented

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/instrumented/codegen"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedOpenAIEmbeddingClient wraps an OpenAI-compatible embedding client
// with official OpenTelemetry GenAI Semantic Conventions for traces and metrics.
type InstrumentedOpenAIEmbeddingClient struct {
	client         codegen.ClientWithResponsesInterface
	inner          EmbedderWithEmbeddingResponse
	model          string
	dimensionality *int32
	isNormalized   bool
	tracer         trace.Tracer
	metrics        *EmbeddingMetrics
	system         string
}

// Compile-time assertion that InstrumentedOpenAIEmbeddingClient implements EmbedderWithEmbeddingResponse.
var _ EmbedderWithEmbeddingResponse = (*InstrumentedOpenAIEmbeddingClient)(nil)

// NewInstrumentedOpenAIEmbeddingClient initializes the instrumented OpenAI embedding client.
func NewInstrumentedOpenAIEmbeddingClient(
	ctx context.Context,
	model string,
	dimensionality int32,
	isNormalized bool,
	metrics *EmbeddingMetrics,
) (*InstrumentedOpenAIEmbeddingClient, error) {
	if model == "" {
		model = "google/embeddinggemma-300m"
	}

	client, err := codegen.NewClientWithResponses("http://127.0.0.1:8080/")
	if err != nil {
		return nil, err
	}

	if metrics == nil {
		var metricErr error
		metrics, metricErr = NewEmbeddingMetrics(nil)
		if metricErr != nil {
			return nil, fmt.Errorf("failed to initialize embedding metrics: %w", metricErr)
		}
	}

	return &InstrumentedOpenAIEmbeddingClient{
		client:         client,
		model:          model,
		dimensionality: &dimensionality,
		isNormalized:   isNormalized,
		tracer:         otel.Tracer("openai-embedding-client"),
		metrics:        metrics,
		system:         "openai",
	}, nil
}

// NewInstrumentedOpenAIEmbeddingClientWithClient creates an instrumented client decorating a codegen client.
func NewInstrumentedOpenAIEmbeddingClientWithClient(
	client codegen.ClientWithResponsesInterface,
	model string,
	dimensionality int32,
	isNormalized bool,
	tracer trace.Tracer,
	metrics *EmbeddingMetrics,
) *InstrumentedOpenAIEmbeddingClient {
	if tracer == nil {
		tracer = otel.Tracer("openai-embedding-client")
	}
	if model == "" {
		model = "google/embeddinggemma-300m"
	}
	return &InstrumentedOpenAIEmbeddingClient{
		client:         client,
		model:          model,
		dimensionality: &dimensionality,
		isNormalized:   isNormalized,
		tracer:         tracer,
		metrics:        metrics,
		system:         "openai",
	}
}

// NewInstrumentedOpenAIEmbeddingClientFromInner wraps an existing EmbedderWithEmbeddingResponse.
func NewInstrumentedOpenAIEmbeddingClientFromInner(
	inner EmbedderWithEmbeddingResponse,
	tracer trace.Tracer,
	metrics *EmbeddingMetrics,
) *InstrumentedOpenAIEmbeddingClient {
	if tracer == nil {
		tracer = otel.Tracer("openai-embedding-client")
	}
	model := "google/embeddinggemma-300m"
	dim := int32(768)
	normalized := false
	if inner != nil {
		model = inner.GetModel()
		dim = int32(inner.GetDimensionality())
		normalized = inner.IsNormalized()
	}
	return &InstrumentedOpenAIEmbeddingClient{
		inner:          inner,
		model:          model,
		dimensionality: &dim,
		isNormalized:   normalized,
		tracer:         tracer,
		metrics:        metrics,
		system:         "openai",
	}
}

// SetSystem configures the inference system attribute (e.g. "openai", "google", "llama.cpp").
func (gc *InstrumentedOpenAIEmbeddingClient) SetSystem(system string) {
	gc.system = system
}

func (gc *InstrumentedOpenAIEmbeddingClient) GetModel() string { return gc.model }

func (gc *InstrumentedOpenAIEmbeddingClient) GetDimensionality() int {
	if gc.dimensionality == nil {
		return 0
	}
	return int(*gc.dimensionality)
}

func (gc *InstrumentedOpenAIEmbeddingClient) IsNormalized() bool { return gc.isNormalized }

// GetEmbedding generates an embedding for a single text with OpenTelemetry instrumentation.
func (gc *InstrumentedOpenAIEmbeddingClient) GetEmbedding(ctx context.Context, text string) (*codegen.EmbeddingResponse, error) {
	return gc.executeEmbedding(ctx, []string{text}, false)
}

// GetEmbeddings generates embeddings for a batch of texts with OpenTelemetry instrumentation.
func (gc *InstrumentedOpenAIEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) (*codegen.EmbeddingResponse, error) {
	return gc.executeEmbedding(ctx, texts, true)
}

func (gc *InstrumentedOpenAIEmbeddingClient) executeEmbedding(
	ctx context.Context,
	texts []string,
	isBatch bool,
) (*codegen.EmbeddingResponse, error) {
	batchCount := len(texts)
	dims := gc.GetDimensionality()
	startTime := time.Now()

	// 1. In-flight active request tracking
	if gc.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", gc.model),
		)
		gc.metrics.ActiveRequests.Add(ctx, 1, activeAttrs)
		defer gc.metrics.ActiveRequests.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry GenAI Client Span
	// Standard span naming convention: "embeddings {model_name}"
	ctx, span := gc.tracer.Start(ctx, fmt.Sprintf("embeddings %s", gc.model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.request.model", gc.model),
			attribute.Int("gen_ai.request.batch_size", batchCount),
			attribute.Int("gen_ai.response.dimensions", dims),
			attribute.Bool("gen_ai.embedding.normalized", gc.isNormalized),
		),
	)
	defer span.End()

	// 3. Execute underlying call
	var resp *codegen.EmbeddingResponse
	var err error

	if gc.client != nil {
		var input codegen.EmbeddingRequest_Input
		if isBatch {
			if inputErr := input.FromEmbeddingRequestInput1(texts); inputErr != nil {
				err = inputErr
			}
		} else if len(texts) > 0 {
			if inputErr := input.FromEmbeddingRequestInput0(texts[0]); inputErr != nil {
				err = inputErr
			}
		}

		if err == nil {
			var model codegen.EmbeddingRequest_Model
			if modelErr := model.FromEmbeddingRequestModel0(gc.model); modelErr != nil {
				err = modelErr
			} else {
				var dimsPtr *int
				if gc.dimensionality != nil {
					d := int(*gc.dimensionality)
					dimsPtr = &d
				}
				httpResp, httpErr := gc.client.CreateEmbeddingWithResponse(ctx, codegen.EmbeddingRequest{
					Dimensions:     dimsPtr,
					EncodingFormat: new(codegen.Float),
					Input:          input,
					Model:          model,
				})
				if httpErr != nil {
					err = httpErr
				} else if httpResp.HTTPResponse.StatusCode != http.StatusOK {
					err = fmt.Errorf("%s\t%s", httpResp.Status(), string(httpResp.Body))
				} else {
					resp = httpResp.JSON200
				}
			}
		}
	} else if gc.inner != nil {
		if isBatch {
			resp, err = gc.inner.GetEmbeddings(ctx, texts)
		} else if len(texts) > 0 {
			resp, err = gc.inner.GetEmbedding(ctx, texts[0])
		}
	} else {
		err = fmt.Errorf("no underlying client or embedder configured")
	}

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
				attribute.String("gen_ai.request.model", gc.model),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			gc.metrics.OperationDuration.Record(ctx, durationMs, errAttrs)
		}
		return nil, err
	}

	if resp == nil || len(resp.Data) == 0 {
		emptyErr := fmt.Errorf("embedding provider returned empty data")
		span.RecordError(emptyErr)
		span.SetStatus(codes.Error, emptyErr.Error())
		return nil, emptyErr
	}

	// 5. Enrich Span with Output Results & Token Usage
	promptTokens := resp.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = resp.Usage.TotalTokens
	}
	actualDims := len(resp.Data[0].Embedding)

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.response.dimensions", actualDims),
		attribute.Int("gen_ai.response.item_count", len(resp.Data)),
	)
	span.SetStatus(codes.Ok, "embeddings generated successfully")

	// 6. Record Industry Standard Prometheus Metrics
	if gc.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.String("gen_ai.system", gc.system),
			attribute.String("gen_ai.request.model", gc.model),
		}

		// Token usage counter with token.type="input"
		tokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		gc.metrics.TokenUsage.Add(ctx, int64(promptTokens), metric.WithAttributes(tokenAttrs...))

		// Latency histogram
		gc.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))

		// Batch size distribution
		gc.metrics.BatchSize.Record(ctx, int64(batchCount), metric.WithAttributes(baseAttrs...))
	}

	return resp, nil
}

// GetQueryEmbedding generates an embedding for retrieval search queries.
func (gc *InstrumentedOpenAIEmbeddingClient) GetQueryEmbedding(ctx context.Context, text string, taskType TaskType) (*codegen.EmbeddingResponse, error) {
	ctx, span := gc.tracer.Start(ctx, fmt.Sprintf("embedding.query: %s", taskType),
		trace.WithAttributes(
			attribute.String("gen_ai.embedding.task_type", string(taskType)),
		),
	)
	defer span.End()

	return gc.GetEmbedding(ctx, PrepareQuery(taskType, text))
}

// GetDocumentEmbedding generates an embedding for a structured document.
func (gc *InstrumentedOpenAIEmbeddingClient) GetDocumentEmbedding(ctx context.Context, content *Document) (*codegen.EmbeddingResponse, error) {
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
func (gc *InstrumentedOpenAIEmbeddingClient) GetDocumentEmbeddings(ctx context.Context, contents []*Document) (*codegen.EmbeddingResponse, error) {
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

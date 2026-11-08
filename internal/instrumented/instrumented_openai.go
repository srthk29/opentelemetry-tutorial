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

type summaryContextKey struct{}

type summaryMetadata struct {
	summaryType   string
	promptVersion int
	documentCount int
}

func withSummaryMetadata(ctx context.Context, meta summaryMetadata) context.Context {
	return context.WithValue(ctx, summaryContextKey{}, meta)
}

func getSummaryMetadata(ctx context.Context) (summaryMetadata, bool) {
	m, ok := ctx.Value(summaryContextKey{}).(summaryMetadata)
	return m, ok
}

// InstrumentedOpenAISummarizer wraps an OpenAI / local LLM chat completion client
// with official OpenTelemetry GenAI Semantic Conventions for traces and metrics.
type InstrumentedOpenAISummarizer struct {
	inner       SummarizerWithChatCompletionResponse
	client      codegen.ClientWithResponsesInterface
	modelName   string
	tracer      trace.Tracer
	metrics     *SummaryMetrics
	system      string
	temperature float32
	topP        float32
}

// Compile-time assertion that InstrumentedOpenAISummarizer implements SummarizerWithChatCompletionResponse.
var _ SummarizerWithChatCompletionResponse = (*InstrumentedOpenAISummarizer)(nil)

// NewInstrumentedOpenAISummarizer creates an instrumented OpenAI summarizer with default client and model.
func NewInstrumentedOpenAISummarizer(ctx context.Context, modelName string, metrics *SummaryMetrics) (*InstrumentedOpenAISummarizer, error) {
	if modelName == "" {
		modelName = "google/gemma-4-E4B-it-qat-q4_0-gguf"
	}
	client, err := codegen.NewClientWithResponses("http://127.0.0.1:8080/")
	if err != nil {
		return nil, err
	}

	if metrics == nil {
		var metricErr error
		metrics, metricErr = NewSummaryMetrics(nil)
		if metricErr != nil {
			return nil, fmt.Errorf("failed to initialize summary metrics: %w", metricErr)
		}
	}

	return &InstrumentedOpenAISummarizer{
		client:      client,
		modelName:   modelName,
		tracer:      otel.Tracer("openai-summarizer"),
		metrics:     metrics,
		system:      "openai",
		temperature: 0.2,
		topP:        0.9,
	}, nil
}

// NewInstrumentedOpenAISummarizerWithClient creates an instrumented summarizer decorating a codegen client.
func NewInstrumentedOpenAISummarizerWithClient(
	client codegen.ClientWithResponsesInterface,
	modelName string,
	tracer trace.Tracer,
	metrics *SummaryMetrics,
) *InstrumentedOpenAISummarizer {
	if tracer == nil {
		tracer = otel.Tracer("openai-summarizer")
	}
	if modelName == "" {
		modelName = "google/gemma-4-E4B-it-qat-q4_0-gguf"
	}
	return &InstrumentedOpenAISummarizer{
		client:      client,
		modelName:   modelName,
		tracer:      tracer,
		metrics:     metrics,
		system:      "openai",
		temperature: 0.2,
		topP:        0.9,
	}
}

// NewInstrumentedOpenAISummarizerFromInner wraps an existing SummarizerWithChatCompletionResponse.
func NewInstrumentedOpenAISummarizerFromInner(
	inner SummarizerWithChatCompletionResponse,
	tracer trace.Tracer,
	metrics *SummaryMetrics,
) *InstrumentedOpenAISummarizer {
	if tracer == nil {
		tracer = otel.Tracer("openai-summarizer")
	}
	modelName := "google/gemma-4-E4B-it-qat-q4_0-gguf"
	if inner != nil {
		modelName = inner.model()
	}
	return &InstrumentedOpenAISummarizer{
		inner:       inner,
		modelName:   modelName,
		tracer:      tracer,
		metrics:     metrics,
		system:      "openai",
		temperature: 0.2,
		topP:        0.9,
	}
}

// SetSystem configures the provider system attribute (e.g. "openai", "google", "llama.cpp").
func (ios *InstrumentedOpenAISummarizer) SetSystem(system string) {
	ios.system = system
}

// SetHyperparameters configures temperature and top_p.
func (ios *InstrumentedOpenAISummarizer) SetHyperparameters(temperature, topP float32) {
	ios.temperature = temperature
	ios.topP = topP
}

// Model returns the configured model identifier.
func (ios *InstrumentedOpenAISummarizer) Model() string {
	return ios.modelName
}

func (ios *InstrumentedOpenAISummarizer) model() string {
	return ios.Model()
}

// Summary executes a chat completion request with OpenTelemetry GenAI Semantic Conventions.
func (ios *InstrumentedOpenAISummarizer) Summary(ctx context.Context, msgs []codegen.ChatCompletionRequestMessage) (*codegen.ChatCompletionResponse, error) {
	startTime := time.Now()

	// 1. In-flight active request tracking
	if ios.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", ios.system),
			attribute.String("gen_ai.request.model", ios.modelName),
		)
		ios.metrics.ActiveRequests.Add(ctx, 1, activeAttrs)
		defer ios.metrics.ActiveRequests.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry GenAI Client Span
	// Standard naming: "chat {model_name}"
	spanAttrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", ios.system),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", ios.modelName),
		attribute.Float64("gen_ai.request.temperature", float64(ios.temperature)),
		attribute.Float64("gen_ai.request.top_p", float64(ios.topP)),
		attribute.Int("gen_ai.request.messages_count", len(msgs)),
	}

	meta, hasMeta := getSummaryMetadata(ctx)
	if hasMeta {
		if meta.summaryType != "" {
			spanAttrs = append(spanAttrs, attribute.String("gen_ai.summary.type", meta.summaryType))
		}
		if meta.promptVersion > 0 {
			spanAttrs = append(spanAttrs, attribute.Int("gen_ai.request.prompt_version", meta.promptVersion))
		}
		if meta.documentCount > 1 {
			spanAttrs = append(spanAttrs, attribute.Int("gen_ai.request.document_count", meta.documentCount))
		}
	}

	ctx, span := ios.tracer.Start(ctx, fmt.Sprintf("chat %s", ios.modelName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	// 3. Execute underlying HTTP call or inner summarizer
	var resp *codegen.ChatCompletionResponse
	var err error

	if ios.client != nil {
		req := codegen.ChatCompletionRequest{
			Model: func() codegen.ModelIdsShared {
				var model codegen.ModelIdsShared
				_ = model.FromModelIdsShared0(ios.modelName)
				return model
			}(),
			Temperature: func() *codegen.ChatCompletionRequest_Temperature {
				var temp codegen.ChatCompletionRequest_Temperature
				_ = temp.FromChatCompletionRequestTemperature0(ios.temperature)
				return &temp
			}(),
			TopP: func() *codegen.ChatCompletionRequest_TopP {
				var topP codegen.ChatCompletionRequest_TopP
				_ = topP.FromChatCompletionRequestTopP0(ios.topP)
				return &topP
			}(),
			FrequencyPenalty: new(float32(0.0)),
			PresencePenalty:  new(float32(0.0)),
			Stream:           new(false),
			Messages:         msgs,
		}

		httpResp, httpErr := ios.client.CreateChatCompletionWithResponse(ctx, req)
		if httpErr != nil {
			err = httpErr
		} else if httpResp.HTTPResponse.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s\t%s", httpResp.Status(), string(httpResp.Body))
		} else {
			resp = httpResp.JSON200
		}
	} else if ios.inner != nil {
		resp, err = ios.inner.summary(ctx, msgs)
	} else {
		err = fmt.Errorf("no underlying client or summarizer configured")
	}

	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)

		if ios.metrics != nil {
			errAttrs := []attribute.KeyValue{
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.system", ios.system),
				attribute.String("gen_ai.request.model", ios.modelName),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			}
			if hasMeta && meta.summaryType != "" {
				errAttrs = append(errAttrs, attribute.String("gen_ai.summary.type", meta.summaryType))
			}
			ios.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(errAttrs...))
		}
		return nil, err
	}

	if resp == nil || len(resp.Choices) == 0 {
		emptyErr := fmt.Errorf("chat completion provider returned empty choices")
		span.RecordError(emptyErr)
		span.SetStatus(codes.Error, emptyErr.Error())
		return nil, emptyErr
	}

	// 5. Enrich Span with Output Results & Token Usage
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	if resp.Usage != nil {
		promptTokens = resp.Usage.PromptTokens
		completionTokens = resp.Usage.CompletionTokens
		totalTokens = resp.Usage.TotalTokens
		if totalTokens == 0 {
			totalTokens = promptTokens + completionTokens
		}
	}

	finishReasons := make([]string, len(resp.Choices))
	for i, c := range resp.Choices {
		finishReasons[i] = c.FinishReason
	}

	span.SetAttributes(
		attribute.String("gen_ai.response.id", resp.ID),
		attribute.String("gen_ai.response.model", resp.Model),
		attribute.StringSlice("gen_ai.response.finish_reasons", finishReasons),
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.usage.output_tokens", completionTokens),
		attribute.Int("gen_ai.usage.total_tokens", totalTokens),
	)
	span.SetStatus(codes.Ok, "chat completion generated successfully")

	// 6. Record Industry Standard Prometheus Metrics
	if ios.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", ios.system),
			attribute.String("gen_ai.request.model", ios.modelName),
		}
		if hasMeta && meta.summaryType != "" {
			baseAttrs = append(baseAttrs, attribute.String("gen_ai.summary.type", meta.summaryType))
		}

		// Input token counter
		inputTokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		ios.metrics.TokenUsage.Add(ctx, int64(promptTokens), metric.WithAttributes(inputTokenAttrs...))

		// Output token counter
		outputTokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "output"))
		ios.metrics.TokenUsage.Add(ctx, int64(completionTokens), metric.WithAttributes(outputTokenAttrs...))

		// Latency histogram
		ios.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))

		// Batch size histogram if multiple documents
		if hasMeta && meta.documentCount > 0 {
			ios.metrics.DocumentBatchSize.Record(ctx, int64(meta.documentCount), metric.WithAttributes(baseAttrs...))
		}
	}

	return resp, nil
}

func (ios *InstrumentedOpenAISummarizer) summary(ctx context.Context, msgs []codegen.ChatCompletionRequestMessage) (*codegen.ChatCompletionResponse, error) {
	return ios.Summary(ctx, msgs)
}

// GetSummary generates a concise summary with tracing context.
func (ios *InstrumentedOpenAISummarizer) GetSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	ctx = withSummaryMetadata(ctx, summaryMetadata{summaryType: "short", documentCount: 1})
	sysMsg, err := systemMsg(ShortSummarySystemPrompt)
	if err != nil {
		return nil, err
	}
	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}
	return ios.Summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (ios *InstrumentedOpenAISummarizer) getSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	return ios.GetSummary(ctx, text)
}

// GetLongSummary generates a comprehensive summary with tracing context.
func (ios *InstrumentedOpenAISummarizer) GetLongSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	ctx = withSummaryMetadata(ctx, summaryMetadata{summaryType: "long", documentCount: 1})
	sysMsg, err := systemMsg(SummarySystemPrompt)
	if err != nil {
		return nil, err
	}
	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}
	return ios.Summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (ios *InstrumentedOpenAISummarizer) getLongSummary(ctx context.Context, text string) (*codegen.ChatCompletionResponse, error) {
	return ios.GetLongSummary(ctx, text)
}

// GetLongSummaryWithPromptVersion generates a versioned summary with tracing context.
func (ios *InstrumentedOpenAISummarizer) GetLongSummaryWithPromptVersion(ctx context.Context, text string, promptVerison int) (*codegen.ChatCompletionResponse, error) {
	ctx = withSummaryMetadata(ctx, summaryMetadata{
		summaryType:   fmt.Sprintf("long_v%d", promptVerison),
		promptVersion: promptVerison,
		documentCount: 1,
	})
	sysMsg, err := systemMsg(longSummarySystemPromptVersion(PromptVersion(promptVerison)))
	if err != nil {
		return nil, err
	}
	userMsg, err := userMsg(text)
	if err != nil {
		return nil, err
	}
	return ios.Summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsg})
}

func (ios *InstrumentedOpenAISummarizer) getLongSummaryWithPromptVersion(ctx context.Context, text string, promptVerison int) (*codegen.ChatCompletionResponse, error) {
	return ios.GetLongSummaryWithPromptVersion(ctx, text, promptVerison)
}

// GetMergedSummary generates a multi-document merged summary with tracing context.
func (ios *InstrumentedOpenAISummarizer) GetMergedSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	ctx = withSummaryMetadata(ctx, summaryMetadata{
		summaryType:   "merged",
		documentCount: len(texts),
	})
	sysMsg, err := systemMsg(MergeSummarySystemPrompt)
	if err != nil {
		return nil, err
	}
	userMsgs, err := userMsgs(texts)
	if err != nil {
		return nil, err
	}
	return ios.Summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsgs})
}

func (ios *InstrumentedOpenAISummarizer) getMergedSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	return ios.GetMergedSummary(ctx, texts)
}

// GetMergedLongSummary generates a comprehensive multi-document merged summary with tracing context.
func (ios *InstrumentedOpenAISummarizer) GetMergedLongSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	ctx = withSummaryMetadata(ctx, summaryMetadata{
		summaryType:   "merged_long",
		documentCount: len(texts),
	})
	sysMsg, err := systemMsg(MergeLongSummarySystemPrompt)
	if err != nil {
		return nil, err
	}
	userMsgs, err := userMsgs(texts)
	if err != nil {
		return nil, err
	}
	return ios.Summary(ctx, []codegen.ChatCompletionRequestMessage{*sysMsg, *userMsgs})
}

func (ios *InstrumentedOpenAISummarizer) getMergedLongSummary(ctx context.Context, texts []string) (*codegen.ChatCompletionResponse, error) {
	return ios.GetMergedLongSummary(ctx, texts)
}

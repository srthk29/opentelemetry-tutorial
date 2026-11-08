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

// GoogleGemmaIVSummarizer is the uninstrumented base summarizer.
type GoogleGemmaIVSummarizer struct {
	s SummarizerWithChatCompletionResponse
}

// Compile-time assertion that GoogleGemmaIVSummarizer implements Summarizer.
var _ Summarizer = (*GoogleGemmaIVSummarizer)(nil)

// NewGoogleGemmaIVSummarizer creates a new uninstrumented summarizer with default model.
func NewGoogleGemmaIVSummarizer(ctx context.Context) (Summarizer, error) {
	gs, err := NewOpenAISummarizer(ctx, "google/gemma-4-E4B-it-qat-q4_0-gguf")
	if err != nil {
		return nil, err
	}
	return &GoogleGemmaIVSummarizer{s: gs}, nil
}

// Client returns the underlying SummarizerWithChatCompletionResponse.
func (gs *GoogleGemmaIVSummarizer) Client() SummarizerWithChatCompletionResponse {
	return gs.s
}

/*
	curl http://localhost:8080/v1/chat/completions \
	  -H "Content-Type: application/json" \
	  -H "Accept: application/json" \
	  -d '{
	    "messages": [
	      {
	        "role": "system",
	        "content": "You are a concise data processor."
	      },
	      {
	        "role": "user",
	        "content": "Extract the key entities from this text..."
	      }
	    ],
	    "model": "gemma",
	    "temperature": 0.1,
	    "top_p": 1.0,
	    "max_tokens": 1024,
	    "stream": false,
	    "frequency_penalty": 0.0,
	    "presence_penalty": 0.0
	  }'
*/
func (gs *GoogleGemmaIVSummarizer) Model() string { return gs.s.model() }

func (gs *GoogleGemmaIVSummarizer) GetSummary(ctx context.Context, text string) (string, error) {
	respJSON200, err := gs.s.getSummary(ctx, text)
	if err != nil {
		return "", err
	}

	return respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
}

func (gs *GoogleGemmaIVSummarizer) GetLongSummary(ctx context.Context, text string) (string, error) {
	respJSON200, err := gs.s.getLongSummary(ctx, text)
	if err != nil {
		return "", err
	}

	return respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
}

func (gs *GoogleGemmaIVSummarizer) GetLongSummaryWithPromptVersion(ctx context.Context, text string, promptVerison int) (string, error) {
	respJSON200, err := gs.s.getLongSummaryWithPromptVersion(ctx, text, promptVerison)
	if err != nil {
		return "", err
	}

	return respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
}

func (gs *GoogleGemmaIVSummarizer) GetMergedSummary(ctx context.Context, texts []string) (string, error) {
	respJSON200, err := gs.s.getMergedSummary(ctx, texts)
	if err != nil {
		return "", err
	}

	return respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
}

func (gs *GoogleGemmaIVSummarizer) GetMergedLongSummary(ctx context.Context, texts []string) (string, error) {
	respJSON200, err := gs.s.getMergedLongSummary(ctx, texts)
	if err != nil {
		return "", err
	}

	return respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
}

// InstrumentedGoogleGemmaIVSummarizer decorates a summarizer with official
// OpenTelemetry GenAI Semantic Conventions for traces and metrics.
type InstrumentedGoogleGemmaIVSummarizer struct {
	summarizer  SummarizerWithChatCompletionResponse
	tracer      trace.Tracer
	metrics     *SummaryMetrics
	system      string
	temperature float64
	topP        float64
	maxTokens   int
}

// Compile-time assertion that InstrumentedGoogleGemmaIVSummarizer implements Summarizer.
var _ Summarizer = (*InstrumentedGoogleGemmaIVSummarizer)(nil)

// NewInstrumentedGoogleGemmaIVSummarizer initializes the instrumented Gemma summarizer with default model and metrics.
func NewInstrumentedGoogleGemmaIVSummarizer(ctx context.Context, metrics *SummaryMetrics) (Summarizer, error) {
	gs, err := NewOpenAISummarizer(ctx, "google/gemma-4-E4B-it-qat-q4_0-gguf")
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

	return &InstrumentedGoogleGemmaIVSummarizer{
		summarizer:  gs,
		tracer:      otel.Tracer("gemma-summarizer"),
		metrics:     metrics,
		system:      "google",
		temperature: 0.1,
		topP:        1.0,
		maxTokens:   1024,
	}, nil
}

// NewInstrumentedGoogleGemmaIVSummarizerWithClient initializes the instrumented summarizer with a custom client.
func NewInstrumentedGoogleGemmaIVSummarizerWithClient(
	summarizer SummarizerWithChatCompletionResponse,
	tracer trace.Tracer,
	metrics *SummaryMetrics,
) *InstrumentedGoogleGemmaIVSummarizer {
	if tracer == nil {
		tracer = otel.Tracer("gemma-summarizer")
	}
	return &InstrumentedGoogleGemmaIVSummarizer{
		summarizer:  summarizer,
		tracer:      tracer,
		metrics:     metrics,
		system:      "google",
		temperature: 0.1,
		topP:        1.0,
		maxTokens:   1024,
	}
}

// Model returns the underlying model identifier.
func (is *InstrumentedGoogleGemmaIVSummarizer) Model() string {
	return is.summarizer.model()
}

// SetHyperparameters configures inference parameters recorded in trace spans.
func (is *InstrumentedGoogleGemmaIVSummarizer) SetHyperparameters(temperature, topP float64, maxTokens int) {
	is.temperature = temperature
	is.topP = topP
	is.maxTokens = maxTokens
}

// executeChatCall encapsulates the full OpenTelemetry GenAI client span lifecycle and metrics.
func (is *InstrumentedGoogleGemmaIVSummarizer) executeChatCall(
	ctx context.Context,
	summaryType string,
	promptVersion int,
	docCount int,
	fn func(ctx context.Context) (*OpenAIChatCompletionResponse, error),
) (string, error) {
	model := is.summarizer.model()
	startTime := time.Now()

	// 1. In-flight active request tracking
	if is.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", is.system),
			attribute.String("gen_ai.request.model", model),
		)
		is.metrics.ActiveRequests.Add(ctx, 1, activeAttrs)
		defer is.metrics.ActiveRequests.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry GenAI Client Span
	// Standard span naming convention: "chat {model_name}"
	spanAttrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", is.system),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", model),
		attribute.Float64("gen_ai.request.temperature", is.temperature),
		attribute.Float64("gen_ai.request.top_p", is.topP),
		attribute.Int("gen_ai.request.max_tokens", is.maxTokens),
		attribute.String("gen_ai.summary.type", summaryType),
	}
	if promptVersion > 0 {
		spanAttrs = append(spanAttrs, attribute.Int("gen_ai.request.prompt_version", promptVersion))
	}
	if docCount > 1 {
		spanAttrs = append(spanAttrs, attribute.Int("gen_ai.request.document_count", docCount))
	}

	ctx, span := is.tracer.Start(ctx, fmt.Sprintf("chat %s", model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	// 3. Execute underlying inference call
	respJSON200, err := fn(ctx)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)

		if is.metrics != nil {
			errAttrs := metric.WithAttributes(
				attribute.String("gen_ai.operation.name", "chat"),
				attribute.String("gen_ai.system", is.system),
				attribute.String("gen_ai.request.model", model),
				attribute.String("gen_ai.summary.type", summaryType),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			is.metrics.OperationDuration.Record(ctx, durationMs, errAttrs)
		}
		return "", err
	}

	if len(respJSON200.Choices) == 0 {
		emptyErr := fmt.Errorf("summarization provider returned empty choices array")
		span.RecordError(emptyErr)
		span.SetStatus(codes.Error, emptyErr.Error())
		return "", emptyErr
	}

	content, contentErr := respJSON200.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
	if contentErr != nil {
		span.RecordError(contentErr)
		span.SetStatus(codes.Error, contentErr.Error())
		return "", contentErr
	}

	// 5. Enrich Span with Output Results & Token Usage
	promptTokens := respJSON200.Usage.PromptTokens
	completionTokens := respJSON200.Usage.CompletionTokens
	totalTokens := respJSON200.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}

	span.SetAttributes(
		attribute.String("gen_ai.response.id", respJSON200.ID),
		attribute.String("gen_ai.response.model", respJSON200.Model),
		attribute.StringSlice("gen_ai.response.finish_reasons", []string{respJSON200.Choices[0].FinishReason}),
		attribute.Int("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int("gen_ai.usage.output_tokens", completionTokens),
		attribute.Int("gen_ai.usage.total_tokens", totalTokens),
	)
	span.SetStatus(codes.Ok, "summary generated successfully")

	// 6. Record Industry Standard Prometheus Metrics
	if is.metrics != nil {
		baseAttrs := []attribute.KeyValue{
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.String("gen_ai.system", is.system),
			attribute.String("gen_ai.request.model", model),
			attribute.String("gen_ai.summary.type", summaryType),
		}

		// Token usage counter with token.type="input"
		inputTokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "input"))
		is.metrics.TokenUsage.Add(ctx, int64(promptTokens), metric.WithAttributes(inputTokenAttrs...))

		// Token usage counter with token.type="output"
		outputTokenAttrs := append(baseAttrs, attribute.String("gen_ai.token.type", "output"))
		is.metrics.TokenUsage.Add(ctx, int64(completionTokens), metric.WithAttributes(outputTokenAttrs...))

		// Operation latency histogram
		is.metrics.OperationDuration.Record(ctx, durationMs, metric.WithAttributes(baseAttrs...))

		// Document batch size histogram (for merged summaries)
		if docCount > 0 {
			is.metrics.DocumentBatchSize.Record(ctx, int64(docCount), metric.WithAttributes(baseAttrs...))
		}
	}

	return content, nil
}

// GetSummary generates a concise summary of the input text with OpenTelemetry instrumentation.
func (is *InstrumentedGoogleGemmaIVSummarizer) GetSummary(ctx context.Context, text string) (string, error) {
	return is.executeChatCall(ctx, "short", 0, 1, func(callCtx context.Context) (*OpenAIChatCompletionResponse, error) {
		return is.summarizer.getSummary(callCtx, text)
	})
}

// GetLongSummary generates a comprehensive summary with OpenTelemetry instrumentation.
func (is *InstrumentedGoogleGemmaIVSummarizer) GetLongSummary(ctx context.Context, text string) (string, error) {
	return is.executeChatCall(ctx, "long", 0, 1, func(callCtx context.Context) (*OpenAIChatCompletionResponse, error) {
		return is.summarizer.getLongSummary(callCtx, text)
	})
}

// GetLongSummaryWithPromptVersion generates a versioned summary with OpenTelemetry instrumentation.
func (is *InstrumentedGoogleGemmaIVSummarizer) GetLongSummaryWithPromptVersion(ctx context.Context, text string, promptVersion int) (string, error) {
	return is.executeChatCall(ctx, fmt.Sprintf("long_v%d", promptVersion), promptVersion, 1, func(callCtx context.Context) (*OpenAIChatCompletionResponse, error) {
		return is.summarizer.getLongSummaryWithPromptVersion(callCtx, text, promptVersion)
	})
}

// GetMergedSummary generates a combined summary from multiple texts with OpenTelemetry instrumentation.
func (is *InstrumentedGoogleGemmaIVSummarizer) GetMergedSummary(ctx context.Context, texts []string) (string, error) {
	return is.executeChatCall(ctx, "merged", 0, len(texts), func(callCtx context.Context) (*OpenAIChatCompletionResponse, error) {
		return is.summarizer.getMergedSummary(callCtx, texts)
	})
}

// GetMergedLongSummary generates a detailed combined summary from multiple texts with OpenTelemetry instrumentation.
func (is *InstrumentedGoogleGemmaIVSummarizer) GetMergedLongSummary(ctx context.Context, texts []string) (string, error) {
	return is.executeChatCall(ctx, "merged_long", 0, len(texts), func(callCtx context.Context) (*OpenAIChatCompletionResponse, error) {
		return is.summarizer.getMergedLongSummary(callCtx, texts)
	})
}

package stages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// SummarizerPipeline synthesizes transcripts and chunks into an executive summary.
// Notice: ZERO OpenTelemetry imports! Pure business stage.
type SummarizerPipeline struct {
	model string
}

var _ pipeline.Pipeline = (*SummarizerPipeline)(nil)

func NewSummarizerPipeline(model string) *SummarizerPipeline {
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &SummarizerPipeline{model: model}
}

func (p *SummarizerPipeline) Name() string {
	return "llm-executive-summarizer"
}

func (p *SummarizerPipeline) Stage() request.PipelineStage {
	return request.StageSummarizer
}

func (p *SummarizerPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	if params.TranscriptText == "" {
		return errors.New("cannot summarize empty text")
	}

	// Simulate LLM inference generation latency (60-120ms)
	time.Sleep(75 * time.Millisecond)

	inputTokens := len(params.TranscriptText) / 4
	summary := "Executive Summary: Comprehensive tutorial exploring OpenTelemetry integration in Go, " +
		"covering Clean Architecture, generic pipeline telemetry decorators, and GenAI semantic conventions."
	outputTokens := len(summary) / 4

	params.Summary = summary
	params.SetMeta("gen_ai.request.model", p.model)
	params.SetMeta("gen_ai.usage.input_tokens", inputTokens)
	params.SetMeta("gen_ai.usage.output_tokens", outputTokens)

	fmt.Printf("  [Stage: %s] Generated summary with model '%s' (in: %d tokens, out: %d tokens)\n",
		p.Stage(), p.model, inputTokens, outputTokens)
	return nil
}

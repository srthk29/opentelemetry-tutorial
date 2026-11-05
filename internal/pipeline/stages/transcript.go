package stages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// TranscriptPipeline simulates transcribing an audio file into text.
// Notice: ZERO OpenTelemetry imports! Pure business stage.
type TranscriptPipeline struct{}

var _ pipeline.Pipeline = (*TranscriptPipeline)(nil)

func NewTranscriptPipeline() *TranscriptPipeline {
	return &TranscriptPipeline{}
}

func (p *TranscriptPipeline) Name() string {
	return "whisper-speech-to-text"
}

func (p *TranscriptPipeline) Stage() request.PipelineStage {
	return request.StageTranscript
}

func (p *TranscriptPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	if params.AudioURI == "" {
		return errors.New("missing audio_uri in request params")
	}

	// Simulate speech-to-text processing time (30-60ms)
	time.Sleep(40 * time.Millisecond)

	// Simulated transcription output
	sampleTranscript := "Welcome to this technical tutorial on OpenTelemetry integration in Go. " +
		"In this session, we explore multi-stage data processing pipelines, distributed tracing, " +
		"and GenAI semantic conventions including embeddings, token usage, and summarization metrics. " +
		"By using Clean Architecture and the Decorator Pattern, we keep our business logic pure."

	params.TranscriptText = sampleTranscript
	words := len(strings.Fields(sampleTranscript))

	// Attach stage business metadata (picked up by the TelemetryDecorator)
	params.SetMeta("transcript.word_count", words)
	params.SetMeta("audio.duration_sec", 124.5)
	params.SetMeta("stt.model", "whisper-large-v3")

	fmt.Printf("  [Stage: %s] Transcribed %d words from %s\n", p.Stage(), words, params.AudioURI)
	return nil
}

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

// ChunkingPipeline splits long documents or transcripts into manageable chunks for embeddings.
// Notice: ZERO OpenTelemetry imports! Pure business stage.
type ChunkingPipeline struct {
	chunkSize int
}

var _ pipeline.Pipeline = (*ChunkingPipeline)(nil)

func NewChunkingPipeline(chunkSize int) *ChunkingPipeline {
	if chunkSize <= 0 {
		chunkSize = 100
	}
	return &ChunkingPipeline{chunkSize: chunkSize}
}

func (p *ChunkingPipeline) Name() string {
	return "recursive-character-text-splitter"
}

func (p *ChunkingPipeline) Stage() request.PipelineStage {
	return request.StageChunking
}

func (p *ChunkingPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	if params.TranscriptText == "" {
		return errors.New("cannot chunk empty transcript text")
	}

	// Simulate chunking latency (10-20ms)
	time.Sleep(15 * time.Millisecond)

	// Split text by sentence or fixed window
	sentences := strings.Split(params.TranscriptText, ". ")
	var chunks []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) > 0 {
			chunks = append(chunks, s)
		}
	}

	params.Chunks = chunks
	params.SetMeta("chunk.count", len(chunks))
	params.SetMeta("chunk.strategy", "sentence_boundary")
	params.SetMeta("chunk.size_chars", p.chunkSize)

	fmt.Printf("  [Stage: %s] Generated %d text chunks\n", p.Stage(), len(chunks))
	return nil
}

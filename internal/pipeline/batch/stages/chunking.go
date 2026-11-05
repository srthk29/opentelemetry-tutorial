package stages

import (
	"context"
	"fmt"
	"strings"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// BatchChunkingPipeline segments all documents in the batch into smaller textual chunks.
// Pure Go implementation with ZERO OpenTelemetry imports.
type BatchChunkingPipeline struct {
	maxChunkWords int
}

var _ pipeline.Pipeline = (*BatchChunkingPipeline)(nil)

// NewBatchChunkingPipeline creates a new batch chunking stage.
func NewBatchChunkingPipeline(maxChunkWords int) *BatchChunkingPipeline {
	if maxChunkWords <= 0 {
		maxChunkWords = 50
	}
	return &BatchChunkingPipeline{maxChunkWords: maxChunkWords}
}

func (p *BatchChunkingPipeline) Name() string {
	return "batch-chunking"
}

func (p *BatchChunkingPipeline) Stage() request.PipelineStage {
	return request.StageChunking
}

func (p *BatchChunkingPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	docs, err := batch.GetDocuments(params)
	if err != nil {
		return fmt.Errorf("chunking stage failed: %w", err)
	}

	for _, doc := range docs {
		words := strings.Fields(doc.Content)
		if len(words) == 0 {
			continue
		}

		doc.Chunks = make([]batch.ChunkItem, 0)
		chunkIdx := 0

		for i := 0; i < len(words); i += p.maxChunkWords {
			end := i + p.maxChunkWords
			if end > len(words) {
				end = len(words)
			}

			chunkText := strings.Join(words[i:end], " ")
			doc.Chunks = append(doc.Chunks, batch.ChunkItem{
				ID:         fmt.Sprintf("%s-chunk-%03d", doc.ID, chunkIdx),
				DocumentID: doc.ID,
				Index:      chunkIdx,
				Text:       chunkText,
			})
			chunkIdx++
		}
	}

	return nil
}

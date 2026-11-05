package stages

import (
	"context"
	"fmt"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/modelmanager"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// BatchSummarizerPipeline executes LLM summarization over all chunks across all documents.
// Pure Go implementation with ZERO OpenTelemetry imports.
type BatchSummarizerPipeline struct {
	modelManager modelmanager.ModelManager
	modelName    string
	modelSize    int64
	summarizer   batch.ChunkSummarizer
}

var _ pipeline.Pipeline = (*BatchSummarizerPipeline)(nil)

// NewBatchSummarizerPipeline creates a batch summarizer stage.
func NewBatchSummarizerPipeline(
	modelMgr modelmanager.ModelManager,
	modelName string,
	modelSizeBytes int64,
	summarizer batch.ChunkSummarizer,
) *BatchSummarizerPipeline {
	if modelSizeBytes <= 0 {
		modelSizeBytes = 4_500_000_000 // default 4.5 GB for 8B LLM Q4
	}
	return &BatchSummarizerPipeline{
		modelManager: modelMgr,
		modelName:    modelName,
		modelSize:    modelSizeBytes,
		summarizer:   summarizer,
	}
}

func (p *BatchSummarizerPipeline) Name() string {
	return "batch-summarizer"
}

func (p *BatchSummarizerPipeline) Stage() request.PipelineStage {
	return request.StageSummarizer
}

func (p *BatchSummarizerPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	docs, err := batch.GetDocuments(params)
	if err != nil {
		return fmt.Errorf("summarizer stage failed: %w", err)
	}

	// 1. Memory constraint: Load LLM model into VRAM
	// This will automatically evict any previously loaded model (e.g. embeddings or whisper).
	release, err := p.modelManager.LoadModel(ctx, p.modelName, p.modelSize)
	if err != nil {
		return fmt.Errorf("failed to load model %s: %w", p.modelName, err)
	}
	defer release(ctx) // Always free memory/VRAM once the batch stage completes!

	// 2. Process ALL chunks across ALL documents with the model resident in VRAM
	for _, doc := range docs {
		for i := range doc.Chunks {
			if err := p.summarizer.SummarizeChunk(ctx, &doc.Chunks[i]); err != nil {
				return fmt.Errorf("failed summarizing chunk %s: %w", doc.Chunks[i].ID, err)
			}
		}
	}

	return nil
}

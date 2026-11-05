package stages

import (
	"context"
	"fmt"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/modelmanager"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// BatchEmbeddingsPipeline computes vector embeddings over all chunks across all documents.
// Pure Go implementation with ZERO OpenTelemetry imports.
type BatchEmbeddingsPipeline struct {
	modelManager modelmanager.ModelManager
	modelName    string
	modelSize    int64
	embedder     batch.ChunkEmbedder
}

var _ pipeline.Pipeline = (*BatchEmbeddingsPipeline)(nil)

// NewBatchEmbeddingsPipeline creates a batch embeddings stage.
func NewBatchEmbeddingsPipeline(
	modelMgr modelmanager.ModelManager,
	modelName string,
	modelSizeBytes int64,
	embedder batch.ChunkEmbedder,
) *BatchEmbeddingsPipeline {
	if modelSizeBytes <= 0 {
		modelSizeBytes = 1_300_000_000 // default 1.3 GB for large embedding model
	}
	return &BatchEmbeddingsPipeline{
		modelManager: modelMgr,
		modelName:    modelName,
		modelSize:    modelSizeBytes,
		embedder:     embedder,
	}
}

func (p *BatchEmbeddingsPipeline) Name() string {
	return "batch-embeddings"
}

func (p *BatchEmbeddingsPipeline) Stage() request.PipelineStage {
	return request.StageEmbeddings
}

func (p *BatchEmbeddingsPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	docs, err := batch.GetDocuments(params)
	if err != nil {
		return fmt.Errorf("embeddings stage failed: %w", err)
	}

	// 1. Memory constraint: Evict LLM and load Embedding model into VRAM
	release, err := p.modelManager.LoadModel(ctx, p.modelName, p.modelSize)
	if err != nil {
		return fmt.Errorf("failed to load model %s: %w", p.modelName, err)
	}
	defer release(ctx) // Always free memory/VRAM once the batch stage completes!

	// 2. Process ALL chunks across ALL documents with the model resident in VRAM
	for _, doc := range docs {
		for i := range doc.Chunks {
			if err := p.embedder.EmbedChunk(ctx, &doc.Chunks[i]); err != nil {
				return fmt.Errorf("failed embedding chunk %s: %w", doc.Chunks[i].ID, err)
			}
		}
	}

	return nil
}

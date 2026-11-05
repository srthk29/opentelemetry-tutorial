package stages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// EmbeddingsPipeline generates vector embeddings for each chunk via an embedding model.
// Notice: ZERO OpenTelemetry imports! Pure business stage.
type EmbeddingsPipeline struct {
	model      string
	dimensions int
}

var _ pipeline.Pipeline = (*EmbeddingsPipeline)(nil)

func NewEmbeddingsPipeline(model string, dimensions int) *EmbeddingsPipeline {
	if model == "" {
		model = "text-embedding-3-small"
	}
	if dimensions <= 0 {
		dimensions = 1536
	}
	return &EmbeddingsPipeline{
		model:      model,
		dimensions: dimensions,
	}
}

func (p *EmbeddingsPipeline) Name() string {
	return "openai-embeddings-generator"
}

func (p *EmbeddingsPipeline) Stage() request.PipelineStage {
	return request.StageEmbeddings
}

func (p *EmbeddingsPipeline) Run(ctx context.Context, params *request.FilterParams) error {
	if len(params.Chunks) == 0 {
		return errors.New("cannot generate embeddings: no chunks provided")
	}

	// Simulate vector embedding API network latency (40-80ms)
	time.Sleep(50 * time.Millisecond)

	// Simulate generating 1536-dimensional vectors for each chunk
	embeddings := make([][]float32, len(params.Chunks))
	totalTokens := 0
	for i, chunk := range params.Chunks {
		embeddings[i] = make([]float32, 4) // lightweight representation for demo
		totalTokens += len(chunk) / 4      // rough heuristic: 1 token ≈ 4 characters
	}

	params.Embeddings = embeddings
	params.SetMeta("gen_ai.request.model", p.model)
	params.SetMeta("gen_ai.usage.input_tokens", totalTokens)
	params.SetMeta("embedding.dimensions", p.dimensions)
	params.SetMeta("embedding.vector_count", len(embeddings))

	fmt.Printf("  [Stage: %s] Created %d vectors using model '%s' (%d tokens)\n",
		p.Stage(), len(embeddings), p.model, totalTokens)
	return nil
}

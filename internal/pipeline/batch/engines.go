package batch

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ChunkSummarizer defines the contract for summarizing an individual chunk.
// Pure Go domain interface with zero OpenTelemetry imports.
type ChunkSummarizer interface {
	ModelName() string
	SummarizeChunk(ctx context.Context, chunk *ChunkItem) error
}

// ChunkEmbedder defines the contract for generating vector embeddings for a chunk.
// Pure Go domain interface with zero OpenTelemetry imports.
type ChunkEmbedder interface {
	ModelName() string
	Dimensions() int
	EmbedChunk(ctx context.Context, chunk *ChunkItem) error
}

// LocalLlamaSummarizer simulates a local LLM summarizer (e.g., llama.cpp).
type LocalLlamaSummarizer struct {
	modelName string
}

func NewLocalLlamaSummarizer(modelName string) *LocalLlamaSummarizer {
	return &LocalLlamaSummarizer{modelName: modelName}
}

func (s *LocalLlamaSummarizer) ModelName() string {
	return s.modelName
}

func (s *LocalLlamaSummarizer) SummarizeChunk(ctx context.Context, chunk *ChunkItem) error {
	// Simulate local GPU/NPU inference latency per chunk
	time.Sleep(30 * time.Millisecond)

	wordCount := len(strings.Fields(chunk.Text))
	chunk.InputTokens = wordCount * 4 / 3 // Approximate token-to-word ratio
	chunk.OutputTokens = 20 + rand.Intn(15)

	chunk.Summary = fmt.Sprintf("Summary [model=%s]: Key insights extracted from %d words in %s",
		s.modelName, wordCount, chunk.ID)
	return nil
}

// LocalBgeEmbedder simulates a local embedding model (e.g. bge-large via llama.cpp).
type LocalBgeEmbedder struct {
	modelName  string
	dimensions int
}

func NewLocalBgeEmbedder(modelName string, dimensions int) *LocalBgeEmbedder {
	if dimensions <= 0 {
		dimensions = 768
	}
	return &LocalBgeEmbedder{
		modelName:  modelName,
		dimensions: dimensions,
	}
}

func (e *LocalBgeEmbedder) ModelName() string {
	return e.modelName
}

func (e *LocalBgeEmbedder) Dimensions() int {
	return e.dimensions
}

func (e *LocalBgeEmbedder) EmbedChunk(ctx context.Context, chunk *ChunkItem) error {
	// Simulate embedding inference latency
	time.Sleep(15 * time.Millisecond)

	wordCount := len(strings.Fields(chunk.Text))
	chunk.InputTokens = wordCount * 4 / 3

	// Generate deterministic normalized mock embedding vector
	vec := make([]float32, e.dimensions)
	for i := 0; i < e.dimensions; i++ {
		vec[i] = float32(i%10) / 10.0
	}
	chunk.Embedding = vec
	return nil
}

package instrumented_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/instrumented"
	"go.opentelemetry.io/otel"
)

func TestInstrumentedGoogleGemmaEmbeddingClient(t *testing.T) {
	ctx := context.Background()

	metrics, err := instrumented.NewMetrics(otel.Meter("test-embeddings"))
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	client, err := instrumented.NewInstrumentedGoogleGemmaEmbeddingClient(ctx, metrics)
	if err != nil {
		t.Fatalf("Failed to create instrumented client: %v", err)
	}

	t.Run("GetEmbedding single text", func(t *testing.T) {
		vec, err := client.GetEmbedding(ctx, "OpenTelemetry GenAI Semantic Conventions")
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if len(vec) != 768 {
			t.Errorf("Expected 768 dimensions, got %d", len(vec))
		}
	})

	t.Run("GetEmbeddings batch texts", func(t *testing.T) {
		texts := []string{
			"Retrieval Augmented Generation",
			"Dense vector search with HNSW",
			"Document chunking strategies",
		}
		vecs, err := client.GetEmbeddings(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if len(vecs) != 3 {
			t.Errorf("Expected 3 vector results, got %d", len(vecs))
		}
		for i, vec := range vecs {
			if len(vec) != 768 {
				t.Errorf("Item %d: expected 768 dimensions, got %d", i, len(vec))
			}
		}
	})

	t.Run("GetQueryEmbedding with task type", func(t *testing.T) {
		vec, err := client.GetQueryEmbedding(ctx, "What are the latest OTel GenAI standards?", instrumented.TaskTypeRetrievalQuery)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if len(vec) != 768 {
			t.Errorf("Expected 768 dimensions, got %d", len(vec))
		}
	})

	t.Run("GetDocumentEmbeddings batch documents", func(t *testing.T) {
		docs := []*instrumented.Document{
			{Title: "Doc 1", Content: "OpenTelemetry is an observability framework."},
			{Title: "Doc 2", Content: "Vectors are high-dimensional numerical representations."},
		}
		vecs, err := client.GetDocumentEmbeddings(ctx, docs)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if len(vecs) != 2 {
			t.Errorf("Expected 2 vector results, got %d", len(vecs))
		}
	})
}

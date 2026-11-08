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

func TestInstrumentedGoogleGemmaIVSummarizer(t *testing.T) {
	ctx := context.Background()

	summaryMetrics, err := instrumented.NewSummaryMetrics(otel.Meter("test-summarizer"))
	if err != nil {
		t.Fatalf("Failed to create summary metrics: %v", err)
	}

	summarizer, err := instrumented.NewInstrumentedGoogleGemmaIVSummarizer(ctx, summaryMetrics)
	if err != nil {
		t.Fatalf("Failed to create instrumented summarizer: %v", err)
	}

	if summarizer.Model() != "google/gemma-4-E4B-it-qat-q4_0-gguf" {
		t.Errorf("Unexpected model: %s", summarizer.Model())
	}

	t.Run("GetSummary concise text", func(t *testing.T) {
		text := "OpenTelemetry provides vendor-neutral APIs and SDKs for collecting telemetry data."
		summary, err := summarizer.GetSummary(ctx, text)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if summary == "" {
			t.Errorf("Expected non-empty summary")
		}
	})

	t.Run("GetLongSummary comprehensive text", func(t *testing.T) {
		text := "Large Language Models are deep learning systems trained on massive datasets to understand and generate natural language."
		summary, err := summarizer.GetLongSummary(ctx, text)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if summary == "" {
			t.Errorf("Expected non-empty long summary")
		}
	})

	t.Run("GetLongSummaryWithPromptVersion versioned prompt", func(t *testing.T) {
		text := "Context window constraints require efficient prompt engineering and chunk summarization."
		summary, err := summarizer.GetLongSummaryWithPromptVersion(ctx, text, 2)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if summary == "" {
			t.Errorf("Expected non-empty versioned summary")
		}
	})

	t.Run("GetMergedSummary multi-text merge", func(t *testing.T) {
		texts := []string{
			"Chapter 1: Distributed Tracing foundations.",
			"Chapter 2: Metric aggregation and histograms.",
			"Chapter 3: Log correlation via trace context.",
		}
		summary, err := summarizer.GetMergedSummary(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if summary == "" {
			t.Errorf("Expected non-empty merged summary")
		}
	})

	t.Run("GetMergedLongSummary multi-text comprehensive merge", func(t *testing.T) {
		texts := []string{
			"Section A: Gemma model family architecture.",
			"Section B: Instruction tuned quantization.",
		}
		summary, err := summarizer.GetMergedLongSummary(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if summary == "" {
			t.Errorf("Expected non-empty merged long summary")
		}
	})
}

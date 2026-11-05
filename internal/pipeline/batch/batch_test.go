package batch_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch/stages"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/modelmanager"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
	"go.opentelemetry.io/otel"
)

func TestLocalAI_BatchWorkflow(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize OpenTelemetry stdout sdk for test
	cfg := telemetry.DefaultConfig("test-localai-batch")
	cfg.ExporterType = "stdout"
	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to setup telemetry: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	// 2. Initialize ModelManager and Batch Metrics
	modelMgr, err := modelmanager.New("metal", otel.Meter("test-mgr"), otel.Tracer("test-mgr"))
	if err != nil {
		t.Fatalf("Failed to create model manager: %v", err)
	}

	batchMetrics, err := batch.NewBatchMetrics()
	if err != nil {
		t.Fatalf("Failed to create batch metrics: %v", err)
	}

	// 3. Setup Chunk Engines (wrapped with GenAI telemetry decorators)
	rawSummarizer := batch.NewLocalLlamaSummarizer("llama-3-8b-instruct.gguf")
	decoratedSummarizer := batch.WrapSummarizer(rawSummarizer, batchMetrics)

	rawEmbedder := batch.NewLocalBgeEmbedder("bge-large-en-v1.5.gguf", 768)
	decoratedEmbedder := batch.WrapEmbedder(rawEmbedder, batchMetrics)

	// 4. Assemble Pure Batch Stages (ZERO OpenTelemetry code inside stages!)
	stageList := []pipeline.Pipeline{
		stages.NewBatchChunkingPipeline(30), // 30 words per chunk
		stages.NewBatchSummarizerPipeline(modelMgr, "llama-3-8b-instruct.gguf", 4_500_000_000, decoratedSummarizer),
		stages.NewBatchEmbeddingsPipeline(modelMgr, "bge-large-en-v1.5.gguf", 1_300_000_000, decoratedEmbedder),
	}

	orchestrator := batch.NewBatchOrchestrator("local-ai-staged-pipeline", stageList, batchMetrics)

	// 5. Create test input documents
	testDocs := []*batch.DocumentItem{
		{
			ID:    "doc-ai-01",
			Title: "OpenTelemetry GenAI Overview",
			Content: "OpenTelemetry is introducing standardized GenAI semantic conventions. " +
				"These conventions cover prompt tokens, completion tokens, model names, and system engines. " +
				"In local AI environments using llama.cpp or whisper.cpp, memory constraints require bulk execution.",
		},
		{
			ID:    "doc-ai-02",
			Title: "Batch Ingestion Mechanics",
			Content: "Swapping models between RAM and VRAM causes significant latency penalties. " +
				"By executing the chunking stage completely across all documents, followed by summarization, " +
				"and finally embeddings, we swap models exactly once instead of on every item.",
		},
	}

	params := batch.NewBatchFilterParams("req-batch-001", "tenant-alpha", testDocs)

	// 6. Execute Batch Workflow
	if err := orchestrator.Execute(ctx, params); err != nil {
		t.Fatalf("Expected batch orchestrator to succeed, got: %v", err)
	}

	// 7. Verify Document and Chunk outputs
	resDocs, err := batch.GetDocuments(params)
	if err != nil {
		t.Fatalf("Failed to get documents: %v", err)
	}

	totalChunks := batch.TotalChunks(resDocs)
	if totalChunks == 0 {
		t.Fatal("Expected chunks to be generated, got 0")
	}

	for _, doc := range resDocs {
		if len(doc.Chunks) == 0 {
			t.Errorf("Document %s has 0 chunks", doc.ID)
		}
		for _, chunk := range doc.Chunks {
			if chunk.Summary == "" {
				t.Errorf("Chunk %s is missing summary", chunk.ID)
			}
			if len(chunk.Embedding) != 768 {
				t.Errorf("Chunk %s expected 768-dim embedding, got %d", chunk.ID, len(chunk.Embedding))
			}
			if chunk.InputTokens == 0 {
				t.Errorf("Chunk %s missing input token count", chunk.ID)
			}
		}
	}

	// 8. Verify Memory Constraint Invariant: All models should be evicted, VRAM back to 0
	if modelMgr.ActiveModel() != "" {
		t.Errorf("Expected active model to be evicted after pipeline, but got %q", modelMgr.ActiveModel())
	}
	if modelMgr.AllocatedVRAM() != 0 {
		t.Errorf("Expected 0 VRAM allocated after pipeline, but got %d bytes", modelMgr.AllocatedVRAM())
	}
}

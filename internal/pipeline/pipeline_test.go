package pipeline_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/stages"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
)

func TestPipeline_CleanArchitecture(t *testing.T) {
	ctx := context.Background()

	// Initialize stdout telemetry for test
	cfg := telemetry.DefaultConfig("test-pipeline")
	cfg.ExporterType = "stdout"
	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to setup telemetry: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	// 1. Initialize Pipeline Metrics
	pipelineMetrics, err := pipeline.NewMetrics()
	if err != nil {
		t.Fatalf("Failed to initialize pipeline metrics: %v", err)
	}

	// 2. Instantiate Pure Pipeline Stages (100% Free of any OpenTelemetry imports!)
	stageList := []pipeline.Pipeline{
		stages.NewTranscriptPipeline(),
		stages.NewChunkingPipeline(150),
		stages.NewEmbeddingsPipeline("text-embedding-3-small", 1536),
		stages.NewSummarizerPipeline("gemini-1.5-pro"),
	}

	// 3. Assemble Orchestrator (automatically wraps each stage with TelemetryDecorator)
	orchestrator := pipeline.NewOrchestrator("rag-audio-ingestion-workflow", stageList, pipelineMetrics)

	t.Run("Execute complete successful pipeline workflow", func(t *testing.T) {
		params := request.NewFilterParams("req-pipe-101", "doc-audio-01", "tenant-acme")
		params.AudioURI = "s3://media-bucket/audio/tutorial-part-1.mp3"

		if err := orchestrator.Execute(ctx, params); err != nil {
			t.Fatalf("Expected pipeline to succeed, got: %v", err)
		}

		if params.TranscriptText == "" {
			t.Error("Expected transcript text to be populated")
		}
		if len(params.Chunks) == 0 {
			t.Error("Expected text chunks to be populated")
		}
		if len(params.Embeddings) == 0 {
			t.Error("Expected vector embeddings to be populated")
		}
		if params.Summary == "" {
			t.Error("Expected summary to be populated")
		}
	})

	t.Run("Pipeline halts gracefully and records error on invalid input", func(t *testing.T) {
		// Missing AudioURI causes transcript stage to fail
		params := request.NewFilterParams("req-pipe-102", "doc-audio-02", "tenant-acme")

		err := orchestrator.Execute(ctx, params)
		if err == nil {
			t.Fatal("Expected error due to missing audio URI, got nil")
		}
	})
}

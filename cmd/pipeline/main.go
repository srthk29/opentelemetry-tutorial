package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/stages"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
)

func main() {
	exporterType := flag.String("exporter", "stdout", "Telemetry exporter: 'stdout' (Console) or 'otlp' (Collector)")
	collectorEndpoint := flag.String("collector-addr", "localhost:4317", "OTLP gRPC collector address")
	flag.Parse()

	ctx := context.Background()

	// 1. Initialize OpenTelemetry SDK
	cfg := telemetry.DefaultConfig("ai-pipeline-service")
	cfg.ExporterType = *exporterType
	cfg.OTLPEndpoint = *collectorEndpoint

	shutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	// 2. Initialize Standard Pipeline Metrics
	pipelineMetrics, err := pipeline.NewMetrics()
	if err != nil {
		log.Fatalf("Failed to create pipeline metrics: %v", err)
	}

	// 3. Instantiate Pure Pipeline Stages (ZERO OpenTelemetry code inside stages!)
	stageList := []pipeline.Pipeline{
		stages.NewTranscriptPipeline(),
		stages.NewChunkingPipeline(150),
		stages.NewEmbeddingsPipeline("text-embedding-3-small", 1536),
		stages.NewSummarizerPipeline("gemini-1.5-pro"),
	}

	// 4. Assemble Orchestrator (Automatically wraps each stage with TelemetryDecorator)
	orchestrator := pipeline.NewOrchestrator("rag-audio-ingestion-workflow", stageList, pipelineMetrics)

	fmt.Println("=================================================================")
	fmt.Println("🚀 Executing Multi-Stage AI Pipeline with OpenTelemetry Decorator")
	fmt.Println("   Workflow: Transcript -> Chunking -> Embeddings -> Summarizer")
	fmt.Println("=================================================================")

	// Run Scenario 1: Successful Audio Document Ingestion
	params := request.NewFilterParams("req-ai-001", "doc-podcasts-episode-42", "tenant-enterprise-1")
	params.AudioURI = "s3://media-vault/audio/podcasts/episode-42.mp3"

	if err := orchestrator.Execute(ctx, params); err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	fmt.Println("\n✅ Pipeline Workflow Completed Successfully!")
	fmt.Printf("   Document ID  : %s\n", params.DocumentID)
	fmt.Printf("   Chunks       : %d chunks\n", len(params.Chunks))
	fmt.Printf("   Vectors      : %d embedding vectors\n", len(params.Embeddings))
	fmt.Printf("   Summary      : %s\n", params.Summary)

	// Run Scenario 2: Error Scenario (Missing Audio URI)
	fmt.Println("\n--- Testing Error Handling in Pipeline ---")
	errParams := request.NewFilterParams("req-ai-002", "doc-faulty-01", "tenant-enterprise-1")
	if err := orchestrator.Execute(ctx, errParams); err != nil {
		fmt.Printf("✓ Correctly caught expected error and recorded in span: %v\n", err)
	}

	fmt.Println("\nFlushing telemetry data to exporter...")
}

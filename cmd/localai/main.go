package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/batch/stages"
	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/modelmanager"
	"github.com/sarthak/opentelemetry-tutorial/internal/telemetry"
	"go.opentelemetry.io/otel"
)

func main() {
	exporterType := flag.String("exporter", "stdout", "Telemetry exporter: 'stdout' (Console) or 'otlp' (Collector)")
	collectorEndpoint := flag.String("collector-addr", "localhost:4317", "OTLP gRPC collector address")
	deviceType := flag.String("device", "metal", "Compute backend device: 'metal', 'cuda', or 'cpu'")
	flag.Parse()

	ctx := context.Background()

	// 1. Initialize OpenTelemetry SDK
	cfg := telemetry.DefaultConfig("local-ai-batch-service")
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

	fmt.Println("================================================================================")
	fmt.Printf("🚀 Local AI Memory-Constrained Staged Batch Pipeline [Device: %s]\n", *deviceType)
	fmt.Println("   Invariant: Only 1 model in VRAM at any time; stages execute in bulk.")
	fmt.Println("================================================================================")

	// 2. Initialize Model Manager and Batch Metrics
	modelMgr, err := modelmanager.New(*deviceType, otel.Meter("local-ai"), otel.Tracer("local-ai"))
	if err != nil {
		log.Fatalf("Failed to initialize ModelManager: %v", err)
	}

	batchMetrics, err := batch.NewBatchMetrics()
	if err != nil {
		log.Fatalf("Failed to initialize BatchMetrics: %v", err)
	}

	// 3. Setup Chunk Engines wrapped with GenAI Telemetry Decorators
	llamaSummarizer := batch.WrapSummarizer(
		batch.NewLocalLlamaSummarizer("llama-3-8b-instruct.gguf"),
		batchMetrics,
	)
	bgeEmbedder := batch.WrapEmbedder(
		batch.NewLocalBgeEmbedder("bge-large-en-v1.5.gguf", 768),
		batchMetrics,
	)

	// 4. Assemble Pure Batch Stages (ZERO OpenTelemetry code inside the stages!)
	stageList := []pipeline.Pipeline{
		stages.NewBatchChunkingPipeline(35), // 35 words per chunk
		stages.NewBatchSummarizerPipeline(modelMgr, "llama-3-8b-instruct.gguf", 4_500_000_000, llamaSummarizer),
		stages.NewBatchEmbeddingsPipeline(modelMgr, "bge-large-en-v1.5.gguf", 1_300_000_000, bgeEmbedder),
	}

	orchestrator := batch.NewBatchOrchestrator("local-ai-staged-workflow", stageList, batchMetrics)

	// 5. Seed input batch items
	sampleDocs := []*batch.DocumentItem{
		{
			ID:    "doc-001",
			Title: "OpenTelemetry GenAI Semantic Conventions",
			Content: "OpenTelemetry has introduced standard GenAI semantic conventions to monitor Large Language Models. " +
				"Key conventions include gen_ai.system, gen_ai.request.model, gen_ai.operation.name, and token usage metrics. " +
				"These metrics track input and output tokens accurately across all inference calls.",
		},
		{
			ID:    "doc-002",
			Title: "Local AI Execution on Apple Silicon with Metal",
			Content: "Apple unified memory architecture allows CPUs and GPUs to share high bandwidth memory. " +
				"However, when running quantized 8B LLMs alongside Whisper and Embedding models, memory limits " +
				"require unloading inactive models to prevent swapping to disk. Bulk stage processing minimizes swaps.",
		},
		{
			ID:    "doc-003",
			Title: "Clean Architecture for AI Telemetry",
			Content: "By decoupling AI inference logic from observability code through the Decorator Pattern, " +
				"developers can unit test their chunking, summarization, and vector indexing logic in pure isolation. " +
				"Telemetry decorators automatically attach trace spans, GenAI attributes, and Prometheus metrics.",
		},
	}

	params := batch.NewBatchFilterParams("req-batch-localai-01", "tenant-engineering", sampleDocs)

	fmt.Printf("\n📥 Input: %d documents submitted for staged bulk processing\n", len(sampleDocs))

	// 6. Execute Workflow
	startTime := time.Now()
	if err := orchestrator.Execute(ctx, params); err != nil {
		log.Fatalf("❌ Batch pipeline execution failed: %v", err)
	}
	totalDuration := time.Since(startTime)

	// 7. Print Results Summary
	resDocs, _ := batch.GetDocuments(params)
	totalChunks := batch.TotalChunks(resDocs)

	fmt.Println("\n================================================================================")
	fmt.Printf("✅ Batch Workflow Completed in %v\n", totalDuration)
	fmt.Printf("📊 Processed: %d documents into %d chunks\n", len(resDocs), totalChunks)
	fmt.Printf("🧠 VRAM State: Model=%q, Allocated VRAM=%d bytes (Invariant Preserved: 0 B)\n",
		modelMgr.ActiveModel(), modelMgr.AllocatedVRAM())
	fmt.Println("================================================================================")

	for _, doc := range resDocs {
		fmt.Printf("\n📄 Document [%s]: %q\n", doc.ID, doc.Title)
		for _, chunk := range doc.Chunks {
			fmt.Printf("   ├─ Chunk #%d [%s]:\n", chunk.Index, chunk.ID)
			fmt.Printf("   │   Summary: %s\n", chunk.Summary)
			fmt.Printf("   │   Tokens: %d in, %d out | Embedding: [%.2f, %.2f, ... (%d dims)]\n",
				chunk.InputTokens, chunk.OutputTokens,
				chunk.Embedding[0], chunk.Embedding[1], len(chunk.Embedding))
		}
	}
	fmt.Println("\n🎉 Done! If running with -exporter=otlp, view the traces in Jaeger (http://localhost:16686).")
}

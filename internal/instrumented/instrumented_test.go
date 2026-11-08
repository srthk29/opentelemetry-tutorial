package instrumented_test

import (
	"context"
	"fmt"
	"strings"
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

func TestInstrumentedOpenAISummarizer(t *testing.T) {
	ctx := context.Background()

	summaryMetrics, err := instrumented.NewSummaryMetrics(otel.Meter("test-openai-summarizer"))
	if err != nil {
		t.Fatalf("Failed to create summary metrics: %v", err)
	}

	summarizer, err := instrumented.NewInstrumentedOpenAISummarizer(ctx, "google/gemma-4-E4B-it-qat-q4_0-gguf", summaryMetrics)
	if err != nil {
		t.Fatalf("Failed to create instrumented OpenAI summarizer: %v", err)
	}

	if summarizer.Model() != "google/gemma-4-E4B-it-qat-q4_0-gguf" {
		t.Errorf("Unexpected model: %s", summarizer.Model())
	}

	t.Run("GetSummary", func(t *testing.T) {
		resp, err := summarizer.GetSummary(ctx, "OpenTelemetry traces and metrics integration with OpenAI API.")
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			t.Fatalf("Expected valid choices in response")
		}
		content, err := resp.Choices[0].Message.Content.AsChatCompletionResponseMessageContent0()
		if err != nil {
			t.Fatalf("Failed to extract content: %v", err)
		}
		if content == "" {
			t.Errorf("Expected non-empty summary content")
		}
	})

	t.Run("GetLongSummary", func(t *testing.T) {
		resp, err := summarizer.GetLongSummary(ctx, "In-depth article on Large Language Model inference optimization.")
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			t.Fatalf("Expected valid choices in response")
		}
	})

	t.Run("GetLongSummaryWithPromptVersion", func(t *testing.T) {
		resp, err := summarizer.GetLongSummaryWithPromptVersion(ctx, "Technical prompt versioning test.", 1)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			t.Fatalf("Expected valid choices in response")
		}
	})

	t.Run("GetMergedSummary", func(t *testing.T) {
		texts := []string{
			"Summary 1: Low-latency metric aggregation.",
			"Summary 2: Span attributes and event logging.",
		}
		resp, err := summarizer.GetMergedSummary(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			t.Fatalf("Expected valid choices in response")
		}
	})

	t.Run("GetMergedLongSummary", func(t *testing.T) {
		texts := []string{
			"Doc 1: OpenTelemetry Collector pipelines.",
			"Doc 2: Prometheus scraping exporters.",
		}
		resp, err := summarizer.GetMergedLongSummary(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			t.Fatalf("Expected valid choices in response")
		}
	})
}

func TestInstrumentedOpenAIEmbeddingClient(t *testing.T) {
	ctx := context.Background()

	embeddingMetrics, err := instrumented.NewEmbeddingMetrics(otel.Meter("test-openai-embeddings"))
	if err != nil {
		t.Fatalf("Failed to create embedding metrics: %v", err)
	}

	client, err := instrumented.NewInstrumentedOpenAIEmbeddingClient(
		ctx,
		"google/embeddinggemma-300m",
		768,
		false,
		embeddingMetrics,
	)
	if err != nil {
		t.Fatalf("Failed to create instrumented OpenAI embedding client: %v", err)
	}

	if client.GetModel() != "google/embeddinggemma-300m" {
		t.Errorf("Unexpected model: %s", client.GetModel())
	}
	if client.GetDimensionality() != 768 {
		t.Errorf("Unexpected dimensionality: %d", client.GetDimensionality())
	}
	if client.IsNormalized() != false {
		t.Errorf("Unexpected normalized flag: %v", client.IsNormalized())
	}

	t.Run("GetEmbedding single text", func(t *testing.T) {
		resp, err := client.GetEmbedding(ctx, "OpenTelemetry GenAI vector embeddings")
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Data) != 1 {
			t.Fatalf("Expected 1 vector data result")
		}
		if len(resp.Data[0].Embedding) != 768 {
			t.Errorf("Expected 768 dimensions, got %d", len(resp.Data[0].Embedding))
		}
	})

	t.Run("GetEmbeddings batch texts", func(t *testing.T) {
		texts := []string{
			"Semantic search vectorization",
			"Retrieval-Augmented Generation chunking",
			"HNSW graph index distance metrics",
		}
		resp, err := client.GetEmbeddings(ctx, texts)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Data) != 3 {
			t.Fatalf("Expected 3 vector data results")
		}
		for i, d := range resp.Data {
			if len(d.Embedding) != 768 {
				t.Errorf("Index %d: expected 768 dimensions, got %d", i, len(d.Embedding))
			}
		}
	})

	t.Run("GetQueryEmbedding with task type", func(t *testing.T) {
		resp, err := client.GetQueryEmbedding(ctx, "What is OTel GenAI?", instrumented.TaskTypeRetrievalQuery)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Data) != 1 {
			t.Fatalf("Expected 1 vector data result")
		}
	})

	t.Run("GetDocumentEmbedding single document", func(t *testing.T) {
		doc := &instrumented.Document{
			Title:   "Tracing Spec",
			Content: "Client spans must model outbound inference calls.",
		}
		resp, err := client.GetDocumentEmbedding(ctx, doc)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Data) != 1 {
			t.Fatalf("Expected 1 vector data result")
		}
	})

	t.Run("GetDocumentEmbeddings batch documents", func(t *testing.T) {
		docs := []*instrumented.Document{
			{Title: "Doc A", Content: "Metrics instruments for tokens and duration."},
			{Title: "Doc B", Content: "Context propagation across distributed microservices."},
		}
		resp, err := client.GetDocumentEmbeddings(ctx, docs)
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if resp == nil || len(resp.Data) != 2 {
			t.Fatalf("Expected 2 vector data results")
		}
	})
}

func TestInstrumentedLaunchController(t *testing.T) {
	ctx := context.Background()

	metrics, err := instrumented.NewLaunchctlMetrics(otel.Meter("test-launchctl"))
	if err != nil {
		t.Fatalf("Failed to create launchctl metrics: %v", err)
	}

	// Create mock command runner to test without affecting system launchd
	mockCommands := make([]string, 0)
	mockRunner := func(cmdCtx context.Context, name string, args ...string) ([]byte, error) {
		cmdStr := name + " " + strings.Join(args, " ")
		mockCommands = append(mockCommands, cmdStr)
		if len(args) > 0 && args[0] == "print" {
			return []byte("state = running\npid = 12345"), nil
		}
		return []byte("ok"), nil
	}

	baseController := instrumented.NewExecLaunchControllerWithRunner(mockRunner, "gui/501")
	lc, err := instrumented.NewInstrumentedLaunchController(baseController, metrics)
	if err != nil {
		t.Fatalf("Failed to create instrumented launch controller: %v", err)
	}

	t.Run("Start and Stop lifecycle (user example pattern)", func(t *testing.T) {
		out, startErr := lc.Start(ctx, "whisper")
		if startErr != nil {
			t.Fatalf("Expected success, got: %v", startErr)
		}
		if out != "ok" {
			t.Errorf("Expected 'ok', got %q", out)
		}

		stopOut, stopErr := lc.Stop(ctx, "whisper")
		if stopErr != nil {
			t.Fatalf("Expected success, got: %v", stopErr)
		}
		if stopOut != "ok" {
			t.Errorf("Expected 'ok', got %q", stopOut)
		}
	})

	t.Run("Kickstart service", func(t *testing.T) {
		out, kickErr := lc.Kickstart(ctx, "llama-server")
		if kickErr != nil {
			t.Fatalf("Expected success, got: %v", kickErr)
		}
		if out != "ok" {
			t.Errorf("Expected 'ok', got %q", out)
		}
	})

	t.Run("Print service status", func(t *testing.T) {
		out, printErr := lc.Print(ctx, "whisper")
		if printErr != nil {
			t.Fatalf("Expected success, got: %v", printErr)
		}
		if !strings.Contains(out, "running") {
			t.Errorf("Expected running state in output, got %q", out)
		}
	})

	t.Run("Error propagation on command failure", func(t *testing.T) {
		failingRunner := func(cmdCtx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("service not found"), fmt.Errorf("exit status 1")
		}
		errController := instrumented.NewExecLaunchControllerWithRunner(failingRunner, "")
		errLc, _ := instrumented.NewInstrumentedLaunchController(errController, metrics)

		_, err := errLc.Start(ctx, "nonexistent")
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})
}

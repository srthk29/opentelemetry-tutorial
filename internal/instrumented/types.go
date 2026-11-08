package instrumented

import (
	"context"
	"fmt"

	"github.com/sarthak/opentelemetry-tutorial/internal/instrumented/codegen"
)

// TaskType represents the intended purpose of the embedding (used by models like Gemma/E5/Instructor).
type TaskType string

const (
	TaskTypeRetrievalQuery    TaskType = "retrieval_query"
	TaskTypeRetrievalDocument TaskType = "retrieval_document"
	TaskTypeClassification    TaskType = "classification"
	TaskTypeClustering        TaskType = "clustering"
)

// Document represents an input text document with optional title.
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// OpenAIEmbeddingResponse represents the standard OpenAI-compatible API response format.
type OpenAIEmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// EmbeddingData represents an individual vector item in the response.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// Usage holds token consumption statistics returned by the GenAI provider.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens"`
}

// Summarizer defines the public client interface used by application code for summarization.
type Summarizer interface {
	Model() string
	GetSummary(ctx context.Context, text string) (string, error)
	GetLongSummary(ctx context.Context, text string) (string, error)
	GetLongSummaryWithPromptVersion(ctx context.Context, text string, promptVersion int) (string, error)
	GetMergedSummary(ctx context.Context, texts []string) (string, error)
	GetMergedLongSummary(ctx context.Context, texts []string) (string, error)
}

// OpenAIChatCompletionResponse aliases codegen.ChatCompletionResponse for compatibility.
type OpenAIChatCompletionResponse = codegen.ChatCompletionResponse
type ChatChoice = codegen.ChatCompletionChoice
type ChatMessage = codegen.ChatCompletionResponseMessage
type ChatCompletionResponseContent = codegen.ChatCompletionResponseMessage_Content

// EmbedderWithEmbeddingResponse defines the low-level provider interface returning the raw response with token usage.
type EmbedderWithEmbeddingResponse interface {
	GetEmbedding(ctx context.Context, text string) (*OpenAIEmbeddingResponse, error)
	GetEmbeddings(ctx context.Context, texts []string) (*OpenAIEmbeddingResponse, error)
	GetModel() string
	GetDimensionality() int
	IsNormalized() bool
}

// Embedder defines the public client interface used by application code.
type Embedder interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	GetModel() string
	GetDimensionality() int
	GetQueryEmbedding(ctx context.Context, text string, taskType TaskType) ([]float32, error)
	GetDocumentEmbedding(ctx context.Context, content *Document) ([]float32, error)
	GetDocumentEmbeddings(ctx context.Context, contents []*Document) ([][]float32, error)
	IsNormalized() bool
}

// PrepareQuery prepares text for query embeddings.
func PrepareQuery(taskType TaskType, text string) string {
	return fmt.Sprintf("task: %s | query: %s", taskType, text)
}

// PrepareDocument prepares title and content for document embeddings.
func PrepareDocument(title, content string) string {
	if title == "" {
		return content
	}
	return fmt.Sprintf("title: %s | text: %s", title, content)
}

// mockOpenAIEmbeddingClient provides a default in-memory implementation for testing & fallback.
type mockOpenAIEmbeddingClient struct {
	model      string
	dimensions int
	normalized bool
}

// NewOpenAIEmbeddingClient creates an EmbedderWithEmbeddingResponse client.
func NewOpenAIEmbeddingClient(ctx context.Context, model string, dimensions int, normalized bool) (EmbedderWithEmbeddingResponse, error) {
	if dimensions <= 0 {
		dimensions = 768
	}
	return &mockOpenAIEmbeddingClient{
		model:      model,
		dimensions: dimensions,
		normalized: normalized,
	}, nil
}

func (m *mockOpenAIEmbeddingClient) GetModel() string {
	return m.model
}

func (m *mockOpenAIEmbeddingClient) GetDimensionality() int {
	return m.dimensions
}

func (m *mockOpenAIEmbeddingClient) IsNormalized() bool {
	return m.normalized
}

func (m *mockOpenAIEmbeddingClient) GetEmbedding(ctx context.Context, text string) (*OpenAIEmbeddingResponse, error) {
	return m.GetEmbeddings(ctx, []string{text})
}

func (m *mockOpenAIEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) (*OpenAIEmbeddingResponse, error) {
	data := make([]EmbeddingData, len(texts))
	totalTokens := 0

	for i, txt := range texts {
		vec := make([]float32, m.dimensions)
		for j := 0; j < m.dimensions; j++ {
			vec[j] = float32(j%10) / 10.0
		}
		data[i] = EmbeddingData{
			Object:    "embedding",
			Embedding: vec,
			Index:     i,
		}
		// Approximate 4 chars per token
		tokens := len(txt) / 4
		if tokens == 0 {
			tokens = 1
		}
		totalTokens += tokens
	}

	return &OpenAIEmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  m.model,
		Usage: Usage{
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
		},
	}, nil
}

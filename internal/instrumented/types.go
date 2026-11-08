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

// OpenAIEmbeddingResponse aliases codegen.EmbeddingResponse for compatibility.
type OpenAIEmbeddingResponse = codegen.EmbeddingResponse
type EmbeddingData = codegen.EmbeddingData
type EmbeddingUsage = codegen.EmbeddingUsage

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
	GetEmbedding(ctx context.Context, text string) (*codegen.EmbeddingResponse, error)
	GetEmbeddings(ctx context.Context, texts []string) (*codegen.EmbeddingResponse, error)
	GetModel() string
	GetDimensionality() int
	GetQueryEmbedding(ctx context.Context, text string, taskType TaskType) (*codegen.EmbeddingResponse, error)
	GetDocumentEmbedding(ctx context.Context, content *Document) (*codegen.EmbeddingResponse, error)
	GetDocumentEmbeddings(ctx context.Context, contents []*Document) (*codegen.EmbeddingResponse, error)
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

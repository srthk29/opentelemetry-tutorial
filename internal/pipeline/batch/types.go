package batch

import (
	"fmt"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

const BatchDocumentsMetaKey = "batch.documents"

// ChunkItem represents an individual segmented unit of a document.
type ChunkItem struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Index      int       `json:"index"`
	Text       string    `json:"text"`
	Summary    string    `json:"summary,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`

	// Token tracking for GenAI telemetry
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// DocumentItem represents a raw input document before and after chunking.
type DocumentItem struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Content string      `json:"content"`
	Chunks  []ChunkItem `json:"chunks"`
}

// NewBatchFilterParams creates a standard FilterParams carrying a collection of documents.
func NewBatchFilterParams(requestID, tenantID string, docs []*DocumentItem) *request.FilterParams {
	params := request.NewFilterParams(requestID, "batch-aggregate", tenantID)
	SetDocuments(params, docs)
	return params
}

// SetDocuments attaches the document collection into the FilterParams metadata.
func SetDocuments(params *request.FilterParams, docs []*DocumentItem) {
	if params == nil {
		return
	}
	params.SetMeta(BatchDocumentsMetaKey, docs)
}

// GetDocuments retrieves the document collection from FilterParams.
func GetDocuments(params *request.FilterParams) ([]*DocumentItem, error) {
	if params == nil || params.Metadata == nil {
		return nil, fmt.Errorf("params or metadata is nil")
	}
	val, exists := params.Metadata[BatchDocumentsMetaKey]
	if !exists {
		return nil, fmt.Errorf("missing %q in FilterParams metadata", BatchDocumentsMetaKey)
	}

	docs, ok := val.([]*DocumentItem)
	if !ok {
		return nil, fmt.Errorf("invalid type for %q, expected []*DocumentItem", BatchDocumentsMetaKey)
	}
	return docs, nil
}

// TotalChunks returns the total number of chunks across all documents.
func TotalChunks(docs []*DocumentItem) int {
	total := 0
	for _, doc := range docs {
		total += len(doc.Chunks)
	}
	return total
}

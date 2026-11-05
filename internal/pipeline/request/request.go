package request

// PipelineStage defines the logical stage identifier in a multi-stage workflow.
type PipelineStage string

const (
	StageTranscript PipelineStage = "transcript"
	StageChunking   PipelineStage = "chunking"
	StageEmbeddings PipelineStage = "embeddings"
	StageSummarizer PipelineStage = "summarizer"
)

// FilterParams represents the state and data carried through pipeline execution.
type FilterParams struct {
	RequestID  string `json:"request_id"`
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`

	// Pipeline Data Flow
	AudioURI       string      `json:"audio_uri,omitempty"`
	TranscriptText string      `json:"transcript_text,omitempty"`
	Chunks         []string    `json:"chunks,omitempty"`
	Embeddings     [][]float32 `json:"embeddings,omitempty"`
	Summary        string      `json:"summary,omitempty"`

	// Metadata holds stage-specific outputs (tokens, models, chunk sizes)
	// that the Telemetry Decorator translates into OTel GenAI semantic attributes.
	Metadata map[string]any `json:"metadata"`
}

// NewFilterParams initializes a new parameter context with an instantiated metadata map.
func NewFilterParams(reqID, docID, tenantID string) *FilterParams {
	return &FilterParams{
		RequestID:  reqID,
		DocumentID: docID,
		TenantID:   tenantID,
		Metadata:   make(map[string]any),
	}
}

// SetMeta is a helper to record metadata keys.
func (p *FilterParams) SetMeta(key string, val any) {
	if p.Metadata == nil {
		p.Metadata = make(map[string]any)
	}
	p.Metadata[key] = val
}

// GetString returns string metadata or empty.
func (p *FilterParams) GetString(key string) string {
	if v, ok := p.Metadata[key].(string); ok {
		return v
	}
	return ""
}

// GetInt returns int metadata or 0.
func (p *FilterParams) GetInt(key string) int {
	if v, ok := p.Metadata[key].(int); ok {
		return v
	}
	return 0
}

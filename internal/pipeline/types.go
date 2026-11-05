package pipeline

import (
	"context"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
)

// Pipeline represents a modular stage in a multi-stage data processing workflow.
// Every stage (Transcript, Chunking, Embeddings, Summarizer) implements this interface.
type Pipeline interface {
	Name() string
	Stage() request.PipelineStage
	Run(context.Context, *request.FilterParams) error
}

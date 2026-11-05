package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/request"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Orchestrator executes a sequence of pipeline stages in order under an
// overarching OpenTelemetry parent span.
type Orchestrator struct {
	name    string
	stages  []Pipeline
	tracer  trace.Tracer
	metrics *Metrics
}

// NewOrchestrator builds an orchestrator with a chain of pipeline stages.
// Every stage passed here is automatically wrapped with the TelemetryDecorator!
func NewOrchestrator(name string, stages []Pipeline, metrics *Metrics) *Orchestrator {
	decorated := make([]Pipeline, len(stages))
	for i, s := range stages {
		decorated[i] = Wrap(s, metrics)
	}

	return &Orchestrator{
		name:    name,
		stages:  decorated,
		tracer:  otel.Tracer("pipeline-orchestrator"),
		metrics: metrics,
	}
}

// Execute runs the complete pipeline workflow sequentially from start to finish.
func (o *Orchestrator) Execute(ctx context.Context, params *request.FilterParams) error {
	startTime := time.Now()

	// 1. In-flight active workflows tracking
	if o.metrics != nil {
		o.metrics.WorkflowActive.Add(ctx, 1, metric.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
		))
		defer o.metrics.WorkflowActive.Add(ctx, -1, metric.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
		))
	}

	// 2. Start Workflow Root Parent Span
	ctx, span := o.tracer.Start(ctx, fmt.Sprintf("pipeline.workflow: %s", o.name),
		trace.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
			attribute.Int("pipeline.workflow.stage_count", len(o.stages)),
			attribute.String("pipeline.request_id", params.RequestID),
			attribute.String("pipeline.document_id", params.DocumentID),
			attribute.String("pipeline.tenant_id", params.TenantID),
		),
	)
	defer span.End()

	// 3. Execute Stages Sequentially
	for _, stage := range o.stages {
		stageName := string(stage.Stage())

		// Add milestone span event
		span.AddEvent(fmt.Sprintf("starting_stage: %s", stageName), trace.WithAttributes(
			attribute.String("pipeline.stage", stageName),
		))

		if err := stage.Run(ctx, params); err != nil {
			durationMs := float64(time.Since(startTime).Milliseconds())
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("pipeline failed at stage %s: %v", stageName, err))

			if o.metrics != nil {
				failAttrs := metric.WithAttributes(
					attribute.String("pipeline.workflow.name", o.name),
					attribute.String("failed_at_stage", stageName),
					attribute.String("status", "failed"),
				)
				o.metrics.WorkflowRuns.Add(ctx, 1, failAttrs)
				o.metrics.WorkflowDuration.Record(ctx, durationMs, failAttrs)
			}
			return fmt.Errorf("stage %s failed: %w", stageName, err)
		}
	}

	// 4. Workflow Success
	durationMs := float64(time.Since(startTime).Milliseconds())
	span.SetStatus(codes.Ok, "pipeline workflow completed successfully")

	if o.metrics != nil {
		successAttrs := metric.WithAttributes(
			attribute.String("pipeline.workflow.name", o.name),
			attribute.String("status", "success"),
		)
		o.metrics.WorkflowRuns.Add(ctx, 1, successAttrs)
		o.metrics.WorkflowDuration.Record(ctx, durationMs, successAttrs)
	}

	return nil
}

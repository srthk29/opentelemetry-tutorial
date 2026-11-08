package instrumented

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedLaunchController wraps a LaunchController with OpenTelemetry
// traces and Prometheus metrics following OTel Process and Service conventions.
type InstrumentedLaunchController struct {
	controller LaunchController
	tracer     trace.Tracer
	metrics    *LaunchctlMetrics
	executable string
}

// Compile-time assertion that InstrumentedLaunchController implements LaunchController.
var _ LaunchController = (*InstrumentedLaunchController)(nil)

// InstrumentedLaunchControllerWrapper aliases InstrumentedLaunchController for naming flexibility.
type InstrumentedLaunchControllerWrapper = InstrumentedLaunchController

// NewInstrumentedLaunchController initializes an instrumented LaunchController.
func NewInstrumentedLaunchController(controller LaunchController, metrics *LaunchctlMetrics) (*InstrumentedLaunchController, error) {
	if controller == nil {
		controller = NewExecLaunchController("")
	}

	if metrics == nil {
		var metricErr error
		metrics, metricErr = NewLaunchctlMetrics(nil)
		if metricErr != nil {
			return nil, fmt.Errorf("failed to initialize launchctl metrics: %w", metricErr)
		}
	}

	return &InstrumentedLaunchController{
		controller: controller,
		tracer:     otel.Tracer("launchctl-controller"),
		metrics:    metrics,
		executable: "launchctl",
	}, nil
}

// NewInstrumentedLaunchControllerWithTracer initializes an instrumented LaunchController with a custom tracer.
func NewInstrumentedLaunchControllerWithTracer(
	controller LaunchController,
	tracer trace.Tracer,
	metrics *LaunchctlMetrics,
) *InstrumentedLaunchController {
	if controller == nil {
		controller = NewExecLaunchController("")
	}
	if tracer == nil {
		tracer = otel.Tracer("launchctl-controller")
	}
	return &InstrumentedLaunchController{
		controller: controller,
		tracer:     tracer,
		metrics:    metrics,
		executable: "launchctl",
	}
}

// Controller returns the underlying LaunchController.
func (ilc *InstrumentedLaunchController) Controller() LaunchController {
	return ilc.controller
}

// execute encapsulates the full OpenTelemetry span lifecycle and metrics tracking for a launchctl action.
func (ilc *InstrumentedLaunchController) execute(
	ctx context.Context,
	action string,
	serviceName string,
	fn func(context.Context) (string, error),
) (string, error) {
	startTime := time.Now()

	// 1. In-flight active operations tracking
	if ilc.metrics != nil {
		activeAttrs := metric.WithAttributes(
			attribute.String("launchctl.action", action),
			attribute.String("service.name", serviceName),
		)
		ilc.metrics.ActiveOperations.Add(ctx, 1, activeAttrs)
		defer ilc.metrics.ActiveOperations.Add(ctx, -1, activeAttrs)
	}

	// 2. Start OpenTelemetry Process/Command Execution Span
	// Standard span naming convention: "launchctl {action}"
	args := []string{ilc.executable, action, serviceName}
	if action == "kickstart" {
		args = []string{ilc.executable, action, "-k", serviceName}
	}

	spanAttrs := []attribute.KeyValue{
		attribute.String("process.executable.name", ilc.executable),
		attribute.String("process.command", ilc.executable),
		attribute.StringSlice("process.command_args", args),
		attribute.String("launchctl.action", action),
		attribute.String("service.name", serviceName),
	}

	ctx, span := ilc.tracer.Start(ctx, fmt.Sprintf("%s %s", ilc.executable, action),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	// 3. Execute underlying launchctl action
	output, err := fn(ctx)
	durationMs := float64(time.Since(startTime).Milliseconds())

	// 4. Handle Failure
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}

		span.SetAttributes(
			attribute.Int("process.exit_code", exitCode),
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)

		if ilc.metrics != nil {
			errAttrs := metric.WithAttributes(
				attribute.String("launchctl.action", action),
				attribute.String("service.name", serviceName),
				attribute.String("status", "error"),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			)
			ilc.metrics.OperationsTotal.Add(ctx, 1, errAttrs)
			ilc.metrics.OperationDuration.Record(ctx, durationMs, errAttrs)
		}
		return output, err
	}

	// 5. Record Success Attributes
	span.SetAttributes(
		attribute.Int("process.exit_code", 0),
		attribute.Int("process.output_length", len(output)),
	)
	span.SetStatus(codes.Ok, fmt.Sprintf("%s %s completed successfully", ilc.executable, action))

	// 6. Record Prometheus Metrics
	if ilc.metrics != nil {
		successAttrs := metric.WithAttributes(
			attribute.String("launchctl.action", action),
			attribute.String("service.name", serviceName),
			attribute.String("status", "success"),
		)
		ilc.metrics.OperationsTotal.Add(ctx, 1, successAttrs)
		ilc.metrics.OperationDuration.Record(ctx, durationMs, successAttrs)

		// Track running state transitions
		switch action {
		case "start", "kickstart":
			ilc.metrics.ServicesRunning.Add(ctx, 1, metric.WithAttributes(attribute.String("service.name", serviceName)))
		case "stop":
			ilc.metrics.ServicesRunning.Add(ctx, -1, metric.WithAttributes(attribute.String("service.name", serviceName)))
		}
	}

	return output, nil
}

// Start initiates the specified service with OpenTelemetry instrumentation.
func (ilc *InstrumentedLaunchController) Start(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "start", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Start(callCtx, serviceName)
	})
}

// Stop terminates the specified service with OpenTelemetry instrumentation.
func (ilc *InstrumentedLaunchController) Stop(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "stop", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Stop(callCtx, serviceName)
	})
}

// Kickstart restarts or activates the specified service with OpenTelemetry instrumentation.
func (ilc *InstrumentedLaunchController) Kickstart(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "kickstart", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Kickstart(callCtx, serviceName)
	})
}

// Print inspects the runtime state of the specified service with OpenTelemetry instrumentation.
func (ilc *InstrumentedLaunchController) Print(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "print", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Print(callCtx, serviceName)
	})
}

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config defines telemetry configuration options for the application.
type Config struct {
	// ServiceName is the logical name of this microservice / application.
	ServiceName string
	// ServiceVersion is the semver version string of this service.
	ServiceVersion string
	// Environment represents the deployment tier (e.g. "development", "staging", "production").
	Environment string
	// ExporterType can be "otlp" (for Collector / Jaeger) or "stdout" (for local console printing).
	ExporterType string
	// OTLPEndpoint is the host:port for the OpenTelemetry Collector (default: "localhost:4317").
	OTLPEndpoint string
	// Insecure configures gRPC connection without TLS (useful for local dev).
	Insecure bool
	// MetricExportInterval defines how frequently metrics are pushed to the collector.
	MetricExportInterval time.Duration
}

// DefaultConfig returns reasonable defaults, reading environment overrides when present.
func DefaultConfig(serviceName string) Config {
	exporterType := os.Getenv("OTEL_EXPORTER_TYPE")
	if exporterType == "" {
		exporterType = "otlp"
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	env := os.Getenv("DEPLOYMENT_ENV")
	if env == "" {
		env = "development"
	}

	return Config{
		ServiceName:          serviceName,
		ServiceVersion:       "1.0.0",
		Environment:          env,
		ExporterType:         exporterType,
		OTLPEndpoint:         endpoint,
		Insecure:             true,
		MetricExportInterval: 3 * time.Second, // short interval for interactive tutorial demos
	}
}

// Setup initializes the OpenTelemetry SDK (TracerProvider, MeterProvider, and Propagators).
// It returns a cleanup function that flushes pending spans/metrics on shutdown.
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// Helper to run all registered shutdown functions upon error or termination
	shutdown = func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if fnErr := fn(ctx); fnErr != nil {
				errs = append(errs, fnErr)
			}
		}
		return errors.Join(errs...)
	}

	// 1. Build Resource description (identifies the service sending the telemetry)
	res, resErr := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if resErr != nil {
		return shutdown, fmt.Errorf("failed to create resource: %w", resErr)
	}

	// 2. Set up W3C Context Propagators (TraceContext for traceparent, Baggage for user baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 3. Initialize TracerProvider based on selected exporter
	tp, tpErr := initTracerProvider(ctx, cfg, res)
	if tpErr != nil {
		return shutdown, fmt.Errorf("failed to initialize tracer provider: %w", tpErr)
	}
	otel.SetTracerProvider(tp)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)

	// 4. Initialize MeterProvider based on selected exporter
	mp, mpErr := initMeterProvider(ctx, cfg, res)
	if mpErr != nil {
		return shutdown, fmt.Errorf("failed to initialize meter provider: %w", mpErr)
	}
	otel.SetMeterProvider(mp)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)

	return shutdown, nil
}

func initTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	var spanExporter sdktrace.SpanExporter
	var err error

	switch cfg.ExporterType {
	case "stdout":
		spanExporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("creating stdout trace exporter: %w", err)
		}
		// In stdout mode, WithSyncer emits immediately upon span completion
		return sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(spanExporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		), nil

	case "otlp":
		fallthrough
	default:
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		spanExporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP gRPC trace exporter: %w", err)
		}

		// In production/OTLP, WithBatcher collects spans in batches before sending
		return sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(spanExporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		), nil
	}
}

func initMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	var metricExporter sdkmetric.Exporter
	var err error

	switch cfg.ExporterType {
	case "stdout":
		metricExporter, err = stdoutmetric.New(
			stdoutmetric.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("creating stdout metric exporter: %w", err)
		}

	case "otlp":
		fallthrough
	default:
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}

		metricExporter, err = otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP gRPC metric exporter: %w", err)
		}
	}

	reader := sdkmetric.NewPeriodicReader(
		metricExporter,
		sdkmetric.WithInterval(cfg.MetricExportInterval),
	)

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	), nil
}

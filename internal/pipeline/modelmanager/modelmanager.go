package modelmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ModelManager manages the lifecycle of local AI models (llama.cpp, whisper.cpp, bge).
// In memory-constrained environments (e.g. edge devices or laptops with limited VRAM),
// only ONE model can reside in memory at a time.
type ModelManager interface {
	// LoadModel ensures the requested model is loaded into memory/VRAM.
	// If a different model is currently resident, it is evicted first.
	// Returns a release function that unloads the model when the stage is complete.
	LoadModel(ctx context.Context, modelName string, sizeBytes int64) (release func(context.Context), err error)

	// ActiveModel returns the name of the model currently resident in memory (or empty).
	ActiveModel() string

	// AllocatedVRAM returns current bytes occupied in memory/VRAM.
	AllocatedVRAM() int64
}

// localModelManager implements ModelManager with OpenTelemetry instrumentation.
type localModelManager struct {
	mu          sync.Mutex
	activeModel string
	sizeBytes   int64
	deviceType  string

	tracer       trace.Tracer
	vramGauge    metric.Int64UpDownCounter
	loadDuration metric.Float64Histogram
	loadCount    metric.Int64Counter
	unloadCount  metric.Int64Counter
}

// New creates a new ModelManager configured with OpenTelemetry instruments.
func New(deviceType string, meter metric.Meter, tracer trace.Tracer) (ModelManager, error) {
	if deviceType == "" {
		deviceType = "metal" // Default to Apple Silicon Metal on macOS
	}
	if meter == nil {
		meter = otel.Meter("ai.model.manager")
	}
	if tracer == nil {
		tracer = otel.Tracer("ai.model.manager")
	}

	vramGauge, err := meter.Int64UpDownCounter(
		"ai.model.vram_allocated_bytes",
		metric.WithDescription("Current VRAM bytes allocated by the active local AI model"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create vram_allocated_bytes metric: %w", err)
	}

	loadDuration, err := meter.Float64Histogram(
		"ai.model.load_duration_ms",
		metric.WithDescription("Latency incurred loading the model into memory/VRAM"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create load_duration_ms metric: %w", err)
	}

	loadCount, err := meter.Int64Counter(
		"ai.model.loads_total",
		metric.WithDescription("Total count of model load operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create loads_total metric: %w", err)
	}

	unloadCount, err := meter.Int64Counter(
		"ai.model.unloads_total",
		metric.WithDescription("Total count of model unload operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create unloads_total metric: %w", err)
	}

	return &localModelManager{
		deviceType:   deviceType,
		tracer:       tracer,
		vramGauge:    vramGauge,
		loadDuration: loadDuration,
		loadCount:    loadCount,
		unloadCount:  unloadCount,
	}, nil
}

// LoadModel loads the target model into VRAM, evicting any previous model if needed.
func (m *localModelManager) LoadModel(ctx context.Context, modelName string, sizeBytes int64) (func(context.Context), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. If this model is already loaded, reuse it
	if m.activeModel == modelName {
		return m.createRelease(modelName), nil
	}

	// 2. If a DIFFERENT model is loaded, evict it first to respect memory constraints
	if m.activeModel != "" {
		m.unloadLocked(ctx)
	}

	// 3. Start OTel Span for model loading
	ctx, span := m.tracer.Start(ctx, "ai.model.load",
		trace.WithAttributes(
			attribute.String("model.name", modelName),
			attribute.Int64("model.size_bytes", sizeBytes),
			attribute.String("device.type", m.deviceType),
		),
	)
	defer span.End()

	loadStart := time.Now()

	// Simulate model weight loading from SSD to VRAM/unified memory
	time.Sleep(80 * time.Millisecond)

	durationMs := float64(time.Since(loadStart).Milliseconds())
	m.activeModel = modelName
	m.sizeBytes = sizeBytes

	// Record Telemetry
	metricAttrs := metric.WithAttributes(
		attribute.String("model.name", modelName),
		attribute.String("device.type", m.deviceType),
	)
	m.vramGauge.Add(ctx, sizeBytes, metricAttrs)
	m.loadDuration.Record(ctx, durationMs, metricAttrs)
	m.loadCount.Add(ctx, 1, metricAttrs)

	span.SetStatus(codes.Ok, fmt.Sprintf("model %s loaded successfully (%d MB)", modelName, sizeBytes/(1024*1024)))

	return m.createRelease(modelName), nil
}

func (m *localModelManager) unloadLocked(ctx context.Context) {
	if m.activeModel == "" {
		return
	}

	prevModel := m.activeModel
	prevSize := m.sizeBytes

	ctx, span := m.tracer.Start(ctx, "ai.model.unload",
		trace.WithAttributes(
			attribute.String("model.name", prevModel),
			attribute.Int64("model.freed_bytes", prevSize),
			attribute.String("device.type", m.deviceType),
		),
	)
	defer span.End()

	metricAttrs := metric.WithAttributes(
		attribute.String("model.name", prevModel),
		attribute.String("device.type", m.deviceType),
	)
	m.vramGauge.Add(ctx, -prevSize, metricAttrs)
	m.unloadCount.Add(ctx, 1, metricAttrs)

	// Simulate memory deallocation
	time.Sleep(15 * time.Millisecond)

	m.activeModel = ""
	m.sizeBytes = 0

	span.SetStatus(codes.Ok, fmt.Sprintf("model %s unloaded, freed %d MB", prevModel, prevSize/(1024*1024)))
}

func (m *localModelManager) createRelease(modelName string) func(context.Context) {
	var once sync.Once
	return func(relCtx context.Context) {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.activeModel == modelName {
				m.unloadLocked(relCtx)
			}
		})
	}
}

func (m *localModelManager) ActiveModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeModel
}

func (m *localModelManager) AllocatedVRAM() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sizeBytes
}

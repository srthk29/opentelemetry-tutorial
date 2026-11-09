package instrumented

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	kickstartPIDRegex = regexp.MustCompile(`service spawned with pid:\s*(\d+)`)
	printPIDRegex     = regexp.MustCompile(`(?m)^\s*pid\s*=\s*(\d+)`)
	listPIDRegex      = regexp.MustCompile(`"PID"\s*=\s*(\d+);`)
)

// ExtractPIDFromOutput attempts to parse the process ID from launchctl kickstart/print/list outputs.
func ExtractPIDFromOutput(output string) (int, bool) {
	if match := kickstartPIDRegex.FindStringSubmatch(output); len(match) > 1 {
		if pid, err := strconv.Atoi(match[1]); err == nil && pid > 0 {
			return pid, true
		}
	}
	if match := printPIDRegex.FindStringSubmatch(output); len(match) > 1 {
		if pid, err := strconv.Atoi(match[1]); err == nil && pid > 0 {
			return pid, true
		}
	}
	if match := listPIDRegex.FindStringSubmatch(output); len(match) > 1 {
		if pid, err := strconv.Atoi(match[1]); err == nil && pid > 0 {
			return pid, true
		}
	}
	return 0, false
}

// ProcessStats holds CPU and memory resource consumption metrics for a process.
type ProcessStats struct {
	PID          int     `json:"pid"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	RSSBytes     int64   `json:"rss_bytes"`
	VSZBytes     int64   `json:"vsz_bytes,omitempty"`
}

// JCProcessRecord represents an individual JSON item emitted by "jc --ps".
type JCProcessRecord struct {
	PID        int     `json:"pid"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	RSS        int64   `json:"rss"`
	VSZ        int64   `json:"vsz"`
}

type serviceMonitor struct {
	serviceName string
	pid         int
	cancel      context.CancelFunc
}

// InstrumentedLaunchController wraps a LaunchController with OpenTelemetry
// traces, Prometheus metrics, and hybrid CPU/memory resource monitoring.
type InstrumentedLaunchController struct {
	controller   LaunchController
	tracer       trace.Tracer
	metrics      *LaunchctlMetrics
	executable   string
	psRunner     CommandRunner
	pollInterval time.Duration
	monitorsMu   sync.Mutex
	monitors     map[string]*serviceMonitor
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
		controller:   controller,
		tracer:       otel.Tracer("launchctl-controller"),
		metrics:      metrics,
		executable:   "launchctl",
		psRunner:     DefaultCommandRunner,
		pollInterval: 0, // 0 = snapshot-only by default; configure via WithPollInterval for background polling
		monitors:     make(map[string]*serviceMonitor),
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
		psRunner:   DefaultCommandRunner,
		monitors:   make(map[string]*serviceMonitor),
	}
}

// SetPollInterval enables continuous background polling of CPU/Memory for running services.
// Setting interval > 0 activates the hybrid continuous monitoring mode.
func (ilc *InstrumentedLaunchController) SetPollInterval(interval time.Duration) {
	ilc.monitorsMu.Lock()
	defer ilc.monitorsMu.Unlock()
	ilc.pollInterval = interval
}

// SetPSRunner configures a custom command runner for "ps ... | jc --ps" (ideal for tests and sandboxes).
func (ilc *InstrumentedLaunchController) SetPSRunner(runner CommandRunner) {
	ilc.psRunner = runner
}

// Controller returns the underlying LaunchController.
func (ilc *InstrumentedLaunchController) Controller() LaunchController {
	return ilc.controller
}

// InspectProcessStats inspects CPU, memory percentage, and RSS bytes for a given PID.
// It executes "ps -p $PID -o %cpu,%mem,rss,vsz | jc --ps", with an automatic pure-Go fallback parser.
func (ilc *InstrumentedLaunchController) InspectProcessStats(ctx context.Context, pid int) (*ProcessStats, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid: %d", pid)
	}

	runner := ilc.psRunner
	if runner == nil {
		runner = DefaultCommandRunner
	}

	// 1. Try running ps piped to jc --ps
	cmdStr := fmt.Sprintf("ps -p %d -o %%cpu,%%mem,rss,vsz | jc --ps", pid)
	out, err := runner(ctx, "sh", "-c", cmdStr)
	if err == nil && len(out) > 0 {
		var records []JCProcessRecord
		if jsonErr := json.Unmarshal(out, &records); jsonErr == nil && len(records) > 0 {
			return &ProcessStats{
				PID:        pid,
				CPUPercent: records[0].CPUPercent,
				MemPercent: records[0].MemPercent,
				RSSBytes:   records[0].RSS * 1024, // Convert KB to Bytes
				VSZBytes:   records[0].VSZ * 1024,
			}, nil
		}
	}

	// 2. Resilience Fallback: Execute plain ps and parse space-delimited table in Go
	psOut, psErr := runner(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "%cpu,%mem,rss,vsz")
	if psErr != nil {
		return nil, fmt.Errorf("failed to inspect process %d: %w", pid, psErr)
	}

	return parsePlainPSOutput(pid, string(psOut))
}

func parsePlainPSOutput(pid int, output string) (*ProcessStats, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected ps output: %q", output)
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return nil, fmt.Errorf("insufficient fields in ps output: %q", output)
	}

	cpu, _ := strconv.ParseFloat(fields[0], 64)
	mem, _ := strconv.ParseFloat(fields[1], 64)
	rssKB, _ := strconv.ParseInt(fields[2], 10, 64)
	var vszKB int64
	if len(fields) >= 4 {
		vszKB, _ = strconv.ParseInt(fields[3], 10, 64)
	}

	return &ProcessStats{
		PID:        pid,
		CPUPercent: cpu,
		MemPercent: mem,
		RSSBytes:   rssKB * 1024,
		VSZBytes:   vszKB * 1024,
	}, nil
}

func (ilc *InstrumentedLaunchController) recordStats(ctx context.Context, serviceName string, stats *ProcessStats, span trace.Span) {
	if stats == nil {
		return
	}

	if span != nil {
		span.SetAttributes(
			attribute.Int("process.pid", stats.PID),
			attribute.Float64("process.cpu.utilization", stats.CPUPercent),
			attribute.Float64("process.memory.utilization", stats.MemPercent),
			attribute.Int64("process.memory.usage", stats.RSSBytes),
		)
	}

	if ilc.metrics != nil {
		metricAttrs := metric.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.Int("process.pid", stats.PID),
		)
		ilc.metrics.ProcessCPU.Record(ctx, stats.CPUPercent, metricAttrs)
		ilc.metrics.ProcessMemoryPerc.Record(ctx, stats.MemPercent, metricAttrs)
		ilc.metrics.ProcessMemoryRSS.Record(ctx, stats.RSSBytes, metricAttrs)
	}
}

func (ilc *InstrumentedLaunchController) startBackgroundMonitoring(serviceName string, pid int) {
	ilc.monitorsMu.Lock()
	defer ilc.monitorsMu.Unlock()

	if ilc.pollInterval <= 0 || pid <= 0 {
		return
	}

	// Cancel existing monitor if already running
	if existing, found := ilc.monitors[serviceName]; found {
		existing.cancel()
		delete(ilc.monitors, serviceName)
	}

	monitorCtx, cancel := context.WithCancel(context.Background())
	ilc.monitors[serviceName] = &serviceMonitor{
		serviceName: serviceName,
		pid:         pid,
		cancel:      cancel,
	}

	go func(ctx context.Context, sName string, p int, interval time.Duration) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := ilc.InspectProcessStats(ctx, p)
				if err != nil {
					// Process likely stopped or disappeared
					return
				}
				ilc.recordStats(ctx, sName, stats, nil)
			}
		}
	}(monitorCtx, serviceName, pid, ilc.pollInterval)
}

func (ilc *InstrumentedLaunchController) stopBackgroundMonitoring(serviceName string) {
	ilc.monitorsMu.Lock()
	defer ilc.monitorsMu.Unlock()

	if monitor, found := ilc.monitors[serviceName]; found {
		monitor.cancel()
		delete(ilc.monitors, serviceName)
	}
}

// StopAllMonitors terminates all active background resource monitoring goroutines.
func (ilc *InstrumentedLaunchController) StopAllMonitors() {
	ilc.monitorsMu.Lock()
	defer ilc.monitorsMu.Unlock()

	for sName, monitor := range ilc.monitors {
		monitor.cancel()
		delete(ilc.monitors, sName)
	}
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
		args = []string{ilc.executable, action, "-k", "-p", serviceName}
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
			ilc.stopBackgroundMonitoring(serviceName)
		}
	}

	// 6. Point-in-time Snapshot & Hybrid Resource Monitoring
	pid, hasPID := ExtractPIDFromOutput(output)
	if !hasPID && action == "start" {
		// For legacy launchctl start, attempt to discover PID from print
		if printOut, printErr := ilc.controller.Print(ctx, serviceName); printErr == nil {
			pid, hasPID = ExtractPIDFromOutput(printOut)
		}
	}

	if hasPID && pid > 0 {
		stats, statsErr := ilc.InspectProcessStats(ctx, pid)
		if statsErr == nil && stats != nil {
			ilc.recordStats(ctx, serviceName, stats, span)
		}

		if action == "start" || action == "kickstart" {
			ilc.startBackgroundMonitoring(serviceName, pid)
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

// Stop terminates the specified service with OpenTelemetry instrumentation and halts monitoring.
func (ilc *InstrumentedLaunchController) Stop(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "stop", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Stop(callCtx, serviceName)
	})
}

// Kickstart restarts or activates the service with -k -p, extracting the spawned PID and inspecting resources.
func (ilc *InstrumentedLaunchController) Kickstart(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "kickstart", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Kickstart(callCtx, serviceName)
	})
}

// Print inspects the runtime state of the specified service, extracting the PID and current CPU/memory.
func (ilc *InstrumentedLaunchController) Print(ctx context.Context, serviceName string) (string, error) {
	return ilc.execute(ctx, "print", serviceName, func(callCtx context.Context) (string, error) {
		return ilc.controller.Print(callCtx, serviceName)
	})
}

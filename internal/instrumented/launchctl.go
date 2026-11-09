package instrumented

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LaunchController defines the service lifecycle control interface for macOS launchctl.
type LaunchController interface {
	Start(ctx context.Context, serviceName string) (string, error)
	Stop(ctx context.Context, serviceName string) (string, error)
	Kickstart(ctx context.Context, serviceName string) (string, error)
	Print(ctx context.Context, serviceName string) (string, error)
}

// CommandRunner defines the signature for executing command-line processes.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// DefaultCommandRunner executes commands using os/exec.
func DefaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// ExecLaunchController is the standard implementation that executes launchctl CLI commands.
type ExecLaunchController struct {
	runner     CommandRunner
	domain     string // e.g. "gui/501" or ""
	binaryPath string // defaults to "launchctl"
}

// Compile-time assertion that ExecLaunchController implements LaunchController.
var _ LaunchController = (*ExecLaunchController)(nil)

// NewExecLaunchController creates a LaunchController using the system launchctl binary.
func NewExecLaunchController(domain string) *ExecLaunchController {
	return &ExecLaunchController{
		runner:     DefaultCommandRunner,
		domain:     domain,
		binaryPath: "launchctl",
	}
}

// NewExecLaunchControllerWithRunner creates a LaunchController with a custom command runner.
func NewExecLaunchControllerWithRunner(runner CommandRunner, domain string) *ExecLaunchController {
	if runner == nil {
		runner = DefaultCommandRunner
	}
	return &ExecLaunchController{
		runner:     runner,
		domain:     domain,
		binaryPath: "launchctl",
	}
}

func (c *ExecLaunchController) target(serviceName string) string {
	if c.domain != "" && !strings.Contains(serviceName, "/") {
		return fmt.Sprintf("%s/%s", c.domain, serviceName)
	}
	return serviceName
}

// Start initiates the specified service via launchctl.
func (c *ExecLaunchController) Start(ctx context.Context, serviceName string) (string, error) {
	target := c.target(serviceName)
	out, err := c.runner(ctx, c.binaryPath, "start", target)
	return string(out), err
}

// Stop terminates the specified service via launchctl.
func (c *ExecLaunchController) Stop(ctx context.Context, serviceName string) (string, error) {
	target := c.target(serviceName)
	out, err := c.runner(ctx, c.binaryPath, "stop", target)
	return string(out), err
}

// Kickstart restarts or immediately activates the specified service via launchctl.
// Passes -k to terminate existing instance and -p to print the spawned PID.
func (c *ExecLaunchController) Kickstart(ctx context.Context, serviceName string) (string, error) {
	target := c.target(serviceName)
	out, err := c.runner(ctx, c.binaryPath, "kickstart", "-k", "-p", target)
	return string(out), err
}

// Print inspects the runtime state of the specified service via launchctl.
func (c *ExecLaunchController) Print(ctx context.Context, serviceName string) (string, error) {
	target := c.target(serviceName)
	out, err := c.runner(ctx, c.binaryPath, "print", target)
	return string(out), err
}

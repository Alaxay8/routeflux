package xray

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Alaxay8/routeflux/internal/backend"
)

// InitdController manages Xray through the OpenWrt init.d script.
type InitdController struct {
	ScriptPath string
}

// Start starts the Xray service.
func (c InitdController) Start(ctx context.Context) error {
	return c.run(ctx, "start")
}

// Stop stops the Xray service.
func (c InitdController) Stop(ctx context.Context) error {
	return c.run(ctx, "stop")
}

// Reload reloads the Xray service.
func (c InitdController) Reload(ctx context.Context) error {
	return c.run(ctx, "reload")
}

// Status reports whether the command succeeds.
func (c InitdController) Status(ctx context.Context) (backend.RuntimeStatus, error) {
	if c.ScriptPath == "" {
		return backend.RuntimeStatus{}, fmt.Errorf("xray service script path is not configured")
	}

	cmd := exec.CommandContext(ctx, c.ScriptPath, "status")
	output, err := cmd.CombinedOutput()
	serviceState := strings.TrimSpace(string(output))
	if serviceState == "" {
		serviceState = "unknown"
	}

	status := backend.RuntimeStatus{
		ConfigPath:   c.ScriptPath,
		ServiceState: serviceState,
		Running:      statusOutputLooksRunning(serviceState),
	}

	if err != nil {
		status.Running = false
		return status, fmt.Errorf("run %s status: %w: %s", c.ScriptPath, err, string(output))
	}

	return status, nil
}

func (c InitdController) run(ctx context.Context, action string) error {
	if c.ScriptPath == "" {
		return fmt.Errorf("xray service script path is not configured")
	}

	cmd := exec.CommandContext(ctx, c.ScriptPath, action)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run %s %s: %w: %s", c.ScriptPath, action, err, string(output))
	}

	return nil
}

func statusOutputLooksRunning(output string) bool {
	normalized := strings.ToLower(strings.TrimSpace(output))
	switch {
	case normalized == "", normalized == "unknown":
		return false
	case strings.Contains(normalized, "no instances"):
		return false
	case strings.Contains(normalized, "inactive"):
		return false
	case strings.Contains(normalized, "not running"):
		return false
	case strings.Contains(normalized, "stopped"):
		return false
	case strings.Contains(normalized, "running"):
		return true
	case strings.Contains(normalized, "active"):
		return true
	default:
		return false
	}
}

package xray

import (
	"context"
	"fmt"
	"os/exec"

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
	err := c.run(ctx, "status")
	status := backend.RuntimeStatus{ConfigPath: c.ScriptPath}
	if err == nil {
		status.Running = true
		status.ServiceState = "running"
		return status, nil
	}

	status.Running = false
	status.ServiceState = "unknown"
	return status, err
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

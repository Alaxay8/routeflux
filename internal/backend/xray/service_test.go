package xray

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitdControllerStatusDetectsActiveWithNoInstances(t *testing.T) {
	t.Parallel()

	script := writeStatusScript(t, "#!/bin/sh\necho 'active with no instances'\nexit 0\n")
	controller := InitdController{ScriptPath: script}

	status, err := controller.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if status.Running {
		t.Fatal("expected runtime to be reported as not running")
	}
	if status.ServiceState != "active with no instances" {
		t.Fatalf("unexpected service state: %q", status.ServiceState)
	}
}

func TestInitdControllerStatusDetectsRunningProcess(t *testing.T) {
	t.Parallel()

	script := writeStatusScript(t, "#!/bin/sh\necho 'running'\nexit 0\n")
	controller := InitdController{ScriptPath: script}

	status, err := controller.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !status.Running {
		t.Fatal("expected runtime to be reported as running")
	}
	if status.ServiceState != "running" {
		t.Fatalf("unexpected service state: %q", status.ServiceState)
	}
}

func TestRuntimeBackendStatusUsesConfigPath(t *testing.T) {
	t.Parallel()

	script := writeStatusScript(t, "#!/bin/sh\necho 'running'\nexit 0\n")
	backend := NewRuntimeBackend("/etc/xray/config.json", InitdController{ScriptPath: script})

	status, err := backend.Status(context.Background())
	if err != nil {
		t.Fatalf("backend status: %v", err)
	}

	if status.ConfigPath != "/etc/xray/config.json" {
		t.Fatalf("unexpected config path: %q", status.ConfigPath)
	}
}

func writeStatusScript(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "xray-status.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write status script: %v", err)
	}

	return path
}

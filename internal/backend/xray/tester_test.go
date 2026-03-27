package xray

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandTesterRejectsEmptyConfigPath(t *testing.T) {
	t.Parallel()

	tester := CommandTester{}
	if err := tester.Test(context.Background(), ""); err == nil {
		t.Fatal("expected empty config path to fail")
	}
}

func TestCommandTesterRunsConfiguredBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	binaryPath := writeExecutable(t, filepath.Join(dir, "xray"), "#!/bin/sh\nif [ \"$1\" = \"-test\" ] && [ \"$2\" = \"-config\" ] && [ -f \"$3\" ]; then\n  exit 0\nfi\nexit 1\n")
	tester := CommandTester{BinaryPath: binaryPath}

	if err := tester.Test(context.Background(), configPath); err != nil {
		t.Fatalf("test config: %v", err)
	}
}

func TestCommandTesterReturnsCommandOutputOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	binaryPath := writeExecutable(t, filepath.Join(dir, "xray"), "#!/bin/sh\necho 'invalid xray config'\nexit 1\n")
	tester := CommandTester{BinaryPath: binaryPath}

	err := tester.Test(context.Background(), configPath)
	if err == nil {
		t.Fatal("expected test config to fail")
	}
	if !strings.Contains(err.Error(), "invalid xray config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCommandTesterUsesEnvironmentBinaryPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binaryPath := writeExecutable(t, filepath.Join(dir, "xray"), "#!/bin/sh\nexit 0\n")

	old := os.Getenv("ROUTEFLUX_XRAY_BINARY")
	t.Cleanup(func() {
		if old == "" {
			_ = os.Unsetenv("ROUTEFLUX_XRAY_BINARY")
		} else {
			_ = os.Setenv("ROUTEFLUX_XRAY_BINARY", old)
		}
	})
	if err := os.Setenv("ROUTEFLUX_XRAY_BINARY", binaryPath); err != nil {
		t.Fatalf("set env: %v", err)
	}

	tester := NewCommandTester()
	if tester.BinaryPath != binaryPath {
		t.Fatalf("unexpected binary path: %q", tester.BinaryPath)
	}
}

func TestFileWriterWritePersistsRawJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writer := FileWriter{Path: path}
	want := []byte("{\"log\":{\"loglevel\":\"warning\"}}\n")

	if err := writer.Write(want); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !jsonEqual(t, got, want) {
		t.Fatalf("unexpected config\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func writeExecutable(t *testing.T, path, body string) string {
	t.Helper()

	return writeTestExecutable(t, path, body)
}

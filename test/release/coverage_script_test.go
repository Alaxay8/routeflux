package release_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverageScriptReportsGoTestFailure(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/sh
set -eu
[ "${1:-}" = "test" ] || exit 1
pkg="${3:-}"
case "$pkg" in
	./internal/backend/xray)
		printf 'simulated go test failure for %s\n' "$pkg" >&2
		exit 1
		;;
	./internal/probe)
		printf 'ok  	example/internal/probe	coverage: 90.8%% of statements\n'
		;;
	./internal/app)
		printf 'ok  	example/internal/app	coverage: 69.5%% of statements\n'
		;;
	*)
		printf 'unexpected package %s\n' "$pkg" >&2
		exit 1
		;;
esac
`)

	stdout, stderr, err := runCoverageScript(t, binDir)
	if err == nil {
		t.Fatalf("expected coverage script to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "checking coverage for ./internal/backend/xray") {
		t.Fatalf("expected package banner in output, got %q", combined)
	}
	if !strings.Contains(combined, "simulated go test failure for ./internal/backend/xray") {
		t.Fatalf("expected go test failure details in output, got %q", combined)
	}
	if !strings.Contains(combined, "go test failed for ./internal/backend/xray") {
		t.Fatalf("expected explicit go test failure message, got %q", combined)
	}
}

func TestCoverageScriptFailsBelowThreshold(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/sh
set -eu
[ "${1:-}" = "test" ] || exit 1
pkg="${3:-}"
case "$pkg" in
	./internal/backend/xray)
		printf 'ok  	example/internal/backend/xray	coverage: 79.9%% of statements\n'
		;;
	./internal/probe)
		printf 'ok  	example/internal/probe	coverage: 90.8%% of statements\n'
		;;
	./internal/app)
		printf 'ok  	example/internal/app	coverage: 64.9%% of statements\n'
		;;
	*)
		printf 'unexpected package %s\n' "$pkg" >&2
		exit 1
		;;
esac
`)

	stdout, stderr, err := runCoverageScript(t, binDir)
	if err == nil {
		t.Fatalf("expected coverage script to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "coverage gate failed for ./internal/app: got 64.9%, need 65.0%") {
		t.Fatalf("expected threshold failure in output, got %q", combined)
	}
}

func TestCoverageScriptAcceptsWholeNumberCoverage(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/bin/sh
set -eu
[ "${1:-}" = "test" ] || exit 1
pkg="${3:-}"
case "$pkg" in
	./internal/backend/xray)
		printf 'ok  	example/internal/backend/xray	coverage: 50%% of statements\n'
		;;
	./internal/probe)
		printf 'ok  	example/internal/probe	coverage: 60%% of statements\n'
		;;
	./internal/app)
		printf 'ok  	example/internal/app	coverage: 65%% of statements\n'
		;;
	*)
		printf 'unexpected package %s\n' "$pkg" >&2
		exit 1
		;;
esac
`)

	stdout, stderr, err := runCoverageScript(t, binDir)
	if err != nil {
		t.Fatalf("expected coverage script to accept whole-number coverage\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

func runCoverageScript(t *testing.T, binDir string) (string, string, error) {
	t.Helper()

	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "coverage-runtime.sh"))
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

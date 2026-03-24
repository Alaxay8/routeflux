package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectPathDetectsExecutableFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "routeflux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}

	status := inspectPath(path)

	if !status.Exists {
		t.Fatal("expected path to exist")
	}
	if !status.Executable {
		t.Fatal("expected file to be executable")
	}
	if status.Directory {
		t.Fatal("expected regular file, not directory")
	}
}

func TestInspectPathDetectsBrokenSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "routeflux")
	target := filepath.Join(dir, "missing-target")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	status := inspectPath(path)

	if !status.Exists {
		t.Fatal("expected symlink path to exist")
	}
	if !status.IsSymlink {
		t.Fatal("expected symlink to be detected")
	}
	if status.Executable {
		t.Fatal("expected broken symlink to be non-executable")
	}
	if !strings.Contains(strings.ToLower(status.Error), "no such file or directory") {
		t.Fatalf("expected missing target error, got %q", status.Error)
	}
}

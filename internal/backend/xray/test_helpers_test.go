package xray

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestExecutable(t *testing.T, path, body string) string {
	t.Helper()

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		t.Fatalf("create executable temp file %s: %v", path, err)
	}

	tmpPath := tmpFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.WriteString(body); err != nil {
		_ = tmpFile.Close()
		t.Fatalf("write executable %s: %v", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close executable %s: %v", path, err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		t.Fatalf("chmod executable %s: %v", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		t.Fatalf("rename executable %s: %v", path, err)
	}

	cleanupTemp = false
	return path
}

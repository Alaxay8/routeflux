package speedtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunnerReturnsBusyWhenLockHeld(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "speedtest.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("lock file: %v", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	runner := Runner{
		LockPath:   lockPath,
		BinaryPath: "/usr/bin/xray",
		TempRoot:   t.TempDir(),
		Start: func(context.Context, string, string) (Process, error) {
			return &fakeProcess{}, nil
		},
		Measure: func(context.Context, string, Provider) (Metrics, error) {
			return Metrics{}, nil
		},
		Now: func() time.Time { return time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC) },
	}

	_, err = runner.Test(context.Background(), Request{
		SubscriptionID: "sub-1",
		NodeID:         "node-1",
		NodeName:       "Node 1",
		Config:         []byte(`{}`),
		HTTPProxyPort:  18080,
	})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
}

func TestRunnerCleansUpTempConfigAndStopsProcessOnSuccess(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	proc := &fakeProcess{}
	var configPath string

	runner := Runner{
		LockPath:   filepath.Join(tempRoot, "speedtest.lock"),
		BinaryPath: "/usr/bin/xray",
		TempRoot:   tempRoot,
		Start: func(_ context.Context, _ string, path string) (Process, error) {
			configPath = path
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected config to exist before start: %v", err)
			}
			return proc, nil
		},
		Measure: func(_ context.Context, proxyURL string, provider Provider) (Metrics, error) {
			if proxyURL != "http://127.0.0.1:18080" {
				t.Fatalf("unexpected proxy URL: %s", proxyURL)
			}
			if provider.Name == "" {
				t.Fatal("expected default provider")
			}
			return Metrics{
				Latency:          45 * time.Millisecond,
				DownloadBytes:    8 << 20,
				DownloadDuration: 2 * time.Second,
				UploadBytes:      2 << 20,
				UploadDuration:   time.Second,
			}, nil
		},
		Now: func() time.Time { return time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC) },
	}

	result, err := runner.Test(context.Background(), Request{
		SubscriptionID: "sub-1",
		NodeID:         "node-1",
		NodeName:       "Node 1",
		Config:         []byte(`{"log":{"loglevel":"warning"}}`),
		HTTPProxyPort:  18080,
	})
	if err != nil {
		t.Fatalf("run speed test: %v", err)
	}

	if proc.stopCalls != 1 {
		t.Fatalf("expected process to be stopped once, got %d", proc.stopCalls)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to be removed, got %v", err)
	}
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatalf("read temp root: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "routeflux-speedtest-") {
			t.Fatalf("expected temp dir cleanup, found %s", entry.Name())
		}
	}
	if result.SubscriptionID != "sub-1" || result.NodeID != "node-1" || result.NodeName != "Node 1" {
		t.Fatalf("unexpected metadata: %+v", result)
	}
	if result.DownloadBytes != 8<<20 || result.UploadBytes != 2<<20 {
		t.Fatalf("unexpected transfer sizes: %+v", result)
	}
	if result.StartedAt.IsZero() || result.FinishedAt.IsZero() || !result.FinishedAt.After(result.StartedAt) {
		t.Fatalf("unexpected timestamps: %+v", result)
	}
}

func TestRunnerStopsProcessAndCleansUpOnMeasureFailure(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	proc := &fakeProcess{output: "xray failed"}

	runner := Runner{
		LockPath:   filepath.Join(tempRoot, "speedtest.lock"),
		BinaryPath: "/usr/bin/xray",
		TempRoot:   tempRoot,
		Start: func(_ context.Context, _ string, _ string) (Process, error) {
			return proc, nil
		},
		Measure: func(context.Context, string, Provider) (Metrics, error) {
			return Metrics{}, errors.New("probe failed")
		},
	}

	_, err := runner.Test(context.Background(), Request{
		SubscriptionID: "sub-1",
		NodeID:         "node-1",
		NodeName:       "Node 1",
		Config:         []byte(`{}`),
		HTTPProxyPort:  18080,
	})
	if err == nil || !strings.Contains(err.Error(), "probe failed") {
		t.Fatalf("expected probe failure, got %v", err)
	}
	if proc.stopCalls != 1 {
		t.Fatalf("expected process to be stopped on failure, got %d", proc.stopCalls)
	}
}

type fakeProcess struct {
	stopCalls int
	output    string
}

func (p *fakeProcess) Stop() error {
	p.stopCalls++
	return nil
}

func (p *fakeProcess) Output() string {
	return p.output
}

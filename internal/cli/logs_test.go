package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLogsSnapshotFiltersRouteFluxAndXray(t *testing.T) {
	original := runLogread
	originalTail := readLogTail
	t.Cleanup(func() { runLogread = original })
	t.Cleanup(func() { readLogTail = originalTail })
	runLogread = func(context.Context) ([]byte, error) {
		return []byte(strings.Join([]string{
			"daemon.info routeflux[1]: refreshed subscription",
			"daemon.info xray[2]: listening TCP on 127.0.0.1:10808",
			"daemon.warn dnsmasq[1]: possible DNS-rebind attack detected",
			"daemon.err routeflux[1]: apply firewall failed",
		}, "\n")), nil
	}
	readLogTail = func(string, int) ([]string, error) {
		return nil, os.ErrNotExist
	}

	snapshot := buildLogsSnapshot(context.Background(), 100)

	if !snapshot.Available {
		t.Fatal("expected logs to be available")
	}
	if len(snapshot.RouteFlux) != 2 {
		t.Fatalf("expected 2 routeflux lines, got %d", len(snapshot.RouteFlux))
	}
	if len(snapshot.Xray) != 1 {
		t.Fatalf("expected 1 xray line, got %d", len(snapshot.Xray))
	}
	if len(snapshot.System) != 4 {
		t.Fatalf("expected 4 system lines, got %d", len(snapshot.System))
	}
}

func TestBuildLogsSnapshotReportsLogreadError(t *testing.T) {
	original := runLogread
	originalTail := readLogTail
	t.Cleanup(func() { runLogread = original })
	t.Cleanup(func() { readLogTail = originalTail })
	runLogread = func(context.Context) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}
	readLogTail = func(string, int) ([]string, error) {
		return nil, os.ErrNotExist
	}

	snapshot := buildLogsSnapshot(context.Background(), 100)

	if snapshot.Available {
		t.Fatal("expected logs to be unavailable")
	}
	if !strings.Contains(strings.ToLower(snapshot.Error), "deadline") {
		t.Fatalf("unexpected error: %q", snapshot.Error)
	}
}

func TestBuildLogsSnapshotPrefersXrayLogFile(t *testing.T) {
	original := runLogread
	originalTail := readLogTail
	t.Cleanup(func() { runLogread = original })
	t.Cleanup(func() { readLogTail = originalTail })

	runLogread = func(context.Context) ([]byte, error) {
		return []byte(strings.Join([]string{
			"daemon.info routeflux[1]: refreshed subscription",
			"daemon.info xray[2]: stale syslog line",
		}, "\n")), nil
	}
	readLogTail = func(path string, limit int) ([]string, error) {
		if path != xrayLogPath {
			t.Fatalf("unexpected tail path %q", path)
		}
		if limit != 100 {
			t.Fatalf("unexpected limit %d", limit)
		}
		return []string{
			"2026/03/29 05:04:27 [Info] transport/internet/tcp: listening TCP on 127.0.0.1:10808",
			"2026/03/29 05:04:27 [Warning] core: Xray 26.2.6 started",
		}, nil
	}

	snapshot := buildLogsSnapshot(context.Background(), 100)

	if !snapshot.Available {
		t.Fatal("expected logs to be available")
	}
	if strings.Join(snapshot.Xray, "\n") != strings.Join([]string{
		"2026/03/29 05:04:27 [Info] transport/internet/tcp: listening TCP on 127.0.0.1:10808",
		"2026/03/29 05:04:27 [Warning] core: Xray 26.2.6 started",
	}, "\n") {
		t.Fatalf("unexpected xray lines: %v", snapshot.Xray)
	}
	if snapshot.Source != logreadPath+" + "+xrayLogPath {
		t.Fatalf("unexpected source %q", snapshot.Source)
	}
}

func TestDefaultReadLogTailReturnsFileTail(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "xray.log")
	content := strings.Join([]string{
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	lines, err := defaultReadLogTail(path, 2)
	if err != nil {
		t.Fatalf("defaultReadLogTail returned error: %v", err)
	}

	if strings.Join(lines, ",") != "line-3,line-4" {
		t.Fatalf("unexpected tail %v", lines)
	}
}

func TestLastNReturnsTail(t *testing.T) {
	t.Parallel()

	lines := []string{"a", "b", "c", "d"}
	tail := lastN(lines, 2)

	if strings.Join(tail, ",") != "c,d" {
		t.Fatalf("unexpected tail: %v", tail)
	}
}

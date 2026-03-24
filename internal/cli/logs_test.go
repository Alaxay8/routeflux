package cli

import (
	"context"
	"strings"
	"testing"
)

func TestBuildLogsSnapshotFiltersRouteFluxAndXray(t *testing.T) {
	t.Parallel()

	original := runLogread
	t.Cleanup(func() { runLogread = original })
	runLogread = func(context.Context) ([]byte, error) {
		return []byte(strings.Join([]string{
			"daemon.info routeflux[1]: refreshed subscription",
			"daemon.info xray[2]: listening TCP on 127.0.0.1:10808",
			"daemon.warn dnsmasq[1]: possible DNS-rebind attack detected",
			"daemon.err routeflux[1]: apply firewall failed",
		}, "\n")), nil
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
	t.Parallel()

	original := runLogread
	t.Cleanup(func() { runLogread = original })
	runLogread = func(context.Context) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}

	snapshot := buildLogsSnapshot(context.Background(), 100)

	if snapshot.Available {
		t.Fatal("expected logs to be unavailable")
	}
	if !strings.Contains(strings.ToLower(snapshot.Error), "deadline") {
		t.Fatalf("unexpected error: %q", snapshot.Error)
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

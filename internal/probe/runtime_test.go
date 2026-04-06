package probe_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestTCPCheckerSuccess(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
			accepted <- struct{}{}
		}
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener addr type: %T", listener.Addr())
	}

	checker := probe.TCPChecker{Timeout: time.Second}
	result := checker.Check(context.Background(), domain.Node{
		ID:      "node-1",
		Address: "127.0.0.1",
		Port:    addr.Port,
	})

	if !result.Healthy {
		t.Fatalf("expected healthy result, got %+v", result)
	}
	if result.Latency <= 0 {
		t.Fatalf("expected positive latency, got %s", result.Latency)
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("expected checker connection to be accepted")
	}
}

func TestTCPCheckerFailureUsesTimeoutLatency(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	timeout := 25 * time.Millisecond
	checker := probe.TCPChecker{Timeout: timeout}
	result := checker.Check(context.Background(), domain.Node{
		ID:      "node-1",
		Address: "127.0.0.1",
		Port:    port,
	})

	if result.Healthy {
		t.Fatalf("expected unhealthy result, got %+v", result)
	}
	if result.Err == nil {
		t.Fatal("expected probe error")
	}
	if result.Latency != timeout {
		t.Fatalf("expected timeout latency %s, got %s", timeout, result.Latency)
	}
}

func TestUpdateHealthSuccessClearsFailuresAndTracksAverage(t *testing.T) {
	t.Parallel()

	previous := domain.NodeHealth{
		Healthy:              false,
		FailureCount:         2,
		ConsecutiveFailures:  2,
		ConsecutiveSuccesses: 0,
		LastFailureReason:    "dial tcp timeout",
		AverageLatency:       domain.NewDuration(200 * time.Millisecond),
	}

	updated := probe.UpdateHealth(previous, true, 100*time.Millisecond, time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC), "", probe.DefaultSwitchPolicy().FailureThreshold)
	if !updated.Healthy {
		t.Fatal("expected node to become healthy")
	}
	if updated.ConsecutiveFailures != 0 {
		t.Fatalf("expected failures to reset, got %d", updated.ConsecutiveFailures)
	}
	if updated.ConsecutiveSuccesses != 1 {
		t.Fatalf("expected success counter to increment, got %d", updated.ConsecutiveSuccesses)
	}
	if updated.LastFailureReason != "" {
		t.Fatalf("expected failure reason to clear, got %q", updated.LastFailureReason)
	}
	if updated.AverageLatency.Duration() != 180*time.Millisecond {
		t.Fatalf("unexpected average latency: %s", updated.AverageLatency.Duration())
	}
}

func TestUpdateHealthFailurePreservesHealthyNodeUntilThreshold(t *testing.T) {
	t.Parallel()

	previous := domain.NodeHealth{
		Healthy:              true,
		SuccessCount:         3,
		ConsecutiveSuccesses: 3,
		AverageLatency:       domain.NewDuration(90 * time.Millisecond),
	}

	updated := probe.UpdateHealth(previous, false, 250*time.Millisecond, time.Date(2026, 3, 25, 8, 5, 0, 0, time.UTC), "connection refused", probe.DefaultSwitchPolicy().FailureThreshold)
	if !updated.Healthy {
		t.Fatal("expected node to stay healthy before reaching threshold")
	}
	if updated.ConsecutiveSuccesses != 0 {
		t.Fatalf("expected successes to reset, got %d", updated.ConsecutiveSuccesses)
	}
	if updated.ConsecutiveFailures != 1 {
		t.Fatalf("expected one consecutive failure, got %d", updated.ConsecutiveFailures)
	}
	if updated.LastFailureReason != "connection refused" {
		t.Fatalf("unexpected failure reason: %q", updated.LastFailureReason)
	}
	if updated.AverageLatency.Duration() != 90*time.Millisecond {
		t.Fatalf("expected average latency to be preserved, got %s", updated.AverageLatency.Duration())
	}
}

func TestUpdateHealthFailureMarksNodeUnhealthyAtThreshold(t *testing.T) {
	t.Parallel()

	threshold := probe.DefaultSwitchPolicy().FailureThreshold
	previous := domain.NodeHealth{
		Healthy:              true,
		SuccessCount:         2,
		FailureCount:         2,
		ConsecutiveFailures:  threshold - 1,
		ConsecutiveSuccesses: 0,
		AverageLatency:       domain.NewDuration(90 * time.Millisecond),
	}

	updated := probe.UpdateHealth(previous, false, 250*time.Millisecond, time.Date(2026, 3, 25, 8, 10, 0, 0, time.UTC), "connection refused", threshold)
	if updated.Healthy {
		t.Fatal("expected node to become unhealthy after threshold breach")
	}
	if updated.ConsecutiveFailures != threshold {
		t.Fatalf("unexpected consecutive failures: got %d want %d", updated.ConsecutiveFailures, threshold)
	}
}

func TestUpdateHealthFailureKeepsNewNodeUnhealthyWithoutSuccessHistory(t *testing.T) {
	t.Parallel()

	updated := probe.UpdateHealth(domain.NodeHealth{}, false, 250*time.Millisecond, time.Date(2026, 3, 25, 8, 15, 0, 0, time.UTC), "connection refused", probe.DefaultSwitchPolicy().FailureThreshold)
	if updated.Healthy {
		t.Fatal("expected node without success history to remain unhealthy")
	}
}

func TestSelectBestNodeKeepsStableOrderOnTie(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "node-1", Name: "One"},
		{ID: "node-2", Name: "Two"},
	}

	best, result, err := probe.SelectBestNode(nodes, map[string]domain.NodeHealth{}, probe.DefaultScoreConfig())
	if err != nil {
		t.Fatalf("select best node: %v", err)
	}
	if best.ID != "node-1" {
		t.Fatalf("expected stable tie-break to keep first node, got %s", best.ID)
	}
	if !result.Selected {
		t.Fatal("expected best result to be marked selected")
	}
}

func TestSelectBestNodeFailsOnEmptyInput(t *testing.T) {
	t.Parallel()

	if _, _, err := probe.SelectBestNode(nil, nil, probe.DefaultScoreConfig()); err == nil {
		t.Fatal("expected empty node list to fail")
	}
}

func TestShouldSwitchRejectsUnhealthyCandidate(t *testing.T) {
	t.Parallel()

	current := domain.NodeHealth{
		NodeID:         "current",
		Healthy:        true,
		AverageLatency: domain.NewDuration(90 * time.Millisecond),
	}
	candidate := domain.NodeHealth{
		NodeID:         "candidate",
		Healthy:        false,
		AverageLatency: domain.NewDuration(20 * time.Millisecond),
	}

	should, reason := probe.ShouldSwitch(current, candidate, time.Now().UTC(), time.Now().Add(-10*time.Minute), probe.DefaultSwitchPolicy())
	if should {
		t.Fatalf("expected unhealthy candidate to be rejected, got reason %q", reason)
	}
}

func BenchmarkCalculateScore(b *testing.B) {
	health := domain.NodeHealth{
		NodeID:               "node-1",
		Healthy:              true,
		LastLatency:          domain.NewDuration(100 * time.Millisecond),
		AverageLatency:       domain.NewDuration(80 * time.Millisecond),
		ConsecutiveSuccesses: 5,
		SuccessCount:         12,
		FailureCount:         1,
	}
	cfg := probe.DefaultScoreConfig()

	for i := 0; i < b.N; i++ {
		result := probe.CalculateScore(health, cfg)
		if result.NodeID == "" {
			b.Fatal(fmt.Errorf("unexpected empty node id"))
		}
	}
}

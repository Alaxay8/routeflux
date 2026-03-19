package probe_test

import (
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestCalculateScore(t *testing.T) {
	t.Parallel()

	cfg := probe.DefaultScoreConfig()

	tests := []struct {
		name   string
		health domain.NodeHealth
		wantOK bool
	}{
		{
			name: "healthy node gets positive score",
			health: domain.NodeHealth{
				NodeID:               "a",
				Healthy:              true,
				LastLatency:          domain.NewDuration(120 * time.Millisecond),
				AverageLatency:       domain.NewDuration(100 * time.Millisecond),
				ConsecutiveSuccesses: 5,
				FailureCount:         1,
			},
			wantOK: true,
		},
		{
			name: "unhealthy node is penalized",
			health: domain.NodeHealth{
				NodeID:              "b",
				Healthy:             false,
				LastLatency:         domain.NewDuration(50 * time.Millisecond),
				ConsecutiveFailures: 4,
				FailureCount:        10,
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := probe.CalculateScore(tt.health, cfg)
			if tt.wantOK && (!result.Healthy || result.Score <= 0) {
				t.Fatalf("expected positive healthy score, got %+v", result)
			}

			if !tt.wantOK && (result.Healthy || result.Score >= 0) {
				t.Fatalf("expected unhealthy negative score, got %+v", result)
			}
		})
	}
}

func TestShouldSwitch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	policy := probe.DefaultSwitchPolicy()

	current := domain.NodeHealth{
		NodeID:               "current",
		Healthy:              true,
		AverageLatency:       domain.NewDuration(180 * time.Millisecond),
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 3,
	}

	better := domain.NodeHealth{
		NodeID:               "better",
		Healthy:              true,
		AverageLatency:       domain.NewDuration(80 * time.Millisecond),
		ConsecutiveSuccesses: 4,
	}

	should, _ := probe.ShouldSwitch(current, better, now, now.Add(-2*time.Hour), policy)
	if !should {
		t.Fatal("expected switch to better node")
	}

	should, _ = probe.ShouldSwitch(current, better, now, now.Add(-1*time.Minute), policy)
	if should {
		t.Fatal("expected cooldown to prevent switch")
	}

	failing := current
	failing.Healthy = false
	failing.ConsecutiveFailures = policy.FailureThreshold

	should, reason := probe.ShouldSwitch(failing, better, now, now.Add(-1*time.Minute), policy)
	if !should {
		t.Fatal("expected unhealthy current node to trigger switch")
	}

	if reason == "" {
		t.Fatal("expected switch reason")
	}
}

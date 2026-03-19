package probe

import (
	"fmt"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// SwitchPolicy controls anti-flap behavior.
type SwitchPolicy struct {
	Cooldown              time.Duration
	LatencyImprovement    time.Duration
	FailureThreshold      int
	HealthyLatencyCeiling time.Duration
}

// DefaultSwitchPolicy returns conservative switching defaults.
func DefaultSwitchPolicy() SwitchPolicy {
	return SwitchPolicy{
		Cooldown:              5 * time.Minute,
		LatencyImprovement:    50 * time.Millisecond,
		FailureThreshold:      3,
		HealthyLatencyCeiling: 300 * time.Millisecond,
	}
}

// ShouldSwitch decides whether the selector should move to the candidate node.
func ShouldSwitch(current, candidate domain.NodeHealth, now, lastSwitch time.Time, policy SwitchPolicy) (bool, string) {
	if candidate.NodeID == "" || !candidate.Healthy {
		return false, ""
	}

	if current.NodeID == "" {
		return true, "no current node"
	}

	if !current.Healthy || current.ConsecutiveFailures >= policy.FailureThreshold {
		return true, "current node unhealthy"
	}

	if now.Sub(lastSwitch) < policy.Cooldown {
		return false, "cooldown active"
	}

	currentLatency := current.AverageLatency.Duration()
	if currentLatency <= 0 {
		currentLatency = current.LastLatency.Duration()
	}
	candidateLatency := candidate.AverageLatency.Duration()
	if candidateLatency <= 0 {
		candidateLatency = candidate.LastLatency.Duration()
	}

	if currentLatency > 0 &&
		currentLatency <= policy.HealthyLatencyCeiling &&
		currentLatency-candidateLatency < policy.LatencyImprovement {
		return false, "improvement below threshold"
	}

	if candidateLatency > 0 && currentLatency-candidateLatency >= policy.LatencyImprovement {
		return true, fmt.Sprintf("latency improved by %s", currentLatency-candidateLatency)
	}

	return false, "current node acceptable"
}

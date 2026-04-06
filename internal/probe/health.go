package probe

import (
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// UpdateHealth folds a probe result into the rolling health state.
func UpdateHealth(previous domain.NodeHealth, success bool, latency time.Duration, checkedAt time.Time, failureReason string, failureThreshold int) domain.NodeHealth {
	updated := previous
	updated.LastCheckedAt = checkedAt
	updated.LastLatency = domain.NewDuration(latency)
	if failureThreshold < 1 {
		failureThreshold = 1
	}

	if success {
		updated.Healthy = true
		updated.SuccessCount++
		updated.ConsecutiveSuccesses++
		updated.ConsecutiveFailures = 0
		updated.LastFailureReason = ""
		if updated.AverageLatency.Duration() == 0 {
			updated.AverageLatency = domain.NewDuration(latency)
		} else {
			avg := updated.AverageLatency.Duration()
			updated.AverageLatency = domain.NewDuration((avg*4 + latency) / 5)
		}
		return updated
	}

	updated.Healthy = false
	updated.FailureCount++
	updated.ConsecutiveFailures++
	updated.ConsecutiveSuccesses = 0
	updated.LastFailureReason = failureReason
	if updated.AverageLatency.Duration() == 0 {
		updated.AverageLatency = domain.NewDuration(latency)
	}
	if updated.SuccessCount > 0 && updated.ConsecutiveFailures < failureThreshold {
		updated.Healthy = true
		return updated
	}

	return updated
}

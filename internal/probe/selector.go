package probe

import (
	"fmt"
	"sort"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// ScoreConfig configures node ranking.
type ScoreConfig struct {
	HealthyBonus       float64
	UnhealthyPenalty   float64
	LatencyWeight      float64
	SuccessWeight      float64
	FailureWeight      float64
	RecoveryBonus      float64
	MaxLatencyBaseline time.Duration
}

// DefaultScoreConfig returns the default scoring configuration.
func DefaultScoreConfig() ScoreConfig {
	return ScoreConfig{
		HealthyBonus:       200,
		UnhealthyPenalty:   500,
		LatencyWeight:      0.2,
		SuccessWeight:      15,
		FailureWeight:      40,
		RecoveryBonus:      10,
		MaxLatencyBaseline: 2 * time.Second,
	}
}

// CalculateScore converts health telemetry into a comparable score.
func CalculateScore(health domain.NodeHealth, cfg ScoreConfig) domain.ScoreResult {
	score := 0.0
	reason := "healthy"
	latency := health.AverageLatency.Duration()
	if latency <= 0 {
		latency = health.LastLatency.Duration()
	}
	if latency <= 0 {
		latency = cfg.MaxLatencyBaseline
	}

	if health.Healthy {
		score += cfg.HealthyBonus
		score += float64(health.ConsecutiveSuccesses) * cfg.SuccessWeight
		score += float64(health.SuccessCount-health.FailureCount) * cfg.RecoveryBonus
		score -= latency.Seconds() * 1000 * cfg.LatencyWeight
	} else {
		reason = "unhealthy"
		score -= cfg.UnhealthyPenalty
		score -= float64(health.ConsecutiveFailures) * cfg.FailureWeight
		score -= latency.Seconds() * 1000 * cfg.LatencyWeight
	}

	return domain.ScoreResult{
		NodeID:  health.NodeID,
		Healthy: health.Healthy,
		Score:   score,
		Reason:  reason,
	}
}

// SelectBestNode chooses the highest scored node.
func SelectBestNode(nodes []domain.Node, health map[string]domain.NodeHealth, cfg ScoreConfig) (domain.Node, domain.ScoreResult, error) {
	if len(nodes) == 0 {
		return domain.Node{}, domain.ScoreResult{}, fmt.Errorf("no nodes to select from")
	}

	type candidate struct {
		node   domain.Node
		result domain.ScoreResult
	}

	candidates := make([]candidate, 0, len(nodes))
	for _, node := range nodes {
		h := health[node.ID]
		h.NodeID = node.ID
		result := CalculateScore(h, cfg)
		candidates = append(candidates, candidate{node: node, result: result})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].result.Score > candidates[j].result.Score
	})

	best := candidates[0]
	best.result.Selected = true
	return best.node, best.result, nil
}

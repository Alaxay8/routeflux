package domain

import "time"

// NodeHealth stores rolling probe metrics for a node.
type NodeHealth struct {
	NodeID               string    `json:"node_id"`
	LastLatency          Duration  `json:"last_latency"`
	AverageLatency       Duration  `json:"average_latency"`
	SuccessCount         int       `json:"success_count"`
	FailureCount         int       `json:"failure_count"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	LastCheckedAt        time.Time `json:"last_checked_at"`
	Healthy              bool      `json:"healthy"`
	Score                float64   `json:"score"`
	LastFailureReason    string    `json:"last_failure_reason"`
}

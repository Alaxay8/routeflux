package domain

// ScoreResult represents the calculated node quality.
type ScoreResult struct {
	NodeID   string  `json:"node_id"`
	Healthy  bool    `json:"healthy"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
	Selected bool    `json:"selected"`
}

package probe

import (
	"context"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// Result is the outcome of a probe execution.
type Result struct {
	NodeID   string
	Latency  time.Duration
	Healthy  bool
	Checked  time.Time
	Err      error
	Score    domain.ScoreResult
	Health   domain.NodeHealth
	Selected bool
}

// Checker actively probes a node.
type Checker interface {
	Check(ctx context.Context, node domain.Node) Result
}

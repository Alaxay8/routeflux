package probe

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// TCPChecker checks node health using a TCP connect probe.
type TCPChecker struct {
	Timeout time.Duration
	Now     func() time.Time
}

// Check probes a node endpoint and measures TCP connect latency.
func (c TCPChecker) Check(ctx context.Context, node domain.Node) Result {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	now := c.Now
	if now == nil {
		now = time.Now
	}

	start := now()
	address := net.JoinHostPort(node.Address, fmt.Sprintf("%d", node.Port))
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return Result{
			NodeID:  node.ID,
			Healthy: false,
			Checked: now(),
			Err:     err,
			Latency: timeout,
		}
	}
	_ = conn.Close()

	return Result{
		NodeID:  node.ID,
		Healthy: true,
		Checked: now(),
		Latency: time.Since(start),
	}
}

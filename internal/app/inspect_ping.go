package app

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

const (
	inspectPingTimeout     = 2 * time.Second
	inspectPingParallelism = 6
)

// PingInspectNodeResult is the safe LuCI-facing shape for one TCP ping probe.
type PingInspectNodeResult struct {
	NodeID    string    `json:"node_id"`
	Healthy   bool      `json:"healthy"`
	LatencyMS float64   `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"`
}

// PingInspectResponse is the safe LuCI-facing payload for on-demand TCP ping.
type PingInspectResponse struct {
	SubscriptionID string                  `json:"subscription_id"`
	TimeoutMS      int                     `json:"timeout_ms"`
	Results        []PingInspectNodeResult `json:"results"`
}

// InspectPing runs router-side TCP ping probes for one node or all nodes in a subscription.
func (s *Service) InspectPing(ctx context.Context, subscriptionID, nodeID string) (PingInspectResponse, error) {
	sub, err := s.subscriptionByID(subscriptionID)
	if err != nil {
		return PingInspectResponse{}, err
	}

	nodes, err := inspectPingNodes(sub, nodeID)
	if err != nil {
		return PingInspectResponse{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	results := make([]PingInspectNodeResult, len(nodes))
	if len(nodes) == 0 {
		return PingInspectResponse{
			SubscriptionID: sub.ID,
			TimeoutMS:      int(inspectPingTimeout / time.Millisecond),
			Results:        results,
		}, nil
	}

	type job struct {
		index int
		node  domain.Node
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	workerCount := inspectPingParallelism
	if workerCount > len(nodes) {
		workerCount = len(nodes)
	}

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				probeResult := s.inspectPingProbe(ctx, item.node)
				results[item.index] = PingInspectNodeResult{
					NodeID:    item.node.ID,
					Healthy:   probeResult.Healthy,
					LatencyMS: durationMilliseconds(probeResult.Latency),
					CheckedAt: probeResult.Checked.UTC(),
					Error:     errString(probeResult.Err),
				}
			}
		}()
	}

	for index, node := range nodes {
		jobs <- job{index: index, node: node}
	}
	close(jobs)
	wg.Wait()

	return PingInspectResponse{
		SubscriptionID: sub.ID,
		TimeoutMS:      int(inspectPingTimeout / time.Millisecond),
		Results:        results,
	}, nil
}

func inspectPingNodes(sub domain.Subscription, nodeID string) ([]domain.Node, error) {
	if nodeID == "" {
		return append([]domain.Node(nil), sub.Nodes...), nil
	}

	node, ok := sub.NodeByID(nodeID)
	if !ok {
		return nil, fmt.Errorf("node %q not found in subscription %q", nodeID, sub.ID)
	}

	return []domain.Node{node}, nil
}

func (s *Service) inspectPingProbe(ctx context.Context, node domain.Node) probe.Result {
	probeCtx, cancel := context.WithTimeout(ctx, inspectPingTimeout)
	defer cancel()

	if s != nil && s.inspectPingCheck != nil {
		result := s.inspectPingCheck(probeCtx, node)
		if result.NodeID == "" {
			result.NodeID = node.ID
		}
		if result.Checked.IsZero() {
			result.Checked = s.currentTime().UTC()
		}
		return result
	}

	checker := probe.TCPChecker{Timeout: inspectPingTimeout}
	if s != nil && s.now != nil {
		checker.Now = s.now
	}
	return checker.Check(probeCtx, node)
}

func durationMilliseconds(value time.Duration) float64 {
	return math.Round(float64(value)/float64(time.Millisecond)*100) / 100
}

package api

import (
	"math"
	"time"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

// NodeSummary is the API-safe shape for a node in CLI and LuCI JSON responses.
type NodeSummary struct {
	ID             string `json:"id"`
	SubscriptionID string `json:"subscription_id"`
	Name           string `json:"name"`
	Remark         string `json:"remark"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`
	Transport      string `json:"transport"`
	Security       string `json:"security"`
}

// SubscriptionSummary is the API-safe shape for a stored subscription.
type SubscriptionSummary struct {
	ID            string          `json:"id"`
	ProviderName  string          `json:"provider_name"`
	DisplayName   string          `json:"display_name"`
	SourceType    string          `json:"source_type"`
	LastUpdatedAt string          `json:"last_updated_at"`
	ExpiresAt     string          `json:"expires_at"`
	Traffic       *TrafficSummary `json:"traffic,omitempty"`
	ParserStatus  string          `json:"parser_status"`
	LastError     string          `json:"last_error"`
	NodeCount     int             `json:"node_count"`
	RefreshEvery  string          `json:"refresh_every"`
	Nodes         []NodeSummary   `json:"nodes,omitempty"`
}

// TrafficSummary is the API-safe shape for subscription quota counters.
type TrafficSummary struct {
	UploadBytes    int64 `json:"upload_bytes"`
	DownloadBytes  int64 `json:"download_bytes"`
	UsedBytes      int64 `json:"used_bytes"`
	RemainingBytes int64 `json:"remaining_bytes"`
	TotalBytes     int64 `json:"total_bytes"`
	Unlimited      bool  `json:"unlimited"`
}

// StatusResponse is the API-safe shape for current runtime status.
type StatusResponse struct {
	State              domain.RuntimeState  `json:"state"`
	Settings           domain.Settings      `json:"settings"`
	ActiveSubscription *SubscriptionSummary `json:"active_subscription,omitempty"`
	ActiveNode         *NodeSummary         `json:"active_node,omitempty"`
}

// NodeSummaryFromDomain converts a runtime node to its safe public shape.
func NodeSummaryFromDomain(node domain.Node) NodeSummary {
	return NodeSummary{
		ID:             node.ID,
		SubscriptionID: node.SubscriptionID,
		Name:           node.Name,
		Remark:         node.Remark,
		Address:        node.Address,
		Port:           node.Port,
		Protocol:       string(node.Protocol),
		Transport:      node.Transport,
		Security:       node.Security,
	}
}

// NodeSummariesFromDomain converts runtime nodes to safe public shapes.
func NodeSummariesFromDomain(nodes []domain.Node) []NodeSummary {
	if len(nodes) == 0 {
		return nil
	}

	out := make([]NodeSummary, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, NodeSummaryFromDomain(node))
	}

	return out
}

// SubscriptionSummaryFromDomain converts a subscription to its safe public shape.
func SubscriptionSummaryFromDomain(sub domain.Subscription, includeNodes bool) SubscriptionSummary {
	result := SubscriptionSummary{
		ID:            sub.ID,
		ProviderName:  sub.ProviderName,
		DisplayName:   sub.DisplayName,
		SourceType:    string(sub.SourceType),
		LastUpdatedAt: formatTimestamp(sub.LastUpdatedAt),
		ExpiresAt:     formatTimestampPointer(sub.ExpiresAt),
		Traffic:       trafficSummaryFromDomain(sub.Traffic),
		ParserStatus:  sub.ParserStatus,
		LastError:     sub.LastError,
		NodeCount:     len(sub.Nodes),
		RefreshEvery:  sub.RefreshInterval.String(),
	}

	if includeNodes {
		result.Nodes = NodeSummariesFromDomain(sub.Nodes)
	}

	return result
}

// SubscriptionSummariesFromDomain converts subscriptions to safe public shapes.
func SubscriptionSummariesFromDomain(subscriptions []domain.Subscription, includeNodes bool) []SubscriptionSummary {
	if len(subscriptions) == 0 {
		return nil
	}

	out := make([]SubscriptionSummary, 0, len(subscriptions))
	for _, sub := range subscriptions {
		out = append(out, SubscriptionSummaryFromDomain(sub, includeNodes))
	}

	return out
}

// StatusResponseFromSnapshot converts runtime status to its safe public shape.
func StatusResponseFromSnapshot(snapshot app.StatusSnapshot) StatusResponse {
	result := StatusResponse{
		State:    snapshot.State,
		Settings: snapshot.Settings,
	}

	if snapshot.ActiveSubscription != nil {
		sub := SubscriptionSummaryFromDomain(*snapshot.ActiveSubscription, false)
		result.ActiveSubscription = &sub
	}
	if snapshot.ActiveNode != nil {
		node := NodeSummaryFromDomain(*snapshot.ActiveNode)
		result.ActiveNode = &node
	}

	return result
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format(time.RFC3339)
}

func formatTimestampPointer(value *time.Time) string {
	if value == nil {
		return ""
	}

	return formatTimestamp(*value)
}

func trafficSummaryFromDomain(value *domain.SubscriptionTraffic) *TrafficSummary {
	if value == nil {
		return nil
	}

	usedBytes := safeSumBytes(value.UploadBytes, value.DownloadBytes)
	remainingBytes := int64(0)
	if value.TotalBytes > usedBytes {
		remainingBytes = value.TotalBytes - usedBytes
	}

	return &TrafficSummary{
		UploadBytes:    value.UploadBytes,
		DownloadBytes:  value.DownloadBytes,
		UsedBytes:      usedBytes,
		RemainingBytes: remainingBytes,
		TotalBytes:     value.TotalBytes,
		Unlimited:      value.TotalBytes == 0,
	}
}

func safeSumBytes(left, right int64) int64 {
	if right > 0 && left > math.MaxInt64-right {
		return math.MaxInt64
	}

	return left + right
}

package domain

import "time"

// ActiveConnection describes the currently applied runtime selection.
type ActiveConnection struct {
	SubscriptionID string        `json:"subscription_id"`
	NodeID         string        `json:"node_id"`
	ConnectedAt    time.Time     `json:"connected_at"`
	Mode           SelectionMode `json:"mode"`
}

// RuntimeState persists the operational state across restarts.
type RuntimeState struct {
	SchemaVersion        int                   `json:"schema_version"`
	ActiveSubscriptionID string                `json:"active_subscription_id"`
	ActiveNodeID         string                `json:"active_node_id"`
	Mode                 SelectionMode         `json:"mode"`
	Connected            bool                  `json:"connected"`
	LastRefreshAt        map[string]time.Time  `json:"last_refresh_at"`
	Health               map[string]NodeHealth `json:"health"`
	LastSwitchAt         time.Time             `json:"last_switch_at"`
	LastSuccessAt        time.Time             `json:"last_success_at"`
	LastFailureReason    string                `json:"last_failure_reason"`
}

// DefaultRuntimeState returns an empty persisted state.
func DefaultRuntimeState() RuntimeState {
	return RuntimeState{
		SchemaVersion: 1,
		Mode:          SelectionModeDisconnected,
		LastRefreshAt: make(map[string]time.Time),
		Health:        make(map[string]NodeHealth),
	}
}

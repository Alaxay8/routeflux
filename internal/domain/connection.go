package domain

import "time"

// ActiveConnection describes the currently applied runtime selection.
type ActiveConnection struct {
	SubscriptionID string        `json:"subscription_id"`
	NodeID         string        `json:"node_id"`
	ConnectedAt    time.Time     `json:"connected_at"`
	Mode           SelectionMode `json:"mode"`
}

// ZapretTestRestoreState keeps the runtime selection that should be restored
// when a manual Zapret test ends.
type ZapretTestRestoreState struct {
	ActiveSubscriptionID string        `json:"active_subscription_id,omitempty"`
	ActiveNodeID         string        `json:"active_node_id,omitempty"`
	Mode                 SelectionMode `json:"mode,omitempty"`
	Connected            bool          `json:"connected"`
	ActiveTransport      TransportMode `json:"active_transport,omitempty"`
}

// ZapretTestState persists whether RouteFlux is in a user-forced Zapret test
// mode and which runtime selection should be restored afterwards.
type ZapretTestState struct {
	Active  bool                   `json:"active"`
	Restore ZapretTestRestoreState `json:"restore,omitempty"`
}

// RuntimeState persists the operational state across restarts.
type RuntimeState struct {
	SchemaVersion              int                   `json:"schema_version"`
	ActiveSubscriptionID       string                `json:"active_subscription_id"`
	ActiveNodeID               string                `json:"active_node_id"`
	Mode                       SelectionMode         `json:"mode"`
	Connected                  bool                  `json:"connected"`
	ActiveTransport            TransportMode         `json:"active_transport"`
	LastRefreshAt              map[string]time.Time  `json:"last_refresh_at"`
	Health                     map[string]NodeHealth `json:"health"`
	LastSwitchAt               time.Time             `json:"last_switch_at"`
	LastTransportSwitchAt      time.Time             `json:"last_transport_switch_at"`
	LastSuccessAt              time.Time             `json:"last_success_at"`
	LastFailureReason          string                `json:"last_failure_reason"`
	LastTransportFailureReason string                `json:"last_transport_failure_reason"`
	ZapretTest                 ZapretTestState       `json:"zapret_test,omitempty"`
}

// DefaultRuntimeState returns an empty persisted state.
func DefaultRuntimeState() RuntimeState {
	return RuntimeState{
		SchemaVersion:   2,
		Mode:            SelectionModeDisconnected,
		ActiveTransport: TransportModeDirect,
		LastRefreshAt:   make(map[string]time.Time),
		Health:          make(map[string]NodeHealth),
	}
}

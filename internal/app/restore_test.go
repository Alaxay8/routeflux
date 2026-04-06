package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestRestoreRuntimeReappliesPersistedConnection(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			SchemaVersion:        1,
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         "node-1",
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
		},
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Germany",
						Protocol: domain.ProtocolVLESS,
						Address:  "203.0.113.10",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.Mode = domain.FirewallModeTargets
	store.settings.Firewall.Targets = domain.FirewallSelectorSet{CIDRs: []string{"1.1.1.1"}}

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	if err := service.RestoreRuntime(context.Background()); err != nil {
		t.Fatalf("restore runtime: %v", err)
	}
	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply, got %d", len(runtimeBackend.requests))
	}
	if len(firewall.applied) != 1 {
		t.Fatalf("expected firewall apply, got %d", len(firewall.applied))
	}
	if !store.state.Connected {
		t.Fatal("expected state to remain connected")
	}
	if store.state.LastFailureReason != "" {
		t.Fatalf("unexpected failure reason: %q", store.state.LastFailureReason)
	}
}

func TestRestoreRuntimeFailureDisconnectsState(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.AutoMode = true
			settings.Mode = domain.SelectionModeAuto
			return settings
		}(),
		state: domain.RuntimeState{
			SchemaVersion:        1,
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         "node-1",
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
		},
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Germany",
						Protocol: domain.ProtocolVLESS,
						Address:  "203.0.113.10",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.Mode = domain.FirewallModeHosts
	store.settings.Firewall.Hosts = []string{"192.168.1.150"}

	runtimeBackend := &recordingBackend{
		status: backend.RuntimeStatus{
			Running:      false,
			ServiceState: "unknown",
		},
	}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})
	service.backendReadyChecks = 2
	service.backendReadyDelay = 0

	err := service.RestoreRuntime(context.Background())
	if err == nil {
		t.Fatal("expected restore failure")
	}
	if !strings.Contains(err.Error(), "backend is not running") {
		t.Fatalf("unexpected error: %v", err)
	}
	if firewall.disableCalls == 0 {
		t.Fatal("expected firewall to be disabled on restore failure")
	}
	if store.state.Connected {
		t.Fatal("expected state to be disconnected")
	}
	if store.state.Mode != domain.SelectionModeAuto {
		t.Fatalf("expected mode to stay auto, got %s", store.state.Mode)
	}
	if store.state.ActiveSubscriptionID != "sub-1" || store.state.ActiveNodeID != "node-1" {
		t.Fatalf("expected active selection to be preserved, got %+v", store.state)
	}
	if !strings.Contains(store.state.LastFailureReason, "restore runtime: backend is not running") {
		t.Fatalf("unexpected failure reason: %q", store.state.LastFailureReason)
	}
	if !store.settings.AutoMode {
		t.Fatal("expected auto mode setting to be preserved")
	}
	if store.settings.Mode != domain.SelectionModeAuto {
		t.Fatalf("expected settings mode auto, got %s", store.settings.Mode)
	}
}

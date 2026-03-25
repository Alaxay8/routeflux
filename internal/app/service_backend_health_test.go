package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestConnectManualDisablesFirewallWhenBackendIsNotRunning(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Germany",
						Protocol: domain.ProtocolVLESS,
						Address:  "de.example.com",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}
	store.settings.Firewall.TransparentPort = 12345

	runtimeBackend := &recordingBackend{
		status: backend.RuntimeStatus{
			Running:      false,
			ServiceState: "active with no instances",
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

	err := service.ConnectManual(context.Background(), "sub-1", "node-1")
	if err == nil {
		t.Fatal("expected connect to fail when backend is not running")
	}
	if !strings.Contains(err.Error(), "backend is not running") {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply, got %d", len(runtimeBackend.requests))
	}
	if len(firewall.applied) != 0 {
		t.Fatalf("expected no firewall apply on backend failure, got %d", len(firewall.applied))
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected firewall disable once, got %d", firewall.disableCalls)
	}
	if store.state.Connected {
		t.Fatal("expected runtime state to be disconnected")
	}
	if store.state.ActiveSubscriptionID != "sub-1" || store.state.ActiveNodeID != "node-1" {
		t.Fatalf("expected failed selection to be preserved in state, got %+v", store.state)
	}
	if store.state.Mode != domain.SelectionModeManual {
		t.Fatalf("expected mode to stay manual, got %s", store.state.Mode)
	}
	if !strings.Contains(store.state.LastFailureReason, "backend is not running") {
		t.Fatalf("unexpected last failure reason: %q", store.state.LastFailureReason)
	}
}

func TestConnectManualWaitsForBackendToReachRunningState(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Germany",
						Protocol: domain.ProtocolVLESS,
						Address:  "de.example.com",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}

	runtimeBackend := &recordingBackend{
		statuses: []backend.RuntimeStatus{
			{
				Running:      false,
				ServiceState: "active with no instances",
			},
			{
				Running:      true,
				ServiceState: "running",
			},
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

	if err := service.ConnectManual(context.Background(), "sub-1", "node-1"); err != nil {
		t.Fatalf("connect manual: %v", err)
	}

	if runtimeBackend.statusCalls != 2 {
		t.Fatalf("expected two backend status checks, got %d", runtimeBackend.statusCalls)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime state to be connected")
	}
	if store.state.LastFailureReason != "" {
		t.Fatalf("unexpected failure reason: %q", store.state.LastFailureReason)
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected firewall disable once for disabled firewall settings, got %d", firewall.disableCalls)
	}
}

func TestDisconnectDisablesFirewall(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         "node-1",
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
		},
	}
	store.settings.AutoMode = true
	store.settings.Mode = domain.SelectionModeAuto

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	if err := service.Disconnect(context.Background()); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	if runtimeBackend.stopCalls != 1 {
		t.Fatalf("expected backend stop once, got %d", runtimeBackend.stopCalls)
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected firewall disable once, got %d", firewall.disableCalls)
	}
	if store.state.Connected {
		t.Fatal("expected runtime state to be disconnected")
	}
	if store.state.ActiveSubscriptionID != "" || store.state.ActiveNodeID != "" {
		t.Fatalf("expected active selection to be cleared, got %+v", store.state)
	}
	if store.state.Mode != domain.SelectionModeDisconnected {
		t.Fatalf("expected disconnected mode, got %s", store.state.Mode)
	}
	if store.settings.AutoMode {
		t.Fatal("expected auto mode to be disabled in settings")
	}
	if store.settings.Mode != domain.SelectionModeDisconnected {
		t.Fatalf("expected settings mode disconnected, got %s", store.settings.Mode)
	}
}

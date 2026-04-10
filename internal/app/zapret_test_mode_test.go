package app

import (
	"context"
	"slices"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestStartZapretTestActivatesManagedZapretAndStoresRestoreState(t *testing.T) {
	t.Parallel()

	sub := domain.Subscription{
		ID: "sub-1",
		Nodes: []domain.Node{
			{
				ID:             "node-1",
				SubscriptionID: "sub-1",
				Name:           "Germany",
				Protocol:       domain.ProtocolVLESS,
				Address:        "203.0.113.10",
				Port:           443,
				UUID:           "11111111-1111-1111-1111-111111111111",
			},
		},
	}

	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.AutoMode = true
			settings.Mode = domain.SelectionModeAuto
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"telegram"}}
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         "node-1",
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			ActiveTransport:      domain.TransportModeProxy,
		},
	}

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	zapret := &recordingZapretManager{
		applyStatus: domain.ZapretStatus{
			Installed:    true,
			Managed:      true,
			Active:       true,
			ServiceState: "running",
		},
	}

	service := NewService(Dependencies{
		Store:         store,
		Backend:       runtimeBackend,
		Firewaller:    firewall,
		ZapretManager: zapret,
	})

	status, err := service.StartZapretTest(context.Background())
	if err != nil {
		t.Fatalf("start zapret test: %v", err)
	}

	if !status.TestActive {
		t.Fatal("expected zapret test status to be active")
	}
	if runtimeBackend.stopCalls != 1 {
		t.Fatalf("expected backend stop once, got %d", runtimeBackend.stopCalls)
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected firewall disable once, got %d", firewall.disableCalls)
	}
	if len(zapret.applyDomains) != 1 || !slices.Contains(zapret.applyDomains[0], "telegram.org") {
		t.Fatalf("expected telegram domain expansion, got %+v", zapret.applyDomains)
	}
	if len(zapret.applyCIDRs) != 1 || len(zapret.applyCIDRs[0]) != 0 {
		t.Fatalf("expected zapret test mode to stay domain-only, got %+v", zapret.applyCIDRs)
	}
	if !store.state.ZapretTest.Active {
		t.Fatal("expected persisted zapret test state to be active")
	}
	if store.state.ZapretTest.Restore.ActiveTransport != domain.TransportModeProxy {
		t.Fatalf("expected restore transport proxy, got %s", store.state.ZapretTest.Restore.ActiveTransport)
	}
	if store.state.ZapretTest.Restore.ActiveSubscriptionID != "sub-1" || store.state.ZapretTest.Restore.ActiveNodeID != "node-1" {
		t.Fatalf("unexpected restore selection: %+v", store.state.ZapretTest.Restore)
	}
	if store.state.ActiveTransport != domain.TransportModeZapret {
		t.Fatalf("expected active transport zapret, got %s", store.state.ActiveTransport)
	}
	if !store.state.Connected {
		t.Fatal("expected zapret test to keep runtime connected")
	}
}

func TestStopZapretTestRestoresPreviousProxySelection(t *testing.T) {
	t.Parallel()

	sub := domain.Subscription{
		ID: "sub-1",
		Nodes: []domain.Node{
			{
				ID:             "node-1",
				SubscriptionID: "sub-1",
				Name:           "Germany",
				Protocol:       domain.ProtocolVLESS,
				Address:        "203.0.113.10",
				Port:           443,
				UUID:           "11111111-1111-1111-1111-111111111111",
			},
		},
	}

	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.AutoMode = true
			settings.Mode = domain.SelectionModeAuto
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"youtube"}}
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         "node-1",
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			ActiveTransport:      domain.TransportModeZapret,
			ZapretTest: domain.ZapretTestState{
				Active: true,
				Restore: domain.ZapretTestRestoreState{
					ActiveSubscriptionID: "sub-1",
					ActiveNodeID:         "node-1",
					Mode:                 domain.SelectionModeAuto,
					Connected:            true,
					ActiveTransport:      domain.TransportModeProxy,
				},
			},
		},
	}

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	zapret := &recordingZapretManager{
		status: domain.ZapretStatus{
			Installed:    true,
			Managed:      false,
			Active:       false,
			ServiceState: "inactive",
		},
	}

	service := NewService(Dependencies{
		Store:         store,
		Backend:       runtimeBackend,
		Firewaller:    firewall,
		ZapretManager: zapret,
	})

	status, err := service.StopZapretTest(context.Background())
	if err != nil {
		t.Fatalf("stop zapret test: %v", err)
	}

	if status.TestActive {
		t.Fatal("expected zapret test status to be inactive")
	}
	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply during restore, got %d", len(runtimeBackend.requests))
	}
	if zapret.disableCalls != 1 {
		t.Fatalf("expected zapret disable once, got %d", zapret.disableCalls)
	}
	if store.state.ZapretTest.Active {
		t.Fatal("expected persisted zapret test state to be cleared")
	}
	if store.state.ActiveTransport != domain.TransportModeProxy {
		t.Fatalf("expected active transport proxy, got %s", store.state.ActiveTransport)
	}
	if store.state.Mode != domain.SelectionModeAuto {
		t.Fatalf("expected restored mode auto, got %s", store.state.Mode)
	}
	if !store.state.Connected {
		t.Fatal("expected restored runtime to be connected")
	}
}

func TestRunAutoHealthCheckSkipsWhileZapretTestIsActive(t *testing.T) {
	t.Parallel()

	currentNode, _, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.AutoMode = true
			settings.Mode = domain.SelectionModeAuto
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"youtube"}}
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			ActiveTransport:      domain.TransportModeZapret,
			ZapretTest: domain.ZapretTestState{
				Active: true,
				Restore: domain.ZapretTestRestoreState{
					ActiveSubscriptionID: sub.ID,
					ActiveNodeID:         currentNode.ID,
					Mode:                 domain.SelectionModeAuto,
					Connected:            true,
					ActiveTransport:      domain.TransportModeProxy,
				},
			},
			Health: map[string]domain.NodeHealth{},
		},
	}

	zapret := &recordingZapretManager{}
	service := NewService(Dependencies{
		Store:         store,
		ZapretManager: zapret,
		Checker:       fakeChecker{results: map[string]probe.Result{}},
	})

	if err := service.RunAutoHealthCheck(context.Background()); err != nil {
		t.Fatalf("run auto health check: %v", err)
	}

	if zapret.disableCalls != 0 {
		t.Fatalf("expected zapret to stay untouched during test mode, got %d disables", zapret.disableCalls)
	}
	if len(zapret.applyDomains) != 0 {
		t.Fatalf("expected no zapret reapply during test mode, got %+v", zapret.applyDomains)
	}
	if store.saveStateCalls != 0 {
		t.Fatalf("expected no state writes during test mode, got %d", store.saveStateCalls)
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected active node to stay unchanged, got %q", store.state.ActiveNodeID)
	}
}

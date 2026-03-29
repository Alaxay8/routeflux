package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestConnectAutoUsesConfiguredSwitchCooldown(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.SwitchCooldown = domain.NewDuration(10 * time.Minute)
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-6 * time.Minute),
			Health:               map[string]domain.NodeHealth{},
		},
	}

	service := NewService(Dependencies{
		Store: store,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 180 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 20 * time.Millisecond},
		}},
	})

	selected, err := service.ConnectAuto(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("connect auto: %v", err)
	}

	if selected.ID != currentNode.ID {
		t.Fatalf("expected current node to remain selected during configured cooldown, got %s", selected.ID)
	}
}

func TestSchedulerRunHealthOnceSkipsSwitchWhenImprovementBelowThreshold(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.LatencyThreshold = domain.NewDuration(60 * time.Millisecond)
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-time.Hour),
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 120 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 70 * time.Millisecond},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 0 {
		t.Fatalf("expected no backend reapply, got %d requests", len(backend.requests))
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected current node to remain active, got %s", store.state.ActiveNodeID)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime to stay connected")
	}
	if store.state.Health[currentNode.ID].NodeID != currentNode.ID {
		t.Fatalf("expected current node health to be stored, got %+v", store.state.Health[currentNode.ID])
	}
}

func TestSchedulerRunHealthOnceBuffersRepeatedHealthyStateWrites(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs:     []domain.Subscription{sub},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-time.Hour),
			Health:               map[string]domain.NodeHealth{},
		},
	}

	service := NewService(Dependencies{
		Store: store,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 120 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 85 * time.Millisecond},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())
	scheduler.RunHealthOnce(context.Background())

	if store.saveStateCalls != 1 {
		t.Fatalf("expected one persisted healthy-state write, got %d", store.saveStateCalls)
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected current node to remain active, got %s", store.state.ActiveNodeID)
	}
}

func TestSchedulerRunHealthOnceSwitchesImmediatelyWhenCurrentNodeIsUnhealthy(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.SwitchCooldown = domain.NewDuration(30 * time.Minute)
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-time.Minute),
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 5 * time.Second, Err: context.DeadlineExceeded},
			candidateNode.ID: {Healthy: true, Latency: 25 * time.Millisecond},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 1 {
		t.Fatalf("expected one backend reapply, got %d requests", len(backend.requests))
	}
	if store.state.ActiveNodeID != candidateNode.ID {
		t.Fatalf("expected candidate node to become active, got %s", store.state.ActiveNodeID)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime to stay connected after switch")
	}
}

func TestSchedulerRunHealthOnceHonorsConfiguredCooldown(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.SwitchCooldown = domain.NewDuration(10 * time.Minute)
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-6 * time.Minute),
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 200 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 20 * time.Millisecond},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 0 {
		t.Fatalf("expected cooldown to prevent backend reapply, got %d requests", len(backend.requests))
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected current node to remain active during cooldown, got %s", store.state.ActiveNodeID)
	}
}

func TestSchedulerRunHealthOnceRecoversDisconnectedAutoMode(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs:     []domain.Subscription{sub},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            false,
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 150 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 30 * time.Millisecond},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 1 {
		t.Fatalf("expected one backend reapply for recovery, got %d requests", len(backend.requests))
	}
	if !store.state.Connected {
		t.Fatal("expected auto mode recovery to reconnect runtime")
	}
	if store.state.ActiveNodeID != candidateNode.ID {
		t.Fatalf("expected recovery to pick the best healthy node, got %s", store.state.ActiveNodeID)
	}
}

func TestSchedulerRunHealthOnceMarksFailureWithoutRepeatedReapply(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs:     []domain.Subscription{sub},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    backend,
		Firewaller: firewall,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 5 * time.Second, Err: context.DeadlineExceeded},
			candidateNode.ID: {Healthy: false, Latency: 5 * time.Second, Err: context.DeadlineExceeded},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 0 {
		t.Fatalf("expected no backend reapply when all nodes are unhealthy, got %d requests", len(backend.requests))
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected runtime failure to disable firewall once, got %d", firewall.disableCalls)
	}
	if store.state.Connected {
		t.Fatal("expected runtime to be marked disconnected")
	}
	if !strings.Contains(store.state.LastFailureReason, "no healthy") {
		t.Fatalf("unexpected failure reason: %q", store.state.LastFailureReason)
	}
	if store.state.ActiveSubscriptionID != sub.ID || store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected auto context to be preserved, got %+v", store.state)
	}
}

func testAutoSubscription() (domain.Node, domain.Node, domain.Subscription) {
	currentNode := domain.Node{
		ID:             "node-current",
		SubscriptionID: "sub-1",
		Name:           "Current",
		Protocol:       domain.ProtocolVLESS,
		Address:        "current.example.com",
		Port:           443,
		UUID:           "11111111-1111-1111-1111-111111111111",
	}
	candidateNode := domain.Node{
		ID:             "node-candidate",
		SubscriptionID: "sub-1",
		Name:           "Candidate",
		Protocol:       domain.ProtocolVLESS,
		Address:        "candidate.example.com",
		Port:           443,
		UUID:           "22222222-2222-2222-2222-222222222222",
	}

	return currentNode, candidateNode, domain.Subscription{
		ID:          "sub-1",
		Nodes:       []domain.Node{currentNode, candidateNode},
		Source:      "raw",
		DisplayName: "Auto Demo",
	}
}

package app

import (
	"context"
	"errors"
	"slices"
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

func TestConnectAutoPrefersLowerLatencyOverInflatedHistory(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs:     []domain.Subscription{sub},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			Mode:      domain.SelectionModeDisconnected,
			Connected: false,
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:               currentNode.ID,
					Healthy:              true,
					SuccessCount:         9000,
					ConsecutiveSuccesses: 9000,
					AverageLatency:       domain.NewDuration(95 * time.Millisecond),
				},
				candidateNode.ID: {
					NodeID:               candidateNode.ID,
					Healthy:              true,
					SuccessCount:         20,
					ConsecutiveSuccesses: 20,
					AverageLatency:       domain.NewDuration(25 * time.Millisecond),
				},
			},
		},
	}

	service := NewService(Dependencies{
		Store: store,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 95 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 25 * time.Millisecond},
		}},
	})

	selected, err := service.ConnectAuto(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("connect auto: %v", err)
	}

	if selected.ID != candidateNode.ID {
		t.Fatalf("expected lower-latency candidate to win over inflated history, got %s", selected.ID)
	}
}

func TestConnectAutoActivatesZapretWhenNoHealthyNodesRemain(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"youtube"}}
			return settings
		}(),
		state: domain.RuntimeState{
			Mode:            domain.SelectionModeDisconnected,
			Connected:       false,
			ActiveTransport: domain.TransportModeDirect,
			Health:          map[string]domain.NodeHealth{},
		},
	}

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
		ZapretManager: zapret,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 250 * time.Millisecond, Err: errors.New("dial timeout")},
			candidateNode.ID: {Healthy: false, Latency: 250 * time.Millisecond, Err: errors.New("dial timeout")},
		}},
	})

	selected, err := service.ConnectAuto(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("connect auto: %v", err)
	}

	if selected.ID != "" {
		t.Fatalf("expected zapret activation to return no proxy node, got %s", selected.ID)
	}
	if !store.state.Connected {
		t.Fatal("expected zapret fallback to keep runtime connected")
	}
	if store.state.ActiveTransport != domain.TransportModeZapret {
		t.Fatalf("expected active transport zapret, got %s", store.state.ActiveTransport)
	}
	if !strings.Contains(store.state.LastFailureReason, "no healthy") {
		t.Fatalf("expected zapret activation to keep failure context, got %q", store.state.LastFailureReason)
	}
	if len(zapret.applyDomains) != 1 || !slices.Contains(zapret.applyDomains[0], "youtube.com") {
		t.Fatalf("expected zapret manager to receive expanded youtube domains, got %+v", zapret.applyDomains)
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

func TestSchedulerRunHealthOnceKeepsCurrentNodeOnTransientFailure(t *testing.T) {
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
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:               currentNode.ID,
					Healthy:              true,
					SuccessCount:         3,
					ConsecutiveSuccesses: 3,
					AverageLatency:       domain.NewDuration(90 * time.Millisecond),
				},
			},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 5 * time.Second, Err: context.DeadlineExceeded},
			candidateNode.ID: {Healthy: false, Latency: 5 * time.Second, Err: context.DeadlineExceeded},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 0 {
		t.Fatalf("expected no backend reapply on transient failure, got %d requests", len(backend.requests))
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected current node to remain active, got %s", store.state.ActiveNodeID)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime to stay connected on transient failure")
	}
}

func TestSchedulerRunHealthOnceSwitchesAfterFailureThresholdBreach(t *testing.T) {
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
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:              currentNode.ID,
					Healthy:             true,
					SuccessCount:        3,
					FailureCount:        2,
					ConsecutiveFailures: probe.DefaultSwitchPolicy().FailureThreshold - 1,
					AverageLatency:      domain.NewDuration(90 * time.Millisecond),
				},
			},
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

func TestSchedulerRunHealthOnceKeepsCurrentNodeWhenCandidateVerifyFails(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.Firewall.Enabled = true
			settings.Firewall.Mode = domain.FirewallModeHosts
			settings.Firewall.Hosts = []string{"192.168.1.150"}
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			LastSwitchAt:         time.Now().Add(-time.Minute),
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:              currentNode.ID,
					Healthy:             true,
					SuccessCount:        3,
					FailureCount:        2,
					ConsecutiveFailures: probe.DefaultSwitchPolicy().FailureThreshold - 1,
					AverageLatency:      domain.NewDuration(90 * time.Millisecond),
				},
			},
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
			candidateNode.ID: {Healthy: true, Latency: 25 * time.Millisecond},
		}},
	})
	service.backendEgressProbe = func(context.Context) error { return errors.New("candidate probe failed") }
	service.backendEgressTimeout = 5 * time.Millisecond
	service.backendEgressRetryDelay = time.Millisecond

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 1 {
		t.Fatalf("expected one backend apply, got %d", len(backend.requests))
	}
	if backend.rollbackCalls != 1 {
		t.Fatalf("expected candidate failure to rollback runtime once, got %d", backend.rollbackCalls)
	}
	if firewall.disableCalls != 0 {
		t.Fatalf("expected firewall to stay enabled during recovered auto failure, got %d disables", firewall.disableCalls)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime to stay connected after recovered auto failure")
	}
	if store.state.ActiveNodeID != currentNode.ID {
		t.Fatalf("expected current node to remain active after rollback, got %s", store.state.ActiveNodeID)
	}
	if !strings.Contains(store.state.LastFailureReason, "candidate verify failed: backend egress probe failed") {
		t.Fatalf("unexpected failure reason: %q", store.state.LastFailureReason)
	}
}

func TestSchedulerRunHealthOnceSwitchesWhenCurrentRuntimeEgressFails(t *testing.T) {
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
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:               currentNode.ID,
					Healthy:              true,
					SuccessCount:         9000,
					ConsecutiveSuccesses: 9000,
					AverageLatency:       domain.NewDuration(20 * time.Millisecond),
				},
				candidateNode.ID: {
					NodeID:               candidateNode.ID,
					Healthy:              true,
					SuccessCount:         20,
					ConsecutiveSuccesses: 20,
					AverageLatency:       domain.NewDuration(60 * time.Millisecond),
				},
			},
		},
	}

	backend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: true, Latency: 20 * time.Millisecond},
			candidateNode.ID: {Healthy: true, Latency: 60 * time.Millisecond},
		}},
	})

	egressChecks := 0
	service.backendEgressProbe = func(context.Context) error {
		egressChecks++
		if egressChecks == 1 {
			return errors.New("current runtime lost egress")
		}
		return nil
	}
	service.backendEgressTimeout = time.Millisecond
	service.backendEgressRetryDelay = 10 * time.Millisecond

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 1 {
		t.Fatalf("expected runtime egress failure to trigger one backend reapply, got %d requests", len(backend.requests))
	}
	if store.state.ActiveNodeID != candidateNode.ID {
		t.Fatalf("expected candidate node to replace current runtime after egress failure, got %s", store.state.ActiveNodeID)
	}
	if !store.state.Connected {
		t.Fatal("expected runtime to stay connected after switching away from failed node")
	}
	currentHealth := store.state.Health[currentNode.ID]
	if currentHealth.Healthy {
		t.Fatalf("expected current node health to be penalized after runtime egress failure, got %+v", currentHealth)
	}
	if !strings.Contains(currentHealth.LastFailureReason, "current runtime lost egress") {
		t.Fatalf("unexpected current-node failure reason: %q", currentHealth.LastFailureReason)
	}
}

func TestSchedulerRunHealthOnceFailsBackFromZapretAfterStableProxyRecovery(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"youtube"}}
			settings.Zapret.FailbackSuccessThreshold = 3
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID:  sub.ID,
			ActiveNodeID:          currentNode.ID,
			Mode:                  domain.SelectionModeAuto,
			Connected:             true,
			ActiveTransport:       domain.TransportModeZapret,
			LastTransportSwitchAt: time.Now().Add(-time.Hour),
			Health: map[string]domain.NodeHealth{
				candidateNode.ID: {
					NodeID:               candidateNode.ID,
					Healthy:              true,
					SuccessCount:         3,
					ConsecutiveSuccesses: 3,
					AverageLatency:       domain.NewDuration(20 * time.Millisecond),
				},
			},
		},
	}

	backend := &recordingBackend{}
	zapret := &recordingZapretManager{
		status: domain.ZapretStatus{
			Installed:    true,
			Managed:      true,
			Active:       true,
			ServiceState: "running",
		},
	}
	service := NewService(Dependencies{
		Store:         store,
		Backend:       backend,
		ZapretManager: zapret,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 250 * time.Millisecond, Err: errors.New("timeout")},
			candidateNode.ID: {Healthy: true, Latency: 20 * time.Millisecond},
		}},
	})
	service.backendEgressProbe = func(context.Context) error { return nil }

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if len(backend.requests) != 1 {
		t.Fatalf("expected proxy backend reapply after stable recovery, got %d requests", len(backend.requests))
	}
	if zapret.disableCalls != 1 {
		t.Fatalf("expected zapret to be disabled during failback, got %d calls", zapret.disableCalls)
	}
	if store.state.ActiveTransport != domain.TransportModeProxy {
		t.Fatalf("expected transport to fail back to proxy, got %s", store.state.ActiveTransport)
	}
	if store.state.ActiveNodeID != candidateNode.ID {
		t.Fatalf("expected candidate node after failback, got %s", store.state.ActiveNodeID)
	}
}

func TestSchedulerRunHealthOnceFallsBackToDirectWhenZapretIsUnmanaged(t *testing.T) {
	t.Parallel()

	currentNode, candidateNode, sub := testAutoSubscription()
	store := &memoryStore{
		subs: []domain.Subscription{sub},
		settings: func() domain.Settings {
			settings := domain.DefaultSettings()
			settings.Zapret.Enabled = true
			settings.Zapret.Selectors = domain.FirewallSelectorSet{Services: []string{"youtube"}}
			return settings
		}(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: sub.ID,
			ActiveNodeID:         currentNode.ID,
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
			ActiveTransport:      domain.TransportModeProxy,
			Health:               map[string]domain.NodeHealth{},
		},
	}

	backend := &recordingBackend{}
	zapret := &recordingZapretManager{
		status: domain.ZapretStatus{
			Installed:    true,
			Managed:      false,
			Active:       true,
			ServiceState: "running",
			LastReason:   "external/unmanaged zapret is active",
		},
	}
	service := NewService(Dependencies{
		Store:         store,
		Backend:       backend,
		ZapretManager: zapret,
		Checker: fakeChecker{results: map[string]probe.Result{
			currentNode.ID:   {Healthy: false, Latency: 250 * time.Millisecond, Err: errors.New("timeout")},
			candidateNode.ID: {Healthy: false, Latency: 250 * time.Millisecond, Err: errors.New("timeout")},
		}},
	})

	scheduler := NewScheduler(service)
	scheduler.RunHealthOnce(context.Background())

	if store.state.Connected {
		t.Fatal("expected unmanaged zapret fallback to degrade to disconnected direct state")
	}
	if store.state.ActiveTransport != domain.TransportModeDirect {
		t.Fatalf("expected direct transport after unmanaged zapret failure, got %s", store.state.ActiveTransport)
	}
	if !strings.Contains(store.state.LastTransportFailureReason, "external/unmanaged") {
		t.Fatalf("unexpected zapret transport failure reason: %q", store.state.LastTransportFailureReason)
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

func TestSchedulerRunHealthOnceMarksFailureAfterThresholdBreach(t *testing.T) {
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
			Health: map[string]domain.NodeHealth{
				currentNode.ID: {
					NodeID:              currentNode.ID,
					Healthy:             true,
					SuccessCount:        3,
					FailureCount:        2,
					ConsecutiveFailures: probe.DefaultSwitchPolicy().FailureThreshold - 1,
					AverageLatency:      domain.NewDuration(90 * time.Millisecond),
				},
			},
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

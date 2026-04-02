package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestConnectManualLogsRuntimeEvents(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Edge",
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

	service := NewService(Dependencies{
		Store:      store,
		Backend:    &recordingBackend{},
		Firewaller: &recordingFirewaller{},
		Logger:     testLogger(&logs, slog.LevelInfo),
	})

	if err := service.ConnectManual(context.Background(), "sub-1", "node-1"); err != nil {
		t.Fatalf("connect manual: %v", err)
	}

	for _, want := range []string{
		"manual connect requested",
		"apply backend config",
		"backend running confirmed",
		"firewall rules applied",
		"manual connect succeeded",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("expected logs to contain %q, got %q", want, logs.String())
		}
	}
}

func TestConnectAutoLogsSelectionReason(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{ID: "node-1", Name: "Slow", Protocol: domain.ProtocolVLESS, Address: "slow.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111"},
					{ID: "node-2", Name: "Fast", Protocol: domain.ProtocolVLESS, Address: "fast.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222"},
				},
			},
		},
	}
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeManual
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"

	service := NewService(Dependencies{
		Store:  store,
		Logger: testLogger(&logs, slog.LevelDebug),
		Checker: fakeChecker{results: map[string]probe.Result{
			"node-1": {NodeID: "node-1", Healthy: true, Latency: 160 * time.Millisecond, Checked: time.Now().UTC()},
			"node-2": {NodeID: "node-2", Healthy: true, Latency: 20 * time.Millisecond, Checked: time.Now().UTC()},
		}},
	})

	if _, err := service.ConnectAuto(context.Background(), "sub-1"); err != nil {
		t.Fatalf("connect auto: %v", err)
	}

	for _, want := range []string{
		"probe result",
		"auto selection decision",
		"latency improved by",
		"auto connect succeeded",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("expected logs to contain %q, got %q", want, logs.String())
		}
	}
}

func TestRestoreRuntimeLogsFailure(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
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
						Name:     "Edge",
						Protocol: domain.ProtocolVLESS,
						Address:  "203.0.113.10",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}

	service := NewService(Dependencies{
		Store: store,
		Backend: &recordingBackend{
			status: backend.RuntimeStatus{
				Running:      false,
				ServiceState: "stopped",
			},
		},
		Firewaller: &recordingFirewaller{},
		Logger:     testLogger(&logs, slog.LevelInfo),
	})

	if err := service.RestoreRuntime(context.Background()); err == nil {
		t.Fatal("expected restore runtime to fail")
	}

	for _, want := range []string{
		"restore runtime start",
		"restore runtime failed",
		"restore failure persisted",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("expected logs to contain %q, got %q", want, logs.String())
		}
	}
}

func testLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}))
}

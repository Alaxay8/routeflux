package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/backend/xray"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/speedtest"
)

func TestInspectXrayConfigUsesOriginalAddressAndCurrentSettings(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		subs: []domain.Subscription{
			{
				ID:                 "sub-1",
				SourceType:         domain.SourceTypeRaw,
				Source:             "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443?encryption=none&security=reality&sni=cdn.example.com&fp=chrome&pbk=pub&sid=ab12&type=ws&path=%2Fproxy&host=cdn.example.com#Edge",
				ProviderName:       "Demo VPN",
				DisplayName:        "Demo VPN",
				ProviderNameSource: domain.ProviderNameSourceManual,
				LastUpdatedAt:      time.Now().UTC(),
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Netherlands",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
						Encryption:     "none",
						Security:       "reality",
						ServerName:     "cdn.example.com",
						Fingerprint:    "chrome",
						PublicKey:      "pub",
						ShortID:        "ab12",
						Transport:      "ws",
						Path:           "/proxy",
						Host:           "cdn.example.com",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.LogLevel = "debug"
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.Mode = domain.FirewallModeTargets
	store.settings.Firewall.Targets = domain.FirewallSelectorSet{CIDRs: []string{"1.1.1.1/32"}}
	store.settings.Firewall.TransparentPort = 23456

	service := NewService(Dependencies{
		Store:   store,
		Backend: xray.NewRuntimeBackend(filepath.Join(t.TempDir(), "xray-config.json"), nil),
	})

	rendered, err := service.InspectXrayConfig("sub-1", "node-1")
	if err != nil {
		t.Fatalf("inspect xray config: %v", err)
	}

	text := string(rendered)
	if !strings.Contains(text, `"address": "edge.example.com"`) {
		t.Fatalf("expected original node address in config, got %s", text)
	}
	if !strings.Contains(text, `"loglevel": "debug"`) {
		t.Fatalf("expected current log level in config, got %s", text)
	}
	if !strings.Contains(text, `"tag": "transparent-in"`) {
		t.Fatalf("expected transparent inbound in preview, got %s", text)
	}
}

func TestInspectXrayConfigDoesNotApplyRuntimeOrMutateState(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		subs: []domain.Subscription{
			{
				ID:            "sub-1",
				DisplayName:   "Demo VPN",
				LastUpdatedAt: time.Now().UTC(),
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Netherlands",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			SchemaVersion:        1,
			ActiveSubscriptionID: "sub-active",
			ActiveNodeID:         "node-active",
			Mode:                 domain.SelectionModeAuto,
			Connected:            true,
		},
	}
	backend := &inspectBackend{generatedConfig: []byte(`{"ok":true}`)}

	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
	})

	before := store.state
	rendered, err := service.InspectXrayConfig("sub-1", "node-1")
	if err != nil {
		t.Fatalf("inspect xray config: %v", err)
	}

	if string(rendered) != `{"ok":true}` {
		t.Fatalf("unexpected preview: %s", rendered)
	}
	if backend.applyCalls != 0 {
		t.Fatalf("expected no ApplyConfig calls, got %d", backend.applyCalls)
	}
	if !reflect.DeepEqual(store.state, before) {
		t.Fatalf("expected state to stay unchanged, got %+v", store.state)
	}
}

func TestInspectSpeedUsesIsolatedConfigAndDoesNotMutateRuntime(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		subs: []domain.Subscription{
			{
				ID:            "sub-1",
				DisplayName:   "Demo VPN",
				LastUpdatedAt: time.Now().UTC(),
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Netherlands",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
						Encryption:     "none",
						Security:       "reality",
						ServerName:     "cdn.example.com",
						Fingerprint:    "chrome",
						PublicKey:      "pub",
						ShortID:        "ab12",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			SchemaVersion:        1,
			ActiveSubscriptionID: "sub-active",
			ActiveNodeID:         "node-active",
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
		},
	}
	store.settings.LogLevel = "warning"
	store.settings.Firewall.Enabled = true
	store.settings.Firewall.Mode = domain.FirewallModeHosts
	store.settings.Firewall.Hosts = []string{"192.168.1.10/32"}
	store.settings.Firewall.TransparentPort = 12345

	backend := &inspectBackend{generatedConfig: []byte(`{"inbounds":[{"tag":"http-in"}]}`)}
	firewall := &recordingFirewaller{}
	tester := &fakeSpeedTester{
		result: speedtest.Result{
			SubscriptionID: "sub-1",
			NodeID:         "node-1",
			NodeName:       "Netherlands",
			LatencyMS:      42.5,
			DownloadMbps:   88.12,
			UploadMbps:     24.33,
			DownloadBytes:  1234,
			UploadBytes:    4321,
			StartedAt:      time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC),
			FinishedAt:     time.Date(2026, 3, 26, 20, 0, 5, 0, time.UTC),
		},
	}

	service := NewService(Dependencies{
		Store:       store,
		Backend:     backend,
		Firewaller:  firewall,
		SpeedTester: tester,
	})

	before := store.state
	result, err := service.InspectSpeed(context.Background(), "sub-1", "node-1")
	if err != nil {
		t.Fatalf("inspect speed: %v", err)
	}

	if result != tester.result {
		t.Fatalf("unexpected speed test result: %+v", result)
	}
	if backend.applyCalls != 0 {
		t.Fatalf("expected no ApplyConfig calls, got %d", backend.applyCalls)
	}
	if len(firewall.applied) != 0 || firewall.disableCalls != 0 {
		t.Fatalf("expected no firewall mutations, got %+v", firewall)
	}
	if !reflect.DeepEqual(store.state, before) {
		t.Fatalf("expected state to stay unchanged, got %+v", store.state)
	}
	if len(backend.generateRequests) != 1 {
		t.Fatalf("expected one GenerateConfig call, got %d", len(backend.generateRequests))
	}
	req := backend.generateRequests[0]
	if req.TransparentProxy {
		t.Fatal("expected isolated speed test config to disable transparent proxy")
	}
	if len(req.Nodes) != 1 || req.Nodes[0].Address != "edge.example.com" {
		t.Fatalf("expected original node address in speed test request, got %+v", req.Nodes)
	}
	if tester.request.HTTPProxyPort <= 0 {
		t.Fatalf("expected assigned HTTP proxy port, got %d", tester.request.HTTPProxyPort)
	}
	if !json.Valid(tester.request.Config) {
		t.Fatalf("expected generated config JSON, got %s", tester.request.Config)
	}
	if strings.Contains(string(tester.request.Config), "transparent-in") {
		t.Fatalf("expected isolated config without transparent inbound, got %s", tester.request.Config)
	}
	if tester.ctx == nil {
		t.Fatal("expected speed tester context to be captured")
	}
	deadline, ok := tester.ctx.Deadline()
	if !ok {
		t.Fatal("expected inspect speed timeout deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > inspectSpeedTimeout {
		t.Fatalf("expected timeout within %s, got %s", inspectSpeedTimeout, remaining)
	}
}

func TestInspectSpeedProvidesEnoughTimeForRouterRun(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		subs: []domain.Subscription{
			{
				ID:            "sub-1",
				DisplayName:   "Demo VPN",
				LastUpdatedAt: time.Now().UTC(),
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Netherlands",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	tester := &fakeSpeedTester{
		minRemaining: 50 * time.Second,
		result: speedtest.Result{
			SubscriptionID: "sub-1",
			NodeID:         "node-1",
			NodeName:       "Netherlands",
		},
	}

	service := NewService(Dependencies{
		Store:       store,
		Backend:     &inspectBackend{generatedConfig: []byte(`{"ok":true}`)},
		SpeedTester: tester,
	})

	if _, err := service.InspectSpeed(context.Background(), "sub-1", "node-1"); err != nil {
		t.Fatalf("inspect speed: %v", err)
	}
}

type inspectBackend struct {
	generatedConfig  []byte
	generateErr      error
	generateRequests []backend.ConfigRequest
	applyCalls       int
}

func (b *inspectBackend) GenerateConfig(req backend.ConfigRequest) ([]byte, error) {
	b.generateRequests = append(b.generateRequests, req)
	if b.generateErr != nil {
		return nil, b.generateErr
	}
	return append([]byte(nil), b.generatedConfig...), nil
}

func (b *inspectBackend) ApplyConfig(context.Context, backend.ConfigRequest) error {
	b.applyCalls++
	return fmt.Errorf("unexpected ApplyConfig call")
}

func (b *inspectBackend) CaptureRollback() (backend.RollbackSnapshot, error) {
	return backend.RollbackSnapshot{}, nil
}

func (b *inspectBackend) RollbackConfig(context.Context, backend.RollbackSnapshot) error {
	return nil
}

func (b *inspectBackend) Start(context.Context) error  { return nil }
func (b *inspectBackend) Stop(context.Context) error   { return nil }
func (b *inspectBackend) Reload(context.Context) error { return nil }
func (b *inspectBackend) Status(context.Context) (backend.RuntimeStatus, error) {
	return backend.RuntimeStatus{}, nil
}

type fakeSpeedTester struct {
	request speedtest.Request
	result  speedtest.Result
	err     error
	ctx     context.Context

	minRemaining time.Duration
}

func (f *fakeSpeedTester) Test(ctx context.Context, req speedtest.Request) (speedtest.Result, error) {
	f.ctx = ctx
	f.request = req
	if f.minRemaining > 0 {
		deadline, ok := ctx.Deadline()
		if !ok {
			return speedtest.Result{}, fmt.Errorf("missing speed test deadline")
		}
		if remaining := time.Until(deadline); remaining < f.minRemaining {
			return speedtest.Result{}, fmt.Errorf("speed test deadline too short: %s", remaining)
		}
	}
	if f.err != nil {
		return speedtest.Result{}, f.err
	}
	return f.result, nil
}

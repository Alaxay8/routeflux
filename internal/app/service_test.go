package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func TestConfigureFirewallHostsClearsDestinationTargets(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.TargetCIDRs = []string{"1.1.1.1"}

	service := NewService(Dependencies{Store: store})

	settings, err := service.ConfigureFirewallHosts(context.Background(), []string{"192.168.1.150"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall hosts: %v", err)
	}

	if !settings.Enabled {
		t.Fatal("expected firewall to be enabled")
	}
	if settings.TransparentPort != 23456 {
		t.Fatalf("unexpected transparent port: %d", settings.TransparentPort)
	}
	if len(settings.TargetCIDRs) != 0 {
		t.Fatalf("expected destination targets to be cleared, got %v", settings.TargetCIDRs)
	}
	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "192.168.1.150" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
	if !settings.BlockQUIC {
		t.Fatal("expected QUIC blocking to be enabled for host routing")
	}
}

func TestConfigureFirewallHostsCanonicalizesAllAlias(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	service := NewService(Dependencies{Store: store})

	settings, err := service.ConfigureFirewallHosts(context.Background(), []string{"*", "192.168.1.150"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall hosts: %v", err)
	}

	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "all" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
	if len(store.settings.Firewall.SourceCIDRs) != 1 || store.settings.Firewall.SourceCIDRs[0] != "all" {
		t.Fatalf("unexpected persisted source hosts: %v", store.settings.Firewall.SourceCIDRs)
	}
}

func TestConfigureFirewallHostsPreservesBlockQUICSetting(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.BlockQUIC = false

	service := NewService(Dependencies{Store: store})

	settings, err := service.ConfigureFirewallHosts(context.Background(), []string{"192.168.1.150"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall hosts: %v", err)
	}

	if settings.BlockQUIC {
		t.Fatal("expected block-quic to remain false")
	}
}

func TestConnectManualAppliesHostFirewallRouting(t *testing.T) {
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

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	if err := service.ConnectManual(context.Background(), "sub-1", "node-1"); err != nil {
		t.Fatalf("connect manual: %v", err)
	}

	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply, got %d", len(runtimeBackend.requests))
	}
	if !runtimeBackend.requests[0].TransparentProxy {
		t.Fatal("expected transparent proxy to be enabled")
	}

	if len(firewall.applied) != 1 {
		t.Fatalf("expected firewall rules to be applied once, got %d", len(firewall.applied))
	}
	if len(firewall.applied[0].SourceCIDRs) != 1 || firewall.applied[0].SourceCIDRs[0] != "192.168.1.150" {
		t.Fatalf("unexpected applied source hosts: %v", firewall.applied[0].SourceCIDRs)
	}
}

func TestGetSettingsSyncsConnectedRuntimeMode(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.AutoMode = true
	store.settings.Mode = domain.SelectionModeAuto
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeManual

	service := NewService(Dependencies{Store: store})

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}

	if settings.AutoMode {
		t.Fatal("expected auto-mode to be false when runtime state is manual")
	}
	if settings.Mode != domain.SelectionModeManual {
		t.Fatalf("unexpected settings mode: %s", settings.Mode)
	}
	if store.settings.AutoMode {
		t.Fatal("expected persisted settings to be synced to runtime state")
	}
}

func TestSetSettingAutoModeTrueSwitchesCurrentConnection(t *testing.T) {
	t.Parallel()

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
		Store: store,
		Checker: fakeChecker{results: map[string]probe.Result{
			"node-1": {NodeID: "node-1", Healthy: true, Latency: 150 * time.Millisecond, Checked: time.Now().UTC()},
			"node-2": {NodeID: "node-2", Healthy: true, Latency: 20 * time.Millisecond, Checked: time.Now().UTC()},
		}},
	})

	settings, err := service.SetSetting("auto-mode", "true")
	if err != nil {
		t.Fatalf("set auto-mode true: %v", err)
	}

	if !settings.AutoMode || settings.Mode != domain.SelectionModeAuto {
		t.Fatalf("unexpected settings after enabling auto: %+v", settings)
	}
	if store.state.Mode != domain.SelectionModeAuto {
		t.Fatalf("expected runtime state mode auto, got %s", store.state.Mode)
	}
	if store.state.ActiveNodeID != "node-2" {
		t.Fatalf("expected best node to be selected, got %s", store.state.ActiveNodeID)
	}
}

func TestSetSettingAutoModeTrueKeepsCurrentNodeButUpdatesMode(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{ID: "node-1", Name: "Current", Protocol: domain.ProtocolVLESS, Address: "current.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111"},
					{ID: "node-2", Name: "Slightly Better", Protocol: domain.ProtocolVLESS, Address: "better.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222"},
				},
			},
		},
	}
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeManual
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"

	service := NewService(Dependencies{
		Store: store,
		Checker: fakeChecker{results: map[string]probe.Result{
			"node-1": {NodeID: "node-1", Healthy: true, Latency: 100 * time.Millisecond, Checked: time.Now().UTC()},
			"node-2": {NodeID: "node-2", Healthy: true, Latency: 70 * time.Millisecond, Checked: time.Now().UTC()},
		}},
	})

	settings, err := service.SetSetting("auto-mode", "true")
	if err != nil {
		t.Fatalf("set auto-mode true: %v", err)
	}

	if !settings.AutoMode || settings.Mode != domain.SelectionModeAuto {
		t.Fatalf("unexpected settings after enabling auto: %+v", settings)
	}
	if store.state.Mode != domain.SelectionModeAuto {
		t.Fatalf("expected runtime state mode auto, got %s", store.state.Mode)
	}
	if store.state.ActiveNodeID != "node-1" {
		t.Fatalf("expected current node to remain selected, got %s", store.state.ActiveNodeID)
	}
}

func TestSetSettingAutoModeFalsePinsCurrentNode(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{ID: "node-2", Name: "Fast", Protocol: domain.ProtocolVLESS, Address: "fast.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222"},
				},
			},
		},
	}
	store.settings.AutoMode = true
	store.settings.Mode = domain.SelectionModeAuto
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeAuto
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-2"

	service := NewService(Dependencies{Store: store})

	settings, err := service.SetSetting("auto-mode", "false")
	if err != nil {
		t.Fatalf("set auto-mode false: %v", err)
	}

	if settings.AutoMode {
		t.Fatal("expected auto-mode to be disabled")
	}
	if settings.Mode != domain.SelectionModeManual {
		t.Fatalf("expected settings mode manual, got %s", settings.Mode)
	}
	if store.state.Mode != domain.SelectionModeManual {
		t.Fatalf("expected runtime state mode manual, got %s", store.state.Mode)
	}
	if store.state.ActiveNodeID != "node-2" {
		t.Fatalf("expected current node to stay pinned, got %s", store.state.ActiveNodeID)
	}
}

func TestSetSettingDNSFields(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := NewService(Dependencies{Store: store})

	settings, err := service.SetSetting("dns.servers", "dns.google, 1.1.1.1")
	if err != nil {
		t.Fatalf("set dns.servers: %v", err)
	}
	if len(settings.DNS.Servers) != 2 || settings.DNS.Servers[0] != "dns.google" || settings.DNS.Servers[1] != "1.1.1.1" {
		t.Fatalf("unexpected dns servers: %+v", settings.DNS.Servers)
	}

	settings, err = service.SetSetting("dns.bootstrap", "9.9.9.9, 8.8.8.8")
	if err != nil {
		t.Fatalf("set dns.bootstrap: %v", err)
	}
	if len(settings.DNS.Bootstrap) != 2 || settings.DNS.Bootstrap[0] != "9.9.9.9" || settings.DNS.Bootstrap[1] != "8.8.8.8" {
		t.Fatalf("unexpected dns bootstrap: %+v", settings.DNS.Bootstrap)
	}

	settings, err = service.SetSetting("dns.domains", "domain:lan, full:router.lan")
	if err != nil {
		t.Fatalf("set dns.domains: %v", err)
	}
	if len(settings.DNS.DirectDomains) != 2 || settings.DNS.DirectDomains[0] != "domain:lan" || settings.DNS.DirectDomains[1] != "full:router.lan" {
		t.Fatalf("unexpected dns direct domains: %+v", settings.DNS.DirectDomains)
	}
}

func TestSetSettingDNSModeReappliesCurrentConnection(t *testing.T) {
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
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeManual
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"
	store.settings.DNS.Servers = []string{"dns.google"}
	store.settings.DNS.Transport = domain.DNSTransportDoH

	runtimeBackend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: runtimeBackend,
	})

	settings, err := service.SetSetting("dns.mode", string(domain.DNSModeSplit))
	if err != nil {
		t.Fatalf("set dns.mode: %v", err)
	}

	if settings.DNS.Mode != domain.DNSModeSplit {
		t.Fatalf("unexpected dns mode: %s", settings.DNS.Mode)
	}
	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend reapply, got %d", len(runtimeBackend.requests))
	}
	if runtimeBackend.requests[0].DNS.Mode != domain.DNSModeSplit {
		t.Fatalf("unexpected request dns mode: %s", runtimeBackend.requests[0].DNS.Mode)
	}
	if runtimeBackend.requests[0].DNS.Transport != domain.DNSTransportDoH {
		t.Fatalf("unexpected request dns transport: %s", runtimeBackend.requests[0].DNS.Transport)
	}
	if len(runtimeBackend.requests[0].DNS.Servers) != 1 || runtimeBackend.requests[0].DNS.Servers[0] != "dns.google" {
		t.Fatalf("unexpected request dns servers: %+v", runtimeBackend.requests[0].DNS.Servers)
	}
}

func TestApplyDefaultDNSReappliesCurrentConnection(t *testing.T) {
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
	store.settings.DNS.Mode = domain.DNSModeSystem
	store.settings.DNS.Transport = domain.DNSTransportPlain
	store.settings.DNS.Servers = nil
	store.settings.DNS.Bootstrap = []string{"9.9.9.9"}
	store.settings.DNS.DirectDomains = nil
	store.state.Connected = true
	store.state.Mode = domain.SelectionModeManual
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"

	runtimeBackend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: runtimeBackend,
	})

	settings, err := service.ApplyDefaultDNS(context.Background())
	if err != nil {
		t.Fatalf("apply default dns: %v", err)
	}

	want := domain.DefaultDNSSettings()
	if settings.DNS.Mode != want.Mode || settings.DNS.Transport != want.Transport {
		t.Fatalf("unexpected default dns: %+v", settings.DNS)
	}
	if len(settings.DNS.Servers) != len(want.Servers) || settings.DNS.Servers[0] != want.Servers[0] || settings.DNS.Servers[1] != want.Servers[1] {
		t.Fatalf("unexpected default dns servers: %+v", settings.DNS.Servers)
	}
	if len(settings.DNS.DirectDomains) != len(want.DirectDomains) {
		t.Fatalf("unexpected default direct domains: %+v", settings.DNS.DirectDomains)
	}
	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend reapply, got %d", len(runtimeBackend.requests))
	}
	if runtimeBackend.requests[0].DNS.Mode != want.Mode || runtimeBackend.requests[0].DNS.Transport != want.Transport {
		t.Fatalf("unexpected request dns: %+v", runtimeBackend.requests[0].DNS)
	}
}

func TestRefreshSubscriptionParsesJSONArrayOfConfigs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
		  {
		    "remarks": "One",
		    "outbounds": [
		      {
		        "protocol": "vless",
		        "tag": "proxy-one",
		        "settings": {
		          "vnext": [
		            {
		              "address": "one.example.com",
		              "port": 443,
		              "users": [
		                {
		                  "id": "11111111-1111-1111-1111-111111111111",
		                  "encryption": "none",
		                  "flow": "xtls-rprx-vision"
		                }
		              ]
		            }
		          ]
		        },
		        "streamSettings": {
		          "network": "tcp",
		          "security": "reality",
		          "realitySettings": {
		            "serverName": "rbc.ru",
		            "publicKey": "public-key-one",
		            "shortId": "short-one",
		            "fingerprint": "random"
		          }
		        }
		      }
		    ]
		  }
		]`))
	}))
	defer server.Close()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:           "sub-1",
				SourceType:   domain.SourceTypeURL,
				Source:       server.URL,
				ProviderName: "test-provider",
				Nodes: []domain.Node{
					{ID: "old-node"},
				},
			},
		},
	}

	service := NewService(Dependencies{Store: store, HTTPClient: server.Client()})

	sub, err := service.RefreshSubscription(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}

	if sub.ParserStatus != "ok" {
		t.Fatalf("unexpected parser status: %s", sub.ParserStatus)
	}
	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node after refresh, got %d", len(sub.Nodes))
	}
	if sub.Nodes[0].Address != "one.example.com" {
		t.Fatalf("unexpected node address: %s", sub.Nodes[0].Address)
	}
}

func TestRemoveSubscriptionDeletesInactiveSubscription(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{ID: "sub-1", DisplayName: "One"},
			{ID: "sub-2", DisplayName: "Two"},
		},
	}

	service := NewService(Dependencies{Store: store})

	if err := service.RemoveSubscription(context.Background(), "sub-1"); err != nil {
		t.Fatalf("remove subscription: %v", err)
	}

	if len(store.subs) != 1 {
		t.Fatalf("expected one subscription left, got %d", len(store.subs))
	}
	if store.subs[0].ID != "sub-2" {
		t.Fatalf("unexpected remaining subscription: %s", store.subs[0].ID)
	}
}

func TestRemoveSubscriptionDisconnectsActiveSubscription(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{ID: "sub-1", DisplayName: "One"},
		},
	}
	store.settings.AutoMode = true
	store.settings.Mode = domain.SelectionModeAuto
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"
	store.state.Mode = domain.SelectionModeAuto
	store.state.Connected = true

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	if err := service.RemoveSubscription(context.Background(), "sub-1"); err != nil {
		t.Fatalf("remove active subscription: %v", err)
	}

	if runtimeBackend.stopCalls != 1 {
		t.Fatalf("expected backend stop once, got %d", runtimeBackend.stopCalls)
	}
	if firewall.disableCalls != 1 {
		t.Fatalf("expected firewall disable once, got %d", firewall.disableCalls)
	}
	if len(store.subs) != 0 {
		t.Fatalf("expected no subscriptions left, got %d", len(store.subs))
	}
	if store.state.ActiveSubscriptionID != "" || store.state.ActiveNodeID != "" {
		t.Fatalf("expected active subscription to be cleared, got %+v", store.state)
	}
	if store.state.Connected {
		t.Fatal("expected runtime state to be disconnected")
	}
	if store.settings.AutoMode {
		t.Fatal("expected auto mode to be disabled")
	}
	if store.settings.Mode != domain.SelectionModeDisconnected {
		t.Fatalf("expected settings mode disconnected, got %s", store.settings.Mode)
	}
}

type memoryStore struct {
	subs     []domain.Subscription
	settings domain.Settings
	state    domain.RuntimeState
}

func (s *memoryStore) LoadSubscriptions() ([]domain.Subscription, error) {
	return s.subs, nil
}

func (s *memoryStore) SaveSubscriptions(subs []domain.Subscription) error {
	s.subs = subs
	return nil
}

func (s *memoryStore) LoadSettings() (domain.Settings, error) {
	return s.settings, nil
}

func (s *memoryStore) SaveSettings(settings domain.Settings) error {
	s.settings = settings
	return nil
}

func (s *memoryStore) LoadState() (domain.RuntimeState, error) {
	return s.state, nil
}

func (s *memoryStore) SaveState(state domain.RuntimeState) error {
	s.state = state
	return nil
}

type recordingBackend struct {
	requests   []backend.ConfigRequest
	stopCalls  int
	startCalls int
}

func (b *recordingBackend) GenerateConfig(req backend.ConfigRequest) ([]byte, error) {
	return nil, nil
}

func (b *recordingBackend) ApplyConfig(_ context.Context, req backend.ConfigRequest) error {
	b.requests = append(b.requests, req)
	return nil
}

func (b *recordingBackend) Start(context.Context) error {
	b.startCalls++
	return nil
}

func (b *recordingBackend) Stop(context.Context) error {
	b.stopCalls++
	return nil
}

func (b *recordingBackend) Reload(context.Context) error { return nil }
func (b *recordingBackend) Status(context.Context) (backend.RuntimeStatus, error) {
	return backend.RuntimeStatus{}, nil
}

type recordingFirewaller struct {
	applied      []domain.FirewallSettings
	disableCalls int
}

func (f *recordingFirewaller) Apply(_ context.Context, settings domain.FirewallSettings) error {
	f.applied = append(f.applied, settings)
	return nil
}

func (f *recordingFirewaller) Disable(context.Context) error {
	f.disableCalls++
	return nil
}

type fakeChecker struct {
	results map[string]probe.Result
}

func (f fakeChecker) Check(_ context.Context, node domain.Node) probe.Result {
	if result, ok := f.results[node.ID]; ok {
		result.NodeID = node.ID
		if result.Checked.IsZero() {
			result.Checked = time.Now().UTC()
		}
		return result
	}
	return probe.Result{
		NodeID:  node.ID,
		Healthy: true,
		Latency: time.Second,
		Checked: time.Now().UTC(),
		Err:     fmt.Errorf("missing fake probe result for %s", node.ID),
	}
}

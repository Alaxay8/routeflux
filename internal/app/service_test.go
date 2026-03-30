package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

func writeResponse(w http.ResponseWriter, body string) {
	_, _ = io.WriteString(w, body)
}

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
	if len(settings.TargetDomains) != 0 {
		t.Fatalf("expected destination target domains to be cleared, got %v", settings.TargetDomains)
	}
	if len(settings.TargetServices) != 0 {
		t.Fatalf("expected destination target services to be cleared, got %v", settings.TargetServices)
	}
	if len(settings.SourceCIDRs) != 1 || settings.SourceCIDRs[0] != "192.168.1.150" {
		t.Fatalf("unexpected source hosts: %v", settings.SourceCIDRs)
	}
	if !settings.BlockQUIC {
		t.Fatal("expected QUIC blocking to be enabled for host routing")
	}
}

func TestConfigureFirewallParsesMixedTargetsAndValidatesBeforeSave(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	firewall := &recordingFirewaller{
		validateErr: fmt.Errorf("dnsmasq-full is required for domain targets"),
	}

	service := NewService(Dependencies{Store: store, Firewaller: firewall})

	_, err := service.ConfigureFirewall(context.Background(), []string{"youtube", "youtube.com", "1.1.1.1"}, true, 23456)
	if err == nil {
		t.Fatal("expected configure firewall to fail")
	}
	if !strings.Contains(err.Error(), "dnsmasq-full") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firewall.validated) != 1 {
		t.Fatalf("expected validate to be called once, got %d", len(firewall.validated))
	}
	if !reflect.DeepEqual(firewall.validated[0].TargetServices, []string{"youtube"}) {
		t.Fatalf("unexpected validated target services: %+v", firewall.validated[0].TargetServices)
	}
	if !reflect.DeepEqual(firewall.validated[0].TargetCIDRs, []string{"1.1.1.1"}) {
		t.Fatalf("unexpected validated target cidrs: %+v", firewall.validated[0].TargetCIDRs)
	}
	if !reflect.DeepEqual(firewall.validated[0].TargetDomains, []string{"youtube.com"}) {
		t.Fatalf("unexpected validated target domains: %+v", firewall.validated[0].TargetDomains)
	}
	if len(store.settings.Firewall.TargetServices) != 0 || len(store.settings.Firewall.TargetCIDRs) != 0 || len(store.settings.Firewall.TargetDomains) != 0 {
		t.Fatalf("expected settings to stay unchanged on validate failure, got %+v", store.settings.Firewall)
	}
}

func TestConfigureFirewallClearsHostsAndStoresTargetSelectors(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}

	service := NewService(Dependencies{Store: store, Firewaller: &recordingFirewaller{}})

	settings, err := service.ConfigureFirewall(context.Background(), []string{"YouTube", "YouTube.com", "1.1.1.1"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall: %v", err)
	}

	if len(settings.SourceCIDRs) != 0 {
		t.Fatalf("expected hosts to be cleared, got %v", settings.SourceCIDRs)
	}
	if !reflect.DeepEqual(settings.TargetServices, []string{"youtube"}) {
		t.Fatalf("unexpected target services: %+v", settings.TargetServices)
	}
	if !reflect.DeepEqual(settings.TargetCIDRs, []string{"1.1.1.1"}) {
		t.Fatalf("unexpected target cidrs: %+v", settings.TargetCIDRs)
	}
	if !reflect.DeepEqual(settings.TargetDomains, []string{"youtube.com"}) {
		t.Fatalf("unexpected target domains: %+v", settings.TargetDomains)
	}
}

func TestConfigureFirewallAntiTargetsClearsHostsAndStoresTargetSelectors(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.SourceCIDRs = []string{"192.168.1.150"}

	service := NewService(Dependencies{Store: store, Firewaller: &recordingFirewaller{}})

	settings, err := service.ConfigureFirewallAntiTargets(context.Background(), []string{"YouTube", "YouTube.com", "1.1.1.1"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall anti-targets: %v", err)
	}

	if len(settings.SourceCIDRs) != 0 {
		t.Fatalf("expected hosts to be cleared, got %v", settings.SourceCIDRs)
	}
	if settings.TargetMode != domain.FirewallTargetModeBypass {
		t.Fatalf("expected target mode bypass, got %q", settings.TargetMode)
	}
	if !reflect.DeepEqual(settings.TargetServices, []string{"youtube"}) {
		t.Fatalf("unexpected target services: %+v", settings.TargetServices)
	}
	if !reflect.DeepEqual(settings.TargetCIDRs, []string{"1.1.1.1"}) {
		t.Fatalf("unexpected target cidrs: %+v", settings.TargetCIDRs)
	}
	if !reflect.DeepEqual(settings.TargetDomains, []string{"youtube.com"}) {
		t.Fatalf("unexpected target domains: %+v", settings.TargetDomains)
	}
}

func TestConfigureFirewallSupportsCustomServiceAliases(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.TargetServiceCatalog = map[string]domain.FirewallTargetDefinition{
		"openai": {
			Domains: []string{"openai.com", "chatgpt.com"},
		},
	}

	service := NewService(Dependencies{Store: store, Firewaller: &recordingFirewaller{}})

	settings, err := service.ConfigureFirewall(context.Background(), []string{"openai"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall: %v", err)
	}

	if !reflect.DeepEqual(settings.TargetServices, []string{"openai"}) {
		t.Fatalf("unexpected target services: %+v", settings.TargetServices)
	}
}

func TestConfigureFirewallHostsPreservesTargetServiceCatalog(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.TargetServiceCatalog = map[string]domain.FirewallTargetDefinition{
		"openai": {Domains: []string{"openai.com"}},
	}

	service := NewService(Dependencies{Store: store, Firewaller: &recordingFirewaller{}})

	settings, err := service.ConfigureFirewallHosts(context.Background(), []string{"192.168.1.150"}, true, 23456)
	if err != nil {
		t.Fatalf("configure firewall hosts: %v", err)
	}

	if !reflect.DeepEqual(settings.TargetServiceCatalog, map[string]domain.FirewallTargetDefinition{
		"openai": {Domains: []string{"openai.com"}},
	}) {
		t.Fatalf("unexpected target service catalog: %+v", settings.TargetServiceCatalog)
	}
}

func TestSetFirewallTargetServiceReappliesConnectedRuntime(t *testing.T) {
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
	store.settings.Firewall.TargetServices = []string{"openai"}
	store.state.Connected = true
	store.state.ActiveSubscriptionID = "sub-1"
	store.state.ActiveNodeID = "node-1"
	store.state.Mode = domain.SelectionModeManual

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	entry, err := service.SetFirewallTargetService(context.Background(), "openai", []string{"openai.com", "chatgpt.com"})
	if err != nil {
		t.Fatalf("set firewall target service: %v", err)
	}

	if entry.Name != "openai" || entry.Source != domain.FirewallTargetServiceSourceCustom || entry.ReadOnly {
		t.Fatalf("unexpected target service entry: %+v", entry)
	}
	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend reapply, got %d", len(runtimeBackend.requests))
	}
	if want := []string{"openai.com", "chatgpt.com"}; !reflect.DeepEqual(runtimeBackend.requests[0].TransparentTargetDomains, want) {
		t.Fatalf("unexpected transparent target domains:\nwant: %+v\n got: %+v", want, runtimeBackend.requests[0].TransparentTargetDomains)
	}
	if len(firewall.applied) != 1 {
		t.Fatalf("expected firewall apply once, got %d", len(firewall.applied))
	}
}

func TestDeleteFirewallTargetServiceRejectsUsedAlias(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	store.settings.Firewall.TargetServices = []string{"openai"}
	store.settings.Firewall.TargetServiceCatalog = map[string]domain.FirewallTargetDefinition{
		"openai": {Domains: []string{"openai.com"}},
	}

	service := NewService(Dependencies{Store: store, Firewaller: &recordingFirewaller{}})

	err := service.DeleteFirewallTargetService(context.Background(), "openai")
	if err == nil {
		t.Fatal("expected delete to fail while alias is in use")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "remove it from firewall targets first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddSubscriptionRetriesTransientHTTPStatus(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary upstream error", http.StatusServiceUnavailable)
			return
		}

		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL:  server.URL,
		Name: "Retry Test",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 fetch attempts, got %d", attempts)
	}
	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(sub.Nodes))
	}
}

func TestAddSubscriptionUsesCompatibleUserAgent(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() == "" || r.UserAgent() == "Go-http-client/1.1" {
			http.Error(w, "blocked user agent", http.StatusServiceUnavailable)
			return
		}

		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL:  server.URL,
		Name: "UA Test",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(sub.Nodes))
	}
}

func TestAddSubscriptionRetriesWithCookieJar(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if _, err := r.Cookie("routeflux-clearance"); err != nil {
			http.SetCookie(w, &http.Cookie{
				Name:  "routeflux-clearance",
				Value: "ok",
				Path:  "/",
			})
			http.Error(w, "temporary upstream error", http.StatusServiceUnavailable)
			return
		}

		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL:  server.URL,
		Name: "Cookie Test",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 fetch attempts, got %d", attempts)
	}
	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(sub.Nodes))
	}
}

func TestAddSubscriptionUsesProfileTitleHeaderForProviderName(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: server.URL,
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if sub.ProviderName != "Demo VPN" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.DisplayName != "Demo VPN" {
		t.Fatalf("unexpected display name: %q", sub.DisplayName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceHeader {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
	}
	if len(sub.Nodes) != 1 || sub.Nodes[0].ProviderName != "Demo VPN" {
		t.Fatalf("unexpected parsed nodes: %+v", sub.Nodes)
	}
}

func TestAddSubscriptionKeepsManualProviderNameDespiteProfileTitle(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL:  server.URL,
		Name: "Manual Name",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if sub.ProviderName != "Manual Name" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceManual {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
	}
}

func TestAddSubscriptionKeepsDistinctProfilesForSharedURL(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		switch requests {
		case 1:
			writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fone&host=cdn.example.com#Profile%201")
		default:
			writeResponse(w, "vless://22222222-2222-2222-2222-222222222222@node2.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Ftwo&host=cdn.example.com#Profile%202")
		}
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	first, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("add first subscription: %v", err)
	}

	second, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("add second subscription: %v", err)
	}

	if first.ID == second.ID {
		t.Fatalf("expected distinct subscription ids, got %q", first.ID)
	}
	if len(store.subs) != 2 {
		t.Fatalf("expected two stored subscriptions, got %d", len(store.subs))
	}
	if store.subs[0].ID != first.ID || store.subs[1].ID != second.ID {
		t.Fatalf("unexpected stored subscriptions: %+v", store.subs)
	}
	if len(store.subs[0].Nodes) != 1 || store.subs[0].Nodes[0].SubscriptionID != first.ID {
		t.Fatalf("unexpected first subscription nodes: %+v", store.subs[0].Nodes)
	}
	if len(store.subs[1].Nodes) != 1 || store.subs[1].Nodes[0].SubscriptionID != second.ID {
		t.Fatalf("unexpected second subscription nodes: %+v", store.subs[1].Nodes)
	}
}

func TestAddSubscriptionReusesIDForEquivalentSharedURL(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		writeResponse(w, "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fone&host=cdn.example.com#Profile%201")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	first, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("add first subscription: %v", err)
	}

	second, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("add second subscription: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected equivalent shared URL to reuse id, got %q and %q", first.ID, second.ID)
	}
	if len(store.subs) != 1 {
		t.Fatalf("expected one stored subscription, got %d", len(store.subs))
	}
}

func TestAddSubscriptionReturnsJSONEndpointError(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeResponse(w, `{"error":"USER_NOT_FOUND","info":"User account does not exist."}`)
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	_, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: server.URL,
	})
	if err == nil {
		t.Fatal("expected add subscription to fail")
	}
	if got := err.Error(); !strings.Contains(got, "USER_NOT_FOUND") || !strings.Contains(got, "User account does not exist.") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddSubscriptionReturnsHTMLResponseError(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeResponse(w, "<html><body><h1>Login required</h1><p>Open the provider portal first.</p></body></html>")
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	_, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: server.URL,
	})
	if err == nil {
		t.Fatal("expected add subscription to fail")
	}
	if got := err.Error(); !strings.Contains(got, "Login required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddSubscriptionExtractsHTMLShareLinksWithSpacesInRemark(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeResponse(w, `<html><body><input readonly value="vless://8b922611-af1c-40c9-9af0-80fd0d782084@snl4.linkey8.ru:8443?security=reality&amp;type=tcp&amp;flow=xtls-rprx-vision&amp;sni=www.vk.com&amp;fp=qq&amp;pbk=wDQjzXYVtjdLkEyXpReh973y4rDIDH6kkX-g-MR7xAg&amp;sid=#🇳🇱 Нидерланды"></body></html>`)
	}))
	t.Cleanup(server.Close)

	service := NewService(Dependencies{
		Store:      store,
		HTTPClient: server.Client(),
	})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		URL: server.URL,
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(sub.Nodes))
	}
	if sub.Nodes[0].Remark != "🇳🇱 Нидерланды" {
		t.Fatalf("expected full remark from html share link, got %+v", sub.Nodes[0])
	}
	if sub.Nodes[0].Name != sub.Nodes[0].Remark {
		t.Fatalf("expected name to mirror remark, got %+v", sub.Nodes[0])
	}
}

func TestAddSubscriptionAcceptsDirectVLESSJSONConfig(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	service := NewService(Dependencies{Store: store})

	sub, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		Raw: `{
		  "outbounds": [
		    {
		      "settings": {
		        "encryption": "none",
		        "flow": "xtls-rprx-vision",
		        "port": 8443,
		        "address": "hungary-edge.example",
		        "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		      },
		      "protocol": "vless",
		      "tag": "proxy",
		      "streamSettings": {
		        "realitySettings": {
		          "shortId": "testshort01",
		          "publicKey": "test-public-key",
		          "serverName": "gateway.example",
		          "fingerprint": "random"
		        },
		        "security": "reality",
		        "network": "tcp"
		      }
		    }
		  ],
		  "remarks": "🇭🇺Венгрия"
		}`,
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if len(sub.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(sub.Nodes))
	}
	if sub.Nodes[0].Name != "🇭🇺Венгрия" {
		t.Fatalf("unexpected node label: %+v", sub.Nodes[0])
	}
	if sub.Nodes[0].Address != "hungary-edge.example" || sub.Nodes[0].Port != 8443 {
		t.Fatalf("unexpected node endpoint: %+v", sub.Nodes[0])
	}
}

func TestRefreshSubscriptionUpdatesLegacyURLDerivedProviderNameFromProfileTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		writeResponse(w, `[
		  {
		    "outbounds": [
		      {
		        "protocol": "vless",
		        "tag": "proxy",
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
		            "serverName": "gateway-one.example",
		            "publicKey": "public-key-one",
		            "shortId": "short-one",
		            "fingerprint": "random"
		          }
		        }
		      }
		    ]
		  }
		]`)
	}))
	defer server.Close()

	legacyName := deriveProviderName(domain.SourceTypeURL, server.URL)
	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:           "sub-1",
				SourceType:   domain.SourceTypeURL,
				Source:       server.URL,
				ProviderName: legacyName,
				DisplayName:  legacyName,
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

	if sub.ProviderName != "Demo VPN" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.DisplayName != "Demo VPN" {
		t.Fatalf("unexpected display name: %q", sub.DisplayName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceHeader {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
	}
	if len(sub.Nodes) != 1 || sub.Nodes[0].ProviderName != "Demo VPN" {
		t.Fatalf("unexpected parsed nodes: %+v", sub.Nodes)
	}
}

func TestRefreshSubscriptionDoesNotOverrideManualProviderNameWithProfileTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:RGVtbyBWUE4=")
		writeResponse(w, `[
		  {
		    "outbounds": [
		      {
		        "protocol": "vless",
		        "tag": "proxy",
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
		            "serverName": "gateway-one.example",
		            "publicKey": "public-key-one",
		            "shortId": "short-one",
		            "fingerprint": "random"
		          }
		        }
		      }
		    ]
		  }
		]`)
	}))
	defer server.Close()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:                 "sub-1",
				SourceType:         domain.SourceTypeURL,
				Source:             server.URL,
				ProviderName:       "Manual Name",
				DisplayName:        "Manual Name",
				ProviderNameSource: domain.ProviderNameSourceManual,
				Nodes: []domain.Node{
					{ID: "old-node"},
				},
			},
		},
	}
	client := server.Client()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	client.Transport = rewriteURLRoundTripper{
		base:   client.Transport,
		target: targetURL,
	}

	service := NewService(Dependencies{Store: store, HTTPClient: client})

	sub, err := service.RefreshSubscription(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}

	if sub.ProviderName != "Manual Name" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.DisplayName != "Manual Name" {
		t.Fatalf("unexpected display name: %q", sub.DisplayName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceManual {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
	}
	if len(sub.Nodes) != 1 || sub.Nodes[0].ProviderName != "Manual Name" {
		t.Fatalf("unexpected parsed nodes: %+v", sub.Nodes)
	}
}

func TestRefreshSubscriptionUpgradesLegacyKeyVPNNameFromProfileTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", "base64:REVNTyBWUE4=")
		writeResponse(w, `[
		  {
		    "outbounds": [
		      {
		        "protocol": "vless",
		        "tag": "proxy",
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
		            "serverName": "gateway-one.example",
		            "publicKey": "public-key-one",
		            "shortId": "short-one",
		            "fingerprint": "random"
		          }
		        }
		      }
		    ]
		  }
		]`)
	}))
	defer server.Close()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:           "sub-1",
				SourceType:   domain.SourceTypeURL,
				Source:       "https://key.vpndemo.example/subscriptions/demo-token",
				ProviderName: "Key VPN",
				DisplayName:  "Key VPN",
				Nodes: []domain.Node{
					{ID: "old-node"},
				},
			},
		},
	}
	if !canUpgradeLegacyProviderName(store.subs[0], "Key VPN") {
		t.Fatal("expected Key VPN legacy name to be upgradeable")
	}

	client := server.Client()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	client.Transport = rewriteURLRoundTripper{
		base:   client.Transport,
		target: targetURL,
	}

	service := NewService(Dependencies{Store: store, HTTPClient: client})

	sub, err := service.RefreshSubscription(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}

	if sub.ProviderName != "DEMO VPN" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.DisplayName != "DEMO VPN" {
		t.Fatalf("unexpected display name: %q", sub.DisplayName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceHeader {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
	}
}

func TestRefreshSubscriptionNormalizesLegacyRawHostNameWithoutProfileTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(w, `[
		  {
		    "outbounds": [
		      {
		        "protocol": "vless",
		        "tag": "proxy",
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
		            "serverName": "gateway-one.example",
		            "publicKey": "public-key-one",
		            "shortId": "short-one",
		            "fingerprint": "random"
		          }
		        }
		      }
		    ]
		  }
		]`)
	}))
	defer server.Close()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:           "sub-1",
				SourceType:   domain.SourceTypeURL,
				Source:       "https://key.vpndemo.example/subscriptions/demo-token",
				ProviderName: "key.vpndemo.example",
				DisplayName:  "key.vpndemo.example",
				Nodes: []domain.Node{
					{ID: "old-node"},
				},
			},
		},
	}
	if !canUpgradeLegacyProviderName(store.subs[0], "key.vpndemo.example") {
		t.Fatal("expected raw legacy host name to be upgradeable")
	}

	client := server.Client()
	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	client.Transport = rewriteURLRoundTripper{
		base:   client.Transport,
		target: targetURL,
	}

	service := NewService(Dependencies{Store: store, HTTPClient: client})

	sub, err := service.RefreshSubscription(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("refresh subscription: %v", err)
	}

	if sub.ProviderName != "Demo VPN" {
		t.Fatalf("unexpected provider name: %q", sub.ProviderName)
	}
	if sub.DisplayName != "Demo VPN" {
		t.Fatalf("unexpected display name: %q", sub.DisplayName)
	}
	if sub.ProviderNameSource != domain.ProviderNameSourceURL {
		t.Fatalf("unexpected provider name source: %q", sub.ProviderNameSource)
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

func TestConnectManualPassesExpandedTargetSelectorsToBackend(t *testing.T) {
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
	store.settings.Firewall.TargetServices = []string{"youtube", "telegram"}
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

	wantDomains := []string{
		"youtube.com",
		"youtu.be",
		"youtube-nocookie.com",
		"youtubei.googleapis.com",
		"youtube.googleapis.com",
		"googlevideo.com",
		"ytimg.com",
		"ggpht.com",
		"telegram.org",
		"t.me",
		"telegram.me",
		"web.telegram.org",
		"desktop.telegram.org",
		"core.telegram.org",
	}
	if !reflect.DeepEqual(runtimeBackend.requests[0].TransparentTargetDomains, wantDomains) {
		t.Fatalf("unexpected transparent target domains:\nwant: %+v\n got: %+v", wantDomains, runtimeBackend.requests[0].TransparentTargetDomains)
	}

	wantCIDRs := []string{
		"91.108.0.0/16",
		"149.154.0.0/16",
	}
	if !reflect.DeepEqual(runtimeBackend.requests[0].TransparentTargetCIDRs, wantCIDRs) {
		t.Fatalf("unexpected transparent target cidrs:\nwant: %+v\n got: %+v", wantCIDRs, runtimeBackend.requests[0].TransparentTargetCIDRs)
	}
}

func TestConnectManualPassesExpandedAntiTargetSelectorsToBackend(t *testing.T) {
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
	store.settings.Firewall.TargetMode = domain.FirewallTargetModeBypass
	store.settings.Firewall.TargetServices = []string{"youtube", "telegram"}
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
	if runtimeBackend.requests[0].TransparentTargetMode != domain.FirewallTargetModeBypass {
		t.Fatalf("unexpected transparent target mode: %q", runtimeBackend.requests[0].TransparentTargetMode)
	}

	wantDomains := []string{
		"youtube.com",
		"youtu.be",
		"youtube-nocookie.com",
		"youtubei.googleapis.com",
		"youtube.googleapis.com",
		"googlevideo.com",
		"ytimg.com",
		"ggpht.com",
		"telegram.org",
		"t.me",
		"telegram.me",
		"web.telegram.org",
		"desktop.telegram.org",
		"core.telegram.org",
	}
	if !reflect.DeepEqual(runtimeBackend.requests[0].TransparentTargetDomains, wantDomains) {
		t.Fatalf("unexpected transparent target domains:\nwant: %+v\n got: %+v", wantDomains, runtimeBackend.requests[0].TransparentTargetDomains)
	}

	wantCIDRs := []string{
		"91.108.0.0/16",
		"149.154.0.0/16",
	}
	if !reflect.DeepEqual(runtimeBackend.requests[0].TransparentTargetCIDRs, wantCIDRs) {
		t.Fatalf("unexpected transparent target cidrs:\nwant: %+v\n got: %+v", wantCIDRs, runtimeBackend.requests[0].TransparentTargetCIDRs)
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

func TestConnectManualResolvesNodeAddressBeforeApplyingBackendConfig(t *testing.T) {
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
						Name:     "Russia",
						Protocol: domain.ProtocolVLESS,
						Address:  "ru-sb-01.com",
						Port:     8443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}

	runtimeBackend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: runtimeBackend,
		Resolver: fakeResolver{
			lookups: map[string][]net.IPAddr{
				"ru-sb-01.com": {
					{IP: net.ParseIP("103.113.68.112")},
				},
			},
		},
	})

	if err := service.ConnectManual(context.Background(), "sub-1", "node-1"); err != nil {
		t.Fatalf("connect manual: %v", err)
	}

	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply, got %d", len(runtimeBackend.requests))
	}
	if got := runtimeBackend.requests[0].Nodes[0].Address; got != "103.113.68.112" {
		t.Fatalf("expected resolved backend address, got %q", got)
	}
	if got := store.subs[0].Nodes[0].Address; got != "ru-sb-01.com" {
		t.Fatalf("expected stored node address to remain unchanged, got %q", got)
	}
}

func TestRuntimeStatusReturnsBackendStatus(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	backend := &recordingBackend{
		status: backend.RuntimeStatus{
			Running:      true,
			ConfigPath:   "/etc/xray/config.json",
			ServiceState: "running",
		},
	}

	service := NewService(Dependencies{
		Store:   store,
		Backend: backend,
	})

	status, err := service.RuntimeStatus(context.Background())
	if err != nil {
		t.Fatalf("runtime status: %v", err)
	}

	if !status.Running || status.ServiceState != "running" || status.ConfigPath != "/etc/xray/config.json" {
		t.Fatalf("unexpected runtime status: %+v", status)
	}
}

func TestRuntimeStatusWithoutBackendReturnsZeroValue(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}

	service := NewService(Dependencies{Store: store})

	status, err := service.RuntimeStatus(context.Background())
	if err != nil {
		t.Fatalf("runtime status without backend: %v", err)
	}

	if status != (backend.RuntimeStatus{}) {
		t.Fatalf("expected zero runtime status, got %+v", status)
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
		            "serverName": "gateway-one.example",
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

func TestRemoveAllSubscriptionsDisconnectsActiveSubscription(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{ID: "sub-1", DisplayName: "One"},
			{ID: "sub-2", DisplayName: "Two"},
		},
	}
	store.settings.AutoMode = true
	store.settings.Mode = domain.SelectionModeAuto
	store.state.ActiveSubscriptionID = "sub-2"
	store.state.ActiveNodeID = "node-2"
	store.state.Mode = domain.SelectionModeAuto
	store.state.Connected = true

	runtimeBackend := &recordingBackend{}
	firewall := &recordingFirewaller{}
	service := NewService(Dependencies{
		Store:      store,
		Backend:    runtimeBackend,
		Firewaller: firewall,
	})

	removed, err := service.RemoveAllSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("remove all subscriptions: %v", err)
	}

	if removed != 2 {
		t.Fatalf("expected to remove 2 subscriptions, got %d", removed)
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

	saveStateCalls int
}

type rewriteURLRoundTripper struct {
	base   http.RoundTripper
	target *url.URL
}

func (r rewriteURLRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := r.base
	if transport == nil {
		transport = http.DefaultTransport
	}

	cloned := req.Clone(req.Context())
	cloned.URL = cloneURL(req.URL)
	cloned.URL.Scheme = r.target.Scheme
	cloned.URL.Host = r.target.Host
	return transport.RoundTrip(cloned)
}

func cloneURL(value *url.URL) *url.URL {
	if value == nil {
		return &url.URL{}
	}
	copy := *value
	return &copy
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
	s.saveStateCalls++
	return nil
}

type recordingBackend struct {
	requests    []backend.ConfigRequest
	stopCalls   int
	startCalls  int
	status      backend.RuntimeStatus
	statuses    []backend.RuntimeStatus
	statusCalls int
	statusErr   error
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
	if b.statusErr != nil {
		return backend.RuntimeStatus{}, b.statusErr
	}
	if len(b.statuses) > 0 {
		index := b.statusCalls
		b.statusCalls++
		if index >= len(b.statuses) {
			index = len(b.statuses) - 1
		}
		return b.statuses[index], nil
	}
	b.statusCalls++
	if b.status == (backend.RuntimeStatus{}) {
		return backend.RuntimeStatus{Running: true, ServiceState: "running"}, nil
	}
	return b.status, nil
}

type recordingFirewaller struct {
	applied      []domain.FirewallSettings
	validated    []domain.FirewallSettings
	disableCalls int
	validateErr  error
}

func (f *recordingFirewaller) Validate(_ context.Context, settings domain.FirewallSettings) error {
	f.validated = append(f.validated, settings)
	return f.validateErr
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

type fakeResolver struct {
	lookups map[string][]net.IPAddr
	err     error
}

func (r fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	if result, ok := r.lookups[host]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("missing fake resolver result for %s", host)
}

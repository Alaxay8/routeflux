package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestBuildProviderGroupsGroupsSubscriptionsByProvider(t *testing.T) {
	t.Parallel()

	subscriptions := []domain.Subscription{
		{
			ID:           "sub-sample-main",
			ProviderName: "Sample VPN",
			DisplayName:  "Classic VLESS",
			Nodes:        []domain.Node{{ID: "node-1"}},
		},
		{
			ID:           "sub-sample-bypass",
			ProviderName: "Sample VPN",
			DisplayName:  "Bypass",
			Nodes:        []domain.Node{{ID: "node-2"}, {ID: "node-3"}},
		},
		{
			ID:           "sub-demo",
			ProviderName: "Demo VPN",
			DisplayName:  "Main",
			Nodes:        []domain.Node{{ID: "node-4"}},
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 2 {
		t.Fatalf("expected 2 provider groups, got %d", len(groups))
	}

	demo := groups[0]
	if demo.Title != "Demo VPN" {
		t.Fatalf("unexpected provider title: %q", demo.Title)
	}
	if demo.TotalNodes != 1 {
		t.Fatalf("unexpected Demo node count: %d", demo.TotalNodes)
	}
	if len(demo.Subscriptions) != 1 || demo.Subscriptions[0].Label != "Main" {
		t.Fatalf("unexpected Demo subscriptions: %+v", demo.Subscriptions)
	}

	sample := groups[1]
	if sample.Title != "Sample VPN" {
		t.Fatalf("unexpected provider title: %q", sample.Title)
	}
	if sample.TotalNodes != 3 {
		t.Fatalf("unexpected Sample node count: %d", sample.TotalNodes)
	}
	if len(sample.Subscriptions) != 2 {
		t.Fatalf("expected 2 subscriptions for Sample VPN, got %d", len(sample.Subscriptions))
	}
	if sample.Subscriptions[0].Label != "Bypass" || sample.Subscriptions[1].Label != "Classic VLESS" {
		t.Fatalf("unexpected Sample labels: %+v", sample.Subscriptions)
	}
}

func TestBuildProviderGroupsAssignsFallbackProfileLabels(t *testing.T) {
	t.Parallel()

	subscriptions := []domain.Subscription{
		{
			ID:           "sub-1",
			ProviderName: "Sample VPN",
			DisplayName:  "Sample VPN",
		},
		{
			ID:           "sub-2",
			ProviderName: "Sample VPN",
			DisplayName:  "Sample VPN",
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 1 {
		t.Fatalf("expected 1 provider group, got %d", len(groups))
	}

	labels := []string{
		groups[0].Subscriptions[0].Label,
		groups[0].Subscriptions[1].Label,
	}
	if labels[0] != "Profile 1" || labels[1] != "Profile 2" {
		t.Fatalf("unexpected fallback labels: %v", labels)
	}
}

func TestBuildProviderGroupsHumanizesDomainProviderNames(t *testing.T) {
	t.Parallel()

	subscriptions := []domain.Subscription{
		{
			ID:           "sub-1",
			ProviderName: "key.vpndemo.example",
			DisplayName:  "key.vpndemo.example",
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 1 {
		t.Fatalf("expected 1 provider group, got %d", len(groups))
	}
	if groups[0].Title != "Demo VPN" {
		t.Fatalf("unexpected provider title: %q", groups[0].Title)
	}
}

func TestBuildProviderGroupsStripsEmojiFromTUITitlesAndLabels(t *testing.T) {
	t.Parallel()

	subscriptions := []domain.Subscription{
		{
			ID:           "sub-1",
			ProviderName: "Liberty VPN 🗽",
			DisplayName:  "🇯🇵 Japan",
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 1 {
		t.Fatalf("expected 1 provider group, got %d", len(groups))
	}
	if groups[0].Title != "Liberty VPN" {
		t.Fatalf("unexpected provider title: %q", groups[0].Title)
	}
	if groups[0].Subscriptions[0].Label != "Japan" {
		t.Fatalf("unexpected profile label: %q", groups[0].Subscriptions[0].Label)
	}
}

func TestRenderNodesStripsEmojiFromNodeNames(t *testing.T) {
	t.Parallel()

	m := newModel(nil)
	m.subscriptions = []domain.Subscription{
		{
			ID: "sub-1",
			Nodes: []domain.Node{
				{
					ID:         "node-1",
					Name:       "🇷🇺Россия|Torrent-2",
					Protocol:   domain.ProtocolVLESS,
					Transport:  "tcp",
					Address:    "ru.example.com",
					Port:       443,
					Security:   "reality",
					ServerName: "sni.example.com",
				},
			},
		},
	}
	m.providers = buildProviderGroups(m.subscriptions)
	m.ensureSelection()

	view := renderNodes(m, 8)
	if strings.Contains(view, "🇷🇺") {
		t.Fatalf("expected TUI node list to strip emoji, got:\n%s", view)
	}
	if !strings.Contains(view, "Россия|Torrent-2") {
		t.Fatalf("expected sanitized node title, got:\n%s", view)
	}
}

func TestRenderStatusStripsEmojiFromActiveNodeNames(t *testing.T) {
	t.Parallel()

	m := newModel(nil)
	m.status.State.Mode = domain.SelectionModeManual
	m.status.State.Connected = true
	m.status.ActiveNode = &domain.Node{Name: "🇭🇰Гонконг"}
	m.status.ActiveSubscription = &domain.Subscription{
		ProviderName: "Liberty VPN 🗽",
		DisplayName:  "🇭🇰 Profile",
	}

	view := renderStatus(m)
	if strings.Contains(view, "🗽") || strings.Contains(view, "🇭🇰") {
		t.Fatalf("expected status view to strip emoji, got:\n%s", view)
	}
	if !strings.Contains(view, "Liberty VPN") || !strings.Contains(view, "Гонконг") {
		t.Fatalf("expected sanitized status labels, got:\n%s", view)
	}
}

func TestRenderProvidersShowsProviderHierarchy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)
	m := model{
		subscriptions: []domain.Subscription{
			{
				ID:            "sub-sample-main",
				ProviderName:  "Sample VPN",
				DisplayName:   "Classic VLESS",
				LastUpdatedAt: now,
				ParserStatus:  "ok",
				Nodes:         []domain.Node{{ID: "node-1"}, {ID: "node-2"}},
			},
			{
				ID:            "sub-sample-bypass",
				ProviderName:  "Sample VPN",
				DisplayName:   "Bypass",
				LastUpdatedAt: now,
				ParserStatus:  "ok",
				Nodes:         []domain.Node{{ID: "node-3"}},
			},
		},
		selectedProvider: 0,
		selectedProfile:  1,
		headerStyle:      lipgloss.NewStyle(),
		mutedStyle:       lipgloss.NewStyle(),
		activeStyle:      lipgloss.NewStyle(),
	}
	m.providers = buildProviderGroups(m.subscriptions)

	providers := renderProviders(m)
	profiles := renderProfiles(m)

	wants := []string{
		"VPN Services",
		"Sample VPN",
		"2 profiles",
		"Profiles",
		"Classic VLESS",
		"Bypass",
	}
	for _, want := range wants {
		if !strings.Contains(providers+"\n"+profiles, want) {
			t.Fatalf("render output missing %q\nproviders:\n%s\nprofiles:\n%s", want, providers, profiles)
		}
	}
}

func TestRenderMainViewKeepsTopSectionsVisibleWithManyNodes(t *testing.T) {
	t.Parallel()

	nodes := make([]domain.Node, 0, 20)
	for idx := 0; idx < 20; idx++ {
		nodes = append(nodes, domain.Node{
			ID:         fmt.Sprintf("node-%d", idx),
			Name:       fmt.Sprintf("Node %d", idx),
			Protocol:   domain.ProtocolVLESS,
			Transport:  "tcp",
			Address:    fmt.Sprintf("host-%d.example.com", idx),
			Port:       443,
			Security:   "reality",
			ServerName: "gateway.example",
		})
	}

	m := newModel(nil)
	m.height = 18
	m.status.State.Connected = true
	m.status.State.Mode = domain.SelectionModeManual
	m.subscriptions = []domain.Subscription{
		{
			ID:           "sub-sample-main",
			ProviderName: "connsample.example",
			DisplayName:  "Classic VLESS",
			Nodes:        nodes,
		},
	}
	m.providers = buildProviderGroups(m.subscriptions)
	m.ensureSelection()

	view := m.View()
	wants := []string{
		"RouteFlux",
		"VPN Services",
		"Profiles",
		"Locations",
	}
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q\n%s", want, view)
		}
	}

	if strings.Count(view, "security=reality") >= 20 {
		t.Fatalf("expected node list to be windowed, got full list:\n%s", view)
	}
}

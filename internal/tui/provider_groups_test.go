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
			ID:           "sub-lib-main",
			ProviderName: "Liberty VPN",
			DisplayName:  "Classic VLESS",
			Nodes:        []domain.Node{{ID: "node-1"}},
		},
		{
			ID:           "sub-lib-bypass",
			ProviderName: "Liberty VPN",
			DisplayName:  "Bypass",
			Nodes:        []domain.Node{{ID: "node-2"}, {ID: "node-3"}},
		},
		{
			ID:           "sub-starlink",
			ProviderName: "Starlink VPN",
			DisplayName:  "Main",
			Nodes:        []domain.Node{{ID: "node-4"}},
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 2 {
		t.Fatalf("expected 2 provider groups, got %d", len(groups))
	}

	liberty := groups[0]
	if liberty.Title != "Liberty VPN" {
		t.Fatalf("unexpected provider title: %q", liberty.Title)
	}
	if liberty.TotalNodes != 3 {
		t.Fatalf("unexpected provider node count: %d", liberty.TotalNodes)
	}
	if len(liberty.Subscriptions) != 2 {
		t.Fatalf("expected 2 subscriptions for Liberty VPN, got %d", len(liberty.Subscriptions))
	}
	if liberty.Subscriptions[0].Label != "Bypass" || liberty.Subscriptions[1].Label != "Classic VLESS" {
		t.Fatalf("unexpected provider labels: %+v", liberty.Subscriptions)
	}
}

func TestBuildProviderGroupsAssignsFallbackProfileLabels(t *testing.T) {
	t.Parallel()

	subscriptions := []domain.Subscription{
		{
			ID:           "sub-1",
			ProviderName: "Durev VPN",
			DisplayName:  "Durev VPN",
		},
		{
			ID:           "sub-2",
			ProviderName: "Durev VPN",
			DisplayName:  "Durev VPN",
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
			ProviderName: "connliberty.com",
			DisplayName:  "connliberty.com",
		},
	}

	groups := buildProviderGroups(subscriptions)
	if len(groups) != 1 {
		t.Fatalf("expected 1 provider group, got %d", len(groups))
	}
	if groups[0].Title != "Liberty VPN" {
		t.Fatalf("unexpected provider title: %q", groups[0].Title)
	}
}

func TestRenderProvidersShowsProviderHierarchy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)
	m := model{
		subscriptions: []domain.Subscription{
			{
				ID:            "sub-lib-main",
				ProviderName:  "Liberty VPN",
				DisplayName:   "Classic VLESS",
				LastUpdatedAt: now,
				ParserStatus:  "ok",
				Nodes:         []domain.Node{{ID: "node-1"}, {ID: "node-2"}},
			},
			{
				ID:            "sub-lib-bypass",
				ProviderName:  "Liberty VPN",
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
		"Liberty VPN",
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
			ServerName: "rbc.ru",
		})
	}

	m := newModel(nil)
	m.height = 18
	m.status.State.Connected = true
	m.status.State.Mode = domain.SelectionModeManual
	m.subscriptions = []domain.Subscription{
		{
			ID:           "sub-lib-main",
			ProviderName: "connliberty.com",
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

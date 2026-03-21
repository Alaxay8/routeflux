package tui

import (
	"fmt"
	"strings"
)

func renderProviders(m model) string {
	if len(m.providers) == 0 {
		return "VPN Services\n\nNo VPN services imported yet."
	}

	var lines []string
	lines = append(lines, paneHeader(m, "VPN Services", m.focus == focusProviders))
	for idx, provider := range m.providers {
		marker := " "
		if idx == m.selectedProvider {
			marker = ">"
		}

		active := ""
		if m.status.ActiveSubscription != nil && strings.EqualFold(providerTitle(*m.status.ActiveSubscription), provider.Title) {
			active = " active"
		}

		profileCount := len(provider.Subscriptions)
		profileWord := "profile"
		if profileCount != 1 {
			profileWord = "profiles"
		}

		lines = append(lines, fmt.Sprintf("%s %s (%d %s, %d nodes)%s", marker, provider.Title, profileCount, profileWord, provider.TotalNodes, active))
	}

	return strings.Join(lines, "\n")
}

func renderProfiles(m model) string {
	provider, ok := m.currentProvider()
	if !ok {
		return "Profiles\n\nImport a VPN service to see profiles."
	}

	var lines []string
	lines = append(lines, paneHeader(m, "Profiles", m.focus == focusProfiles))
	for idx, profile := range provider.Subscriptions {
		marker := " "
		if idx == m.selectedProfile {
			marker = ">"
		}

		active := ""
		if m.status.State.ActiveSubscriptionID == profile.Subscription.ID {
			active = " active"
		}
		lines = append(lines, fmt.Sprintf("%s %s (%d nodes)%s", marker, profile.Label, len(profile.Subscription.Nodes), active))
		lines = append(lines, m.mutedStyle.Render(fmt.Sprintf("  id=%s  updated=%s  status=%s", profile.Subscription.ID, profile.Subscription.LastUpdatedAt.Format("2006-01-02 15:04:05"), profile.Subscription.ParserStatus)))
	}

	return strings.Join(lines, "\n")
}

func paneHeader(m model, title string, focused bool) string {
	if focused {
		return m.activeStyle.Render(title + " [focus]")
	}
	return m.headerStyle.Render(title)
}

func (m *model) ensureSelection() {
	if len(m.providers) == 0 {
		m.selectedProvider = 0
		m.selectedProfile = 0
		m.selectedNode = 0
		return
	}
	if m.selectedProvider >= len(m.providers) {
		m.selectedProvider = len(m.providers) - 1
	}
	provider := m.providers[m.selectedProvider]
	if len(provider.Subscriptions) == 0 {
		m.selectedProfile = 0
		m.selectedNode = 0
		return
	}
	if m.selectedProfile >= len(provider.Subscriptions) {
		m.selectedProfile = len(provider.Subscriptions) - 1
	}
	sub := provider.Subscriptions[m.selectedProfile].Subscription
	if len(sub.Nodes) == 0 {
		m.selectedNode = 0
		return
	}
	if m.selectedNode >= len(sub.Nodes) {
		m.selectedNode = len(sub.Nodes) - 1
	}
}

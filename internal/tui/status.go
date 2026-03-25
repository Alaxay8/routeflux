package tui

import "fmt"

func renderStatus(m model) string {
	mode := string(m.status.State.Mode)
	if mode == "" {
		mode = "disconnected"
	}

	activeProvider := "none"
	if m.status.ActiveSubscription != nil {
		activeProvider = providerTitle(*m.status.ActiveSubscription)
	}

	activeProfile := "none"
	if m.status.ActiveSubscription != nil {
		activeProfile = profileLabel(*m.status.ActiveSubscription)
		if activeProfile == "" {
			activeProfile = "Main profile"
		}
	}

	activeNode := "none"
	if m.status.ActiveNode != nil {
		activeNode = nodeLabel(*m.status.ActiveNode)
	}

	return fmt.Sprintf(
		"%s\nState: %s\nVPN: %s\nProfile: %s\nNode: %s\nMode: %s\nMessage: %s\n\nKeys: tab pane  h/l pane  j/k move  n/p location  c connect  a auto  d disconnect  r refresh  s settings  q quit",
		m.headerStyle.Render("RouteFlux"),
		connectionState(m.status.State.Connected, mode),
		activeProvider,
		activeProfile,
		activeNode,
		mode,
		m.message,
	)
}

func connectionState(connected bool, mode string) string {
	if !connected {
		return "Disconnected"
	}
	if mode == "auto" {
		return "Auto"
	}
	return "Connected"
}

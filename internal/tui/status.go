package tui

import "fmt"

func renderStatus(m model) string {
	mode := string(m.status.State.Mode)
	if mode == "" {
		mode = "disconnected"
	}

	activeSubscription := "none"
	if m.status.ActiveSubscription != nil {
		activeSubscription = m.status.ActiveSubscription.DisplayName
	}

	activeNode := "none"
	if m.status.ActiveNode != nil {
		activeNode = m.status.ActiveNode.DisplayName()
	}

	return fmt.Sprintf(
		"%s\nState: %s\nSubscription: %s\nNode: %s\nMode: %s\nMessage: %s\n\nKeys: j/k subscription  h/l node  c connect  a auto  d disconnect  r refresh  s settings  q quit",
		m.headerStyle.Render("RouteFlux"),
		connectionState(m.status.State.Connected, mode),
		activeSubscription,
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

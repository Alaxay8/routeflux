package tui

import (
	"fmt"
	"strings"
)

func renderSubscriptions(m model) string {
	if len(m.subscriptions) == 0 {
		return "Subscriptions\n\nNo subscriptions imported yet."
	}

	var lines []string
	lines = append(lines, m.headerStyle.Render("Subscriptions"))
	for idx, sub := range m.subscriptions {
		marker := " "
		if idx == m.selectedSub {
			marker = ">"
		}
		active := ""
		if m.status.State.ActiveSubscriptionID == sub.ID {
			active = " active"
		}
		lines = append(lines, fmt.Sprintf("%s %s (%d nodes)%s", marker, sub.DisplayName, len(sub.Nodes), active))
		lines = append(lines, m.mutedStyle.Render(fmt.Sprintf("  id=%s  updated=%s  status=%s", sub.ID, sub.LastUpdatedAt.Format("2006-01-02 15:04:05"), sub.ParserStatus)))
	}

	return strings.Join(lines, "\n")
}

func (m *model) ensureSelection() {
	if len(m.subscriptions) == 0 {
		m.selectedSub = 0
		m.selectedNode = 0
		return
	}
	if m.selectedSub >= len(m.subscriptions) {
		m.selectedSub = len(m.subscriptions) - 1
	}
	sub := m.subscriptions[m.selectedSub]
	if len(sub.Nodes) == 0 {
		m.selectedNode = 0
		return
	}
	if m.selectedNode >= len(sub.Nodes) {
		m.selectedNode = len(sub.Nodes) - 1
	}
}

package tui

import (
	"fmt"
	"strings"
)

func renderNodes(m model) string {
	if len(m.subscriptions) == 0 {
		return "Nodes\n\nImport a subscription to see nodes."
	}

	sub := m.subscriptions[m.selectedSub]
	if len(sub.Nodes) == 0 {
		return "Nodes\n\nNo nodes in selected subscription."
	}

	var lines []string
	lines = append(lines, m.headerStyle.Render("Nodes"))
	for idx, node := range sub.Nodes {
		marker := " "
		if idx == m.selectedNode {
			marker = ">"
		}

		latencyText := "n/a"
		if health, ok := m.status.State.Health[node.ID]; ok && health.LastLatency.Duration() > 0 {
			latencyText = health.LastLatency.String()
		}

		active := ""
		if m.status.State.ActiveNodeID == node.ID {
			active = " active"
		}

		lines = append(lines, fmt.Sprintf("%s %s  %s  %s  latency=%s%s", marker, node.DisplayName(), node.Protocol, node.Transport, latencyText, active))
		lines = append(lines, m.mutedStyle.Render(fmt.Sprintf("  %s:%d  security=%s sni=%s", node.Address, node.Port, node.Security, node.ServerName)))
	}

	return strings.Join(lines, "\n")
}

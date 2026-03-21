package tui

import (
	"fmt"
	"strings"
)

func renderNodes(m model, maxLines int) string {
	sub, ok := m.currentSubscription()
	if !ok {
		return "Locations\n\nSelect a VPN profile to see locations."
	}

	if len(sub.Nodes) == 0 {
		return "Locations\n\nNo nodes in selected profile."
	}

	var lines []string
	lines = append(lines, paneHeader(m, "Locations", m.focus == focusNodes))

	maxItems := (maxLines - 3) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start, end := visibleWindow(len(sub.Nodes), m.selectedNode, maxItems)
	if start > 0 {
		lines = append(lines, m.mutedStyle.Render("  ↑ more"))
	}

	for idx := start; idx < end; idx++ {
		node := sub.Nodes[idx]
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
	if end < len(sub.Nodes) {
		lines = append(lines, m.mutedStyle.Render("  ↓ more"))
	}

	return strings.Join(lines, "\n")
}

func visibleWindow(total, selected, limit int) (int, int) {
	if total <= 0 || limit <= 0 || total <= limit {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	start := selected - (limit / 2)
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > total {
		end = total
		start = end - limit
	}
	return start, end
}

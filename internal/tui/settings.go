package tui

import (
	"fmt"
	"time"
)

var refreshPresets = []time.Duration{
	15 * time.Minute,
	30 * time.Minute,
	time.Hour,
	6 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

func renderSettings(m model) string {
	return fmt.Sprintf(
		"%s\n\nRefresh interval: %s\nHealth check: %s\nSwitch cooldown: %s\nLatency threshold: %s\nLog level: %s\n\nUse +/- to change refresh interval. Press s to return.",
		m.headerStyle.Render("Settings"),
		m.settings.RefreshInterval,
		m.settings.HealthCheckInterval,
		m.settings.SwitchCooldown,
		m.settings.LatencyThreshold,
		m.settings.LogLevel,
	)
}

func nextRefreshPreset(current time.Duration, delta int) string {
	idx := 0
	for i, preset := range refreshPresets {
		if preset == current {
			idx = i
			break
		}
	}

	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(refreshPresets) {
		idx = len(refreshPresets) - 1
	}

	return refreshPresets[idx].String()
}

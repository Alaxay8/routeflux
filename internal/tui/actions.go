package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", "l", "right":
		m.focus = nextFocus(m.focus, 1)
		return m, nil
	case "shift+tab", "h", "left":
		m.focus = nextFocus(m.focus, -1)
		return m, nil
	case "j", "down":
		m.moveSelection(1)
		return m, nil
	case "k", "up":
		m.moveSelection(-1)
		return m, nil
	case "n", "ctrl+n":
		if sub, ok := m.currentSubscription(); ok && m.selectedNode < len(sub.Nodes)-1 {
			m.selectedNode++
		}
		return m, nil
	case "p", "ctrl+p":
		if m.selectedNode > 0 {
			m.selectedNode--
		}
		return m, nil
	case "s":
		if m.screen == screenMain {
			m.screen = screenSettings
		} else {
			m.screen = screenMain
		}
		return m, nil
	case "+", "=":
		if m.screen == screenSettings {
			next := nextRefreshPreset(m.settings.RefreshInterval.Duration(), 1)
			return m, action(func(ctx context.Context) error {
				_, err := m.service.SetSetting("refresh-interval", next)
				return err
			}, "Updated refresh interval")
		}
		return m, nil
	case "-":
		if m.screen == screenSettings {
			next := nextRefreshPreset(m.settings.RefreshInterval.Duration(), -1)
			return m, action(func(ctx context.Context) error {
				_, err := m.service.SetSetting("refresh-interval", next)
				return err
			}, "Updated refresh interval")
		}
		return m, nil
	case "r":
		sub, ok := m.currentSubscription()
		if !ok {
			return m, nil
		}
		return m, action(func(ctx context.Context) error {
			_, err := m.service.RefreshSubscription(ctx, sub.ID)
			return err
		}, "Subscription refreshed")
	case "c", "enter":
		sub, ok := m.currentSubscription()
		if !ok {
			return m, nil
		}
		node, ok := m.currentNode()
		if !ok {
			return m, nil
		}
		return m, action(func(ctx context.Context) error {
			return m.service.ConnectManual(ctx, sub.ID, node.ID)
		}, "Connected")
	case "a":
		sub, ok := m.currentSubscription()
		if !ok {
			return m, nil
		}
		return m, nodeAction(func(ctx context.Context) (domain.Node, error) {
			return m.service.ConnectAuto(ctx, sub.ID)
		}, "Auto selected")
	case "d":
		return m, action(func(ctx context.Context) error {
			return m.service.Disconnect(ctx)
		}, "Disconnected")
	default:
		return m, nil
	}
}

func nextFocus(current paneFocus, delta int) paneFocus {
	values := []paneFocus{focusProviders, focusProfiles, focusNodes}
	index := 0
	for idx, value := range values {
		if value == current {
			index = idx
			break
		}
	}

	index += delta
	if index < 0 {
		index = len(values) - 1
	}
	if index >= len(values) {
		index = 0
	}
	return values[index]
}

func (m *model) moveSelection(delta int) {
	switch m.focus {
	case focusProviders:
		if len(m.providers) == 0 {
			return
		}
		next := m.selectedProvider + delta
		if next < 0 || next >= len(m.providers) {
			return
		}
		m.selectedProvider = next
		m.selectedProfile = 0
		m.selectedNode = 0
	case focusProfiles:
		provider, ok := m.currentProvider()
		if !ok {
			return
		}
		next := m.selectedProfile + delta
		if next < 0 || next >= len(provider.Subscriptions) {
			return
		}
		m.selectedProfile = next
		m.selectedNode = 0
	case focusNodes:
		sub, ok := m.currentSubscription()
		if !ok {
			return
		}
		next := m.selectedNode + delta
		if next < 0 || next >= len(sub.Nodes) {
			return
		}
		m.selectedNode = next
	}
}

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
	case "j", "down":
		if len(m.subscriptions) > 0 && m.selectedSub < len(m.subscriptions)-1 {
			m.selectedSub++
			m.selectedNode = 0
		}
		return m, nil
	case "k", "up":
		if m.selectedSub > 0 {
			m.selectedSub--
			m.selectedNode = 0
		}
		return m, nil
	case "l", "right":
		if len(m.subscriptions) > 0 {
			sub := m.subscriptions[m.selectedSub]
			if m.selectedNode < len(sub.Nodes)-1 {
				m.selectedNode++
			}
		}
		return m, nil
	case "h", "left":
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
		if len(m.subscriptions) == 0 {
			return m, nil
		}
		sub := m.subscriptions[m.selectedSub]
		return m, action(func(ctx context.Context) error {
			_, err := m.service.RefreshSubscription(ctx, sub.ID)
			return err
		}, "Subscription refreshed")
	case "c", "enter":
		if len(m.subscriptions) == 0 {
			return m, nil
		}
		sub := m.subscriptions[m.selectedSub]
		if len(sub.Nodes) == 0 {
			return m, nil
		}
		node := sub.Nodes[m.selectedNode]
		return m, action(func(ctx context.Context) error {
			return m.service.ConnectManual(ctx, sub.ID, node.ID)
		}, "Connected")
	case "a":
		if len(m.subscriptions) == 0 {
			return m, nil
		}
		sub := m.subscriptions[m.selectedSub]
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

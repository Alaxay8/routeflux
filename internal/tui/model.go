package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
)

type model struct {
	service          *app.Service
	status           app.StatusSnapshot
	subscriptions    []domain.Subscription
	providers        []providerGroup
	settings         domain.Settings
	selectedProvider int
	selectedProfile  int
	selectedNode     int
	focus            paneFocus
	width            int
	height           int
	screen           screen
	message          string
	err              error
	headerStyle      lipgloss.Style
	mutedStyle       lipgloss.Style
	activeStyle      lipgloss.Style
}

type paneFocus int

const (
	focusProviders paneFocus = iota
	focusProfiles
	focusNodes
)

type loadMsg struct {
	status        app.StatusSnapshot
	subscriptions []domain.Subscription
	settings      domain.Settings
	err           error
}

type actionMsg struct {
	message string
	err     error
}

// Run launches the Bubble Tea UI.
func Run(service *app.Service) error {
	program := tea.NewProgram(newModel(service), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func newModel(service *app.Service) model {
	return model{
		service:     service,
		screen:      screenMain,
		focus:       focusNodes,
		width:       100,
		height:      24,
		headerStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")),
		mutedStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		activeStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		message:     "Loading subscriptions...",
	}
}

func (m model) Init() tea.Cmd {
	return loadCmd(m.service)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case loadMsg:
		m.status = msg.status
		m.subscriptions = msg.subscriptions
		m.providers = buildProviderGroups(msg.subscriptions)
		m.settings = msg.settings
		m.err = msg.err
		if msg.err == nil {
			m.message = "Ready"
		}
		m.ensureSelection()
		return m, nil
	case actionMsg:
		m.err = msg.err
		if msg.err == nil && msg.message != "" {
			m.message = msg.message
		}
		return m, loadCmd(m.service)
	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.err != nil {
		return renderStatus(m) + "\n\n" + m.err.Error() + "\n"
	}

	switch m.screen {
	case screenSettings:
		return renderStatus(m) + "\n\n" + renderSettings(m) + "\n"
	default:
		top := renderStatus(m) + "\n\n" + renderProviders(m) + "\n\n" + renderProfiles(m)
		availableLines := m.height - lipgloss.Height(top) - 2
		if availableLines < 6 {
			availableLines = 6
		}
		return top + "\n\n" + renderNodes(m, availableLines) + "\n"
	}
}

func loadCmd(service *app.Service) tea.Cmd {
	return func() tea.Msg {
		status, err := service.Status()
		if err != nil {
			return loadMsg{err: err}
		}
		subscriptions, err := service.ListSubscriptions()
		if err != nil {
			return loadMsg{err: err}
		}
		settings, err := service.GetSettings()
		if err != nil {
			return loadMsg{err: err}
		}
		return loadMsg{status: status, subscriptions: subscriptions, settings: settings}
	}
}

func action(command func(context.Context) error, success string) tea.Cmd {
	return func() tea.Msg {
		if err := command(context.Background()); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: success}
	}
}

func nodeAction(command func(context.Context) (domain.Node, error), prefix string) tea.Cmd {
	return func() tea.Msg {
		node, err := command(context.Background())
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: fmt.Sprintf("%s %s", prefix, node.DisplayName())}
	}
}

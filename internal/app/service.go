package app

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/parser"
	"github.com/Alaxay8/routeflux/internal/probe"
)

// Store defines the persisted state contract required by the service layer.
type Store interface {
	LoadSubscriptions() ([]domain.Subscription, error)
	SaveSubscriptions([]domain.Subscription) error
	LoadSettings() (domain.Settings, error)
	SaveSettings(domain.Settings) error
	LoadState() (domain.RuntimeState, error)
	SaveState(domain.RuntimeState) error
}

// Firewaller applies OpenWrt destination-based transparent proxy rules.
type Firewaller interface {
	Apply(ctx context.Context, settings domain.FirewallSettings) error
	Disable(ctx context.Context) error
}

// AddSubscriptionRequest defines how a subscription is added.
type AddSubscriptionRequest struct {
	URL  string
	Raw  string
	Name string
}

// StatusSnapshot summarizes the current application state.
type StatusSnapshot struct {
	State              domain.RuntimeState  `json:"state"`
	Settings           domain.Settings      `json:"settings"`
	ActiveSubscription *domain.Subscription `json:"active_subscription,omitempty"`
	ActiveNode         *domain.Node         `json:"active_node,omitempty"`
}

// Service orchestrates subscription, health, and backend workflows.
type Service struct {
	store      Store
	backend    backend.Backend
	firewall   Firewaller
	httpClient *http.Client
	checker    probe.Checker
	logger     *slog.Logger
}

// Dependencies groups the service construction inputs.
type Dependencies struct {
	Store      Store
	Backend    backend.Backend
	Firewaller Firewaller
	HTTPClient *http.Client
	Checker    probe.Checker
	Logger     *slog.Logger
}

type subscriptionFetchMetadata struct {
	ProviderName string
}

type subscriptionFetchResult struct {
	Content  string
	Metadata subscriptionFetchMetadata
}

const (
	subscriptionFetchMaxAttempts = 3
	subscriptionFetchBaseBackoff = 250 * time.Millisecond
	subscriptionFetchUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	subscriptionProfileTitleKey  = "Profile-Title"
)

var subscriptionShareLinkPattern = regexp.MustCompile(`(?i)(vless|vmess|trojan|ss)://[^\s"'<>]+`)
var htmlTitlePattern = regexp.MustCompile(`(?is)<title[^>]*>\s*(.*?)\s*</title>`)
var htmlH1Pattern = regexp.MustCompile(`(?is)<h1[^>]*>\s*(.*?)\s*</h1>`)
var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

// NewService creates an application service with sensible defaults.
func NewService(deps Dependencies) *Service {
	checker := deps.Checker
	if checker == nil {
		checker = probe.TCPChecker{Timeout: 5 * time.Second}
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Service{
		store:      deps.Store,
		backend:    deps.Backend,
		firewall:   deps.Firewaller,
		httpClient: ensureSubscriptionHTTPClient(deps.HTTPClient),
		checker:    checker,
		logger:     logger,
	}
}

// AddSubscription adds a new subscription and parses its nodes.
func (s *Service) AddSubscription(ctx context.Context, req AddSubscriptionRequest) (domain.Subscription, error) {
	return runStoreWriteLockedResult(s, func() (domain.Subscription, error) {
		return s.addSubscription(ctx, req)
	})
}

func (s *Service) addSubscription(ctx context.Context, req AddSubscriptionRequest) (domain.Subscription, error) {
	if s.store == nil {
		return domain.Subscription{}, fmt.Errorf("store is not configured")
	}

	source, sourceType, metadata, err := s.resolveSubscriptionSource(ctx, req)
	if err != nil {
		return domain.Subscription{}, err
	}

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("load settings: %w", err)
	}

	providerName, providerNameSource := resolveProviderName(req.Name, sourceType, req.URL, metadata)

	nodes, err := parser.ParseNodes(source, providerName)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("parse subscription: %w", err)
	}

	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("load subscriptions: %w", err)
	}

	now := time.Now().UTC()
	storedSource := sourceOrURL(sourceType, req)
	sub := domain.Subscription{
		ID:                 stableSubscriptionID(sourceType, storedSource),
		SourceType:         sourceType,
		Source:             storedSource,
		ProviderName:       providerName,
		ProviderNameSource: providerNameSource,
		DisplayName:        providerName,
		LastUpdatedAt:      now,
		RefreshInterval:    settings.RefreshInterval,
		ParserStatus:       "ok",
		Nodes:              nodes,
	}

	for idx := range sub.Nodes {
		sub.Nodes[idx].SubscriptionID = sub.ID
	}

	upserted := upsertSubscription(subscriptions, sub)
	if err := s.store.SaveSubscriptions(upserted); err != nil {
		return domain.Subscription{}, fmt.Errorf("save subscriptions: %w", err)
	}

	state, err := s.store.LoadState()
	if err == nil {
		if state.LastRefreshAt == nil {
			state.LastRefreshAt = make(map[string]time.Time)
		}
		state.LastRefreshAt[sub.ID] = now
		_ = s.store.SaveState(state)
	}

	return sub, nil
}

// RemoveSubscription removes a stored subscription and disconnects if it was active.
func (s *Service) RemoveSubscription(ctx context.Context, id string) error {
	return runStoreWriteLocked(s, func() error {
		return s.removeSubscription(ctx, id)
	})
}

func (s *Service) removeSubscription(ctx context.Context, id string) error {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return fmt.Errorf("load subscriptions: %w", err)
	}

	idx := slices.IndexFunc(subscriptions, func(sub domain.Subscription) bool { return sub.ID == id })
	if idx < 0 {
		return fmt.Errorf("subscription %q not found", id)
	}

	state, stateErr := s.store.LoadState()
	active := stateErr == nil && state.ActiveSubscriptionID == id
	if active {
		if err := s.disconnectRuntime(ctx); err != nil {
			return err
		}
	}

	subscriptions = append(subscriptions[:idx], subscriptions[idx+1:]...)
	if err := s.store.SaveSubscriptions(subscriptions); err != nil {
		return fmt.Errorf("save subscriptions: %w", err)
	}

	if !active {
		return nil
	}

	return s.persistDisconnectedState(state)
}

// RemoveAllSubscriptions removes all stored subscriptions and disconnects if one is active.
func (s *Service) RemoveAllSubscriptions(ctx context.Context) (int, error) {
	return runStoreWriteLockedResult(s, func() (int, error) {
		return s.removeAllSubscriptions(ctx)
	})
}

func (s *Service) removeAllSubscriptions(ctx context.Context) (int, error) {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return 0, fmt.Errorf("load subscriptions: %w", err)
	}

	removed := len(subscriptions)
	if removed == 0 {
		return 0, nil
	}

	state, stateErr := s.store.LoadState()
	active := stateErr == nil && state.ActiveSubscriptionID != ""
	if active {
		if err := s.disconnectRuntime(ctx); err != nil {
			return 0, err
		}
	}

	if err := s.store.SaveSubscriptions([]domain.Subscription{}); err != nil {
		return 0, fmt.Errorf("save subscriptions: %w", err)
	}

	if !active {
		return removed, nil
	}

	if err := s.persistDisconnectedState(state); err != nil {
		return 0, err
	}

	return removed, nil
}

func (s *Service) disconnectRuntime(ctx context.Context) error {
	if s.backend != nil {
		if err := s.backend.Stop(ctx); err != nil {
			return fmt.Errorf("stop backend: %w", err)
		}
	}
	if s.firewall != nil {
		if err := s.firewall.Disable(ctx); err != nil {
			return fmt.Errorf("disable firewall: %w", err)
		}
	}

	return nil
}

func (s *Service) persistDisconnectedState(state domain.RuntimeState) error {
	state.ActiveSubscriptionID = ""
	state.ActiveNodeID = ""
	state.Mode = domain.SelectionModeDisconnected
	state.Connected = false
	if err := s.store.SaveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = false
		settings.Mode = domain.SelectionModeDisconnected
		_ = s.store.SaveSettings(settings)
	}

	return nil
}

// RenameSubscription updates the display name of a stored subscription.
func (s *Service) RenameSubscription(id, name string) error {
	return runStoreWriteLocked(s, func() error {
		return s.renameSubscription(id, name)
	})
}

func (s *Service) renameSubscription(id, name string) error {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return fmt.Errorf("load subscriptions: %w", err)
	}

	for idx := range subscriptions {
		if subscriptions[idx].ID == id {
			subscriptions[idx].DisplayName = name
			subscriptions[idx].ProviderName = name
			subscriptions[idx].ProviderNameSource = domain.ProviderNameSourceManual
			return s.store.SaveSubscriptions(subscriptions)
		}
	}

	return fmt.Errorf("subscription %q not found", id)
}

// ListSubscriptions returns the stored subscriptions.
func (s *Service) ListSubscriptions() ([]domain.Subscription, error) {
	return s.store.LoadSubscriptions()
}

// ListNodes returns all nodes for a subscription.
func (s *Service) ListNodes(subscriptionID string) ([]domain.Node, error) {
	sub, err := s.subscriptionByID(subscriptionID)
	if err != nil {
		return nil, err
	}

	return sub.Nodes, nil
}

// RefreshSubscription reloads and reparses a subscription.
func (s *Service) RefreshSubscription(ctx context.Context, subscriptionID string) (domain.Subscription, error) {
	return runStoreWriteLockedResult(s, func() (domain.Subscription, error) {
		return s.refreshSubscription(ctx, subscriptionID)
	})
}

func (s *Service) refreshSubscription(ctx context.Context, subscriptionID string) (domain.Subscription, error) {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("load subscriptions: %w", err)
	}

	index := slices.IndexFunc(subscriptions, func(sub domain.Subscription) bool { return sub.ID == subscriptionID })
	if index < 0 {
		return domain.Subscription{}, fmt.Errorf("subscription %q not found", subscriptionID)
	}

	sub := subscriptions[index]
	content := sub.Source
	metadata := subscriptionFetchMetadata{}
	if sub.SourceType == domain.SourceTypeURL {
		result, err := s.fetchSubscription(ctx, sub.Source)
		if err != nil {
			sub.LastError = err.Error()
			sub.ParserStatus = "error"
			subscriptions[index] = sub
			_ = s.store.SaveSubscriptions(subscriptions)
			return domain.Subscription{}, fmt.Errorf("fetch subscription: %w", err)
		}
		content = result.Content
		metadata = result.Metadata
	}

	providerName, displayName, providerNameSource := refreshedProviderIdentity(sub, metadata)
	nodes, err := parser.ParseNodes(content, providerName)
	if err != nil {
		sub.LastError = err.Error()
		sub.ParserStatus = "error"
		subscriptions[index] = sub
		_ = s.store.SaveSubscriptions(subscriptions)
		return domain.Subscription{}, fmt.Errorf("parse subscription: %w", err)
	}

	for idx := range nodes {
		nodes[idx].SubscriptionID = sub.ID
	}

	sub.Nodes = nodes
	sub.ProviderName = providerName
	sub.DisplayName = displayName
	sub.ProviderNameSource = providerNameSource
	sub.LastError = ""
	sub.ParserStatus = "ok"
	sub.LastUpdatedAt = time.Now().UTC()
	subscriptions[index] = sub
	if err := s.store.SaveSubscriptions(subscriptions); err != nil {
		return domain.Subscription{}, fmt.Errorf("save subscriptions: %w", err)
	}

	state, err := s.store.LoadState()
	if err == nil {
		if state.LastRefreshAt == nil {
			state.LastRefreshAt = make(map[string]time.Time)
		}
		state.LastRefreshAt[sub.ID] = sub.LastUpdatedAt
		_ = s.store.SaveState(state)
	}

	return sub, nil
}

// RefreshAll refreshes every stored subscription.
func (s *Service) RefreshAll(ctx context.Context) ([]domain.Subscription, error) {
	return runStoreWriteLockedResult(s, func() ([]domain.Subscription, error) {
		return s.refreshAll(ctx)
	})
}

func (s *Service) refreshAll(ctx context.Context) ([]domain.Subscription, error) {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
	}

	updated := make([]domain.Subscription, 0, len(subscriptions))
	for _, sub := range subscriptions {
		refreshed, err := s.refreshSubscription(ctx, sub.ID)
		if err != nil {
			return updated, err
		}
		updated = append(updated, refreshed)
	}

	return updated, nil
}

// ConnectManual pins a subscription and node and applies the backend config.
func (s *Service) ConnectManual(ctx context.Context, subscriptionID, nodeID string) error {
	return runStoreWriteLocked(s, func() error {
		return s.connectManual(ctx, subscriptionID, nodeID)
	})
}

func (s *Service) connectManual(ctx context.Context, subscriptionID, nodeID string) error {
	sub, err := s.subscriptionByID(subscriptionID)
	if err != nil {
		return err
	}

	node, ok := sub.NodeByID(nodeID)
	if !ok {
		return fmt.Errorf("node %q not found in subscription %q", nodeID, subscriptionID)
	}

	if err := s.applyNodeSelection(ctx, sub, node, domain.SelectionModeManual); err != nil {
		return err
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = false
		settings.Mode = domain.SelectionModeManual
		_ = s.store.SaveSettings(settings)
	}

	return nil
}

// ConnectAuto probes the selected subscription and applies the best available node.
func (s *Service) ConnectAuto(ctx context.Context, subscriptionID string) (domain.Node, error) {
	return runStoreWriteLockedResult(s, func() (domain.Node, error) {
		return s.connectAuto(ctx, subscriptionID)
	})
}

func (s *Service) connectAuto(ctx context.Context, subscriptionID string) (domain.Node, error) {
	sub, err := s.subscriptionByID(subscriptionID)
	if err != nil {
		return domain.Node{}, err
	}

	state, err := s.store.LoadState()
	if err != nil {
		return domain.Node{}, fmt.Errorf("load state: %w", err)
	}

	if state.Health == nil {
		state.Health = make(map[string]domain.NodeHealth)
	}

	results := s.probeSubscription(ctx, sub, state.Health)
	bestNode, bestScore, err := probe.SelectBestNode(sub.Nodes, state.Health, probe.DefaultScoreConfig())
	if err != nil {
		return domain.Node{}, err
	}
	current := state.Health[state.ActiveNodeID]
	candidate := state.Health[bestNode.ID]
	shouldSwitch, _ := probe.ShouldSwitch(current, candidate, time.Now().UTC(), state.LastSwitchAt, probe.DefaultSwitchPolicy())
	if state.ActiveNodeID == "" {
		shouldSwitch = true
	}

	selectedNode := bestNode
	switched := true
	if !shouldSwitch && state.ActiveNodeID != "" {
		if active, ok := sub.NodeByID(state.ActiveNodeID); ok {
			selectedNode = active
			switched = false
		}
	}

	if err := s.applyNodeSelection(ctx, sub, selectedNode, domain.SelectionModeAuto); err != nil {
		return domain.Node{}, err
	}

	state, _ = s.store.LoadState()
	if state.Health == nil {
		state.Health = make(map[string]domain.NodeHealth)
	}
	for _, result := range results {
		if result.Health.NodeID != "" {
			state.Health[result.NodeID] = result.Health
		}
	}
	scored := state.Health[bestNode.ID]
	scored.Score = bestScore.Score
	state.Health[bestNode.ID] = scored
	if switched {
		state.LastSwitchAt = time.Now().UTC()
	}
	state.Mode = domain.SelectionModeAuto
	state.Connected = true
	state.ActiveSubscriptionID = sub.ID
	state.ActiveNodeID = selectedNode.ID
	if err := s.store.SaveState(state); err != nil {
		return domain.Node{}, fmt.Errorf("save state: %w", err)
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = true
		settings.Mode = domain.SelectionModeAuto
		_ = s.store.SaveSettings(settings)
	}

	return selectedNode, nil
}

// Disconnect tears down the current runtime selection.
func (s *Service) Disconnect(ctx context.Context) error {
	return runStoreWriteLocked(s, func() error {
		return s.disconnect(ctx)
	})
}

func (s *Service) disconnect(ctx context.Context) error {
	if err := s.disconnectRuntime(ctx); err != nil {
		return err
	}

	state, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	state.ActiveNodeID = ""
	state.ActiveSubscriptionID = ""
	state.Mode = domain.SelectionModeDisconnected
	state.Connected = false
	if err := s.store.SaveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = false
		settings.Mode = domain.SelectionModeDisconnected
		_ = s.store.SaveSettings(settings)
	}

	return nil
}

// Status returns the current service status.
func (s *Service) Status() (StatusSnapshot, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return StatusSnapshot{}, fmt.Errorf("load settings: %w", err)
	}

	state, err := s.store.LoadState()
	if err != nil {
		return StatusSnapshot{}, fmt.Errorf("load state: %w", err)
	}

	snapshot := StatusSnapshot{
		State:    state,
		Settings: settings,
	}

	if state.ActiveSubscriptionID == "" {
		return snapshot, nil
	}

	sub, err := s.subscriptionByID(state.ActiveSubscriptionID)
	if err == nil {
		snapshot.ActiveSubscription = &sub
		if node, ok := sub.NodeByID(state.ActiveNodeID); ok {
			snapshot.ActiveNode = &node
		}
	}

	return snapshot, nil
}

// RuntimeStatus returns the current backend runtime status, if a backend is configured.
func (s *Service) RuntimeStatus(ctx context.Context) (backend.RuntimeStatus, error) {
	if s.backend == nil {
		return backend.RuntimeStatus{}, nil
	}

	return s.backend.Status(ctx)
}

// GetSettings returns current settings.
func (s *Service) GetSettings() (domain.Settings, error) {
	return runStoreWriteLockedResult(s, func() (domain.Settings, error) {
		return s.getSettings()
	})
}

func (s *Service) getSettings() (domain.Settings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}

	state, err := s.store.LoadState()
	if err != nil {
		return settings, nil
	}

	if state.Connected && syncSettingsToRuntime(&settings, state) {
		if err := s.store.SaveSettings(settings); err != nil {
			return domain.Settings{}, fmt.Errorf("save settings: %w", err)
		}
	}

	return settings, nil
}

// GetFirewallSettings returns the transparent proxy routing settings.
func (s *Service) GetFirewallSettings() (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	return settings.Firewall, nil
}

// ConfigureFirewall updates firewall targets and enabled state.
func (s *Service) ConfigureFirewall(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.configureFirewall(ctx, targets, enabled, port)
	})
}

func (s *Service) configureFirewall(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.Firewall.Enabled = enabled
	settings.Firewall.TargetCIDRs = slices.Clone(targets)
	settings.Firewall.SourceCIDRs = nil
	if port > 0 {
		settings.Firewall.TransparentPort = port
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// ConfigureFirewallHosts routes all TCP traffic from selected client IPs through the transparent proxy.
func (s *Service) ConfigureFirewallHosts(ctx context.Context, sources []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.configureFirewallHosts(ctx, sources, enabled, port)
	})
}

func (s *Service) configureFirewallHosts(ctx context.Context, sources []string, enabled bool, port int) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.Firewall.Enabled = enabled
	settings.Firewall.SourceCIDRs = normalizeFirewallSources(sources)
	settings.Firewall.TargetCIDRs = nil
	if port > 0 {
		settings.Firewall.TransparentPort = port
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// UpdateFirewallPort changes the transparent redirect port and reapplies the active rules.
func (s *Service) UpdateFirewallPort(ctx context.Context, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.updateFirewallPort(ctx, port)
	})
}

func (s *Service) updateFirewallPort(ctx context.Context, port int) (domain.FirewallSettings, error) {
	if port <= 0 {
		return domain.FirewallSettings{}, fmt.Errorf("transparent port must be greater than zero")
	}

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.Firewall.TransparentPort = port
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// UpdateFirewallBlockQUIC enables or disables QUIC blocking for source-host routing.
func (s *Service) UpdateFirewallBlockQUIC(ctx context.Context, enabled bool) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.updateFirewallBlockQUIC(ctx, enabled)
	})
}

func (s *Service) updateFirewallBlockQUIC(ctx context.Context, enabled bool) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.Firewall.BlockQUIC = enabled
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// DisableFirewall disables transparent proxy routing.
func (s *Service) DisableFirewall(ctx context.Context) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.disableFirewall(ctx)
	})
}

func (s *Service) disableFirewall(ctx context.Context) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.Firewall.Enabled = false
	settings.Firewall.TargetCIDRs = nil
	settings.Firewall.SourceCIDRs = nil
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// SetSetting updates a single setting key.
func (s *Service) SetSetting(key, value string) (domain.Settings, error) {
	return runStoreWriteLockedResult(s, func() (domain.Settings, error) {
		return s.setSetting(key, value)
	})
}

func (s *Service) setSetting(key, value string) (domain.Settings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}

	reapplyRuntime := false

	switch key {
	case "refresh-interval":
		d, err := domain.ParseDurationValue(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.RefreshInterval = d
	case "health-check-interval":
		d, err := domain.ParseDurationValue(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.HealthCheckInterval = d
	case "switch-cooldown":
		d, err := domain.ParseDurationValue(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.SwitchCooldown = d
	case "latency-threshold":
		d, err := domain.ParseDurationValue(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.LatencyThreshold = d
	case "auto-mode":
		enableAuto := strings.EqualFold(value, "true")
		state, stateErr := s.store.LoadState()
		if enableAuto {
			if stateErr == nil && state.Connected && state.ActiveSubscriptionID != "" {
				if _, err := s.connectAuto(context.Background(), state.ActiveSubscriptionID); err != nil {
					return domain.Settings{}, err
				}
				return s.store.LoadSettings()
			}

			settings.AutoMode = true
			settings.Mode = domain.SelectionModeAuto
		} else {
			if stateErr == nil &&
				state.Connected &&
				state.Mode == domain.SelectionModeAuto &&
				state.ActiveSubscriptionID != "" &&
				state.ActiveNodeID != "" {
				if err := s.connectManual(context.Background(), state.ActiveSubscriptionID, state.ActiveNodeID); err != nil {
					return domain.Settings{}, err
				}
				return s.store.LoadSettings()
			}

			settings.AutoMode = false
			if stateErr == nil && state.Connected {
				settings.Mode = state.Mode
				if settings.Mode == domain.SelectionModeAuto {
					settings.Mode = domain.SelectionModeManual
				}
			} else if settings.Mode == domain.SelectionModeAuto {
				settings.Mode = domain.SelectionModeManual
			}
		}
	case "log-level":
		settings.LogLevel = value
		reapplyRuntime = true
	case "dns.mode":
		mode, err := domain.ParseDNSMode(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.DNS.Mode = mode
		reapplyRuntime = true
	case "dns.transport":
		transport, err := domain.ParseDNSTransport(value)
		if err != nil {
			return domain.Settings{}, err
		}
		settings.DNS.Transport = transport
		reapplyRuntime = true
	case "dns.servers":
		settings.DNS.Servers = parseStringList(value)
		reapplyRuntime = true
	case "dns.bootstrap":
		settings.DNS.Bootstrap = parseStringList(value)
		reapplyRuntime = true
	case "dns.direct-domains", "dns.domains":
		settings.DNS.DirectDomains = parseStringList(value)
		reapplyRuntime = true
	default:
		return domain.Settings{}, fmt.Errorf("unsupported setting %q", key)
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.Settings{}, fmt.Errorf("save settings: %w", err)
	}

	if reapplyRuntime {
		if err := s.reapplyCurrentConnection(context.Background()); err != nil {
			return domain.Settings{}, err
		}
	}

	return settings, nil
}

// ApplyDefaultDNS replaces current DNS settings with the RouteFlux recommended profile.
func (s *Service) ApplyDefaultDNS(ctx context.Context) (domain.Settings, error) {
	return runStoreWriteLockedResult(s, func() (domain.Settings, error) {
		return s.applyDefaultDNS(ctx)
	})
}

func (s *Service) applyDefaultDNS(ctx context.Context) (domain.Settings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}

	settings.DNS = domain.DefaultDNSSettings()
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.Settings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.Settings{}, err
	}

	return settings, nil
}

func (s *Service) resolveSubscriptionSource(ctx context.Context, req AddSubscriptionRequest) (string, domain.SourceType, subscriptionFetchMetadata, error) {
	switch {
	case strings.TrimSpace(req.URL) != "":
		result, err := s.fetchSubscription(ctx, req.URL)
		if err != nil {
			return "", "", subscriptionFetchMetadata{}, fmt.Errorf("fetch subscription: %w", err)
		}
		return result.Content, domain.SourceTypeURL, result.Metadata, nil
	case strings.TrimSpace(req.Raw) != "":
		return req.Raw, domain.SourceTypeRaw, subscriptionFetchMetadata{}, nil
	default:
		return "", "", subscriptionFetchMetadata{}, fmt.Errorf("either url or raw payload is required")
	}
}

func (s *Service) fetchSubscription(ctx context.Context, rawURL string) (subscriptionFetchResult, error) {
	var lastErr error

	for attempt := 1; attempt <= subscriptionFetchMaxAttempts; attempt++ {
		result, retry, err := s.fetchSubscriptionOnce(ctx, rawURL)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !retry || attempt == subscriptionFetchMaxAttempts {
			break
		}

		delay := subscriptionFetchBaseBackoff * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return subscriptionFetchResult{}, fmt.Errorf("fetch %s: %w", rawURL, ctx.Err())
		case <-timer.C:
		}
	}

	return subscriptionFetchResult{}, lastErr
}

func (s *Service) fetchSubscriptionOnce(ctx context.Context, rawURL string) (subscriptionFetchResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return subscriptionFetchResult{}, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/plain, application/json;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", subscriptionFetchUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return subscriptionFetchResult{}, false, fmt.Errorf("fetch %s: %w", rawURL, ctx.Err())
		}

		return subscriptionFetchResult{}, true, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		message := summarizeSubscriptionResponseBody(body)
		if message != "" {
			return subscriptionFetchResult{}, isTransientSubscriptionStatus(resp.StatusCode), fmt.Errorf("fetch %s: unexpected status %s: %s", rawURL, resp.Status, message)
		}
		return subscriptionFetchResult{}, isTransientSubscriptionStatus(resp.StatusCode), fmt.Errorf("fetch %s: unexpected status %s", rawURL, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return subscriptionFetchResult{}, false, fmt.Errorf("read response body: %w", err)
	}

	content, err := normalizeSubscriptionResponse(rawURL, body)
	if err != nil {
		return subscriptionFetchResult{}, false, err
	}

	return subscriptionFetchResult{
		Content: content,
		Metadata: subscriptionFetchMetadata{
			ProviderName: decodeProfileTitle(resp.Header.Get(subscriptionProfileTitleKey)),
		},
	}, false, nil
}

func isTransientSubscriptionStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func ensureSubscriptionHTTPClient(base *http.Client) *http.Client {
	var client *http.Client
	if base == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	} else {
		copy := *base
		client = &copy
	}

	if client.Jar == nil {
		if jar, err := cookiejar.New(nil); err == nil {
			client.Jar = jar
		}
	}

	return client
}

func normalizeSubscriptionResponse(rawURL string, body []byte) (string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", fmt.Errorf("fetch %s: empty response body", rawURL)
	}

	if isHTMLLike(trimmed) {
		if links := extractSubscriptionShareLinks(trimmed); len(links) > 0 {
			return strings.Join(links, "\n"), nil
		}

		message := summarizeHTMLResponse(trimmed)
		if message == "" {
			message = "endpoint returned HTML page instead of subscription data"
		}
		return "", fmt.Errorf("fetch %s: %s", rawURL, message)
	}

	if message := summarizeJSONEndpointError(trimmed); message != "" {
		return "", fmt.Errorf("fetch %s: %s", rawURL, message)
	}

	return string(body), nil
}

func summarizeSubscriptionResponseBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	if message := summarizeJSONEndpointError(trimmed); message != "" {
		return message
	}

	if message := summarizeHTMLResponse(trimmed); message != "" {
		return message
	}

	line := strings.TrimSpace(strings.SplitN(trimmed, "\n", 2)[0])
	if len(line) > 160 {
		line = line[:160] + "..."
	}
	return line
}

func summarizeJSONEndpointError(trimmed string) string {
	if !json.Valid([]byte(trimmed)) || !strings.HasPrefix(trimmed, "{") {
		return ""
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return ""
	}

	for _, key := range []string{"outbounds", "protocol", "config", "link"} {
		if _, ok := payload[key]; ok {
			return ""
		}
	}

	code := jsonStringField(payload, "error")
	info := firstNonEmpty(
		jsonStringField(payload, "info"),
		jsonStringField(payload, "message"),
		jsonStringField(payload, "detail"),
	)

	switch {
	case code != "" && info != "":
		return fmt.Sprintf("subscription endpoint error %s: %s", code, info)
	case code != "":
		return fmt.Sprintf("subscription endpoint error %s", code)
	case info != "":
		return fmt.Sprintf("subscription endpoint error: %s", info)
	default:
		return ""
	}
}

func jsonStringField(payload map[string]json.RawMessage, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}

	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return ""
	}

	return strings.TrimSpace(text)
}

func isHTMLLike(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		(strings.Contains(lower, "<body") && strings.Contains(lower, "</html>"))
}

func summarizeHTMLResponse(trimmed string) string {
	if !isHTMLLike(trimmed) {
		return ""
	}

	for _, pattern := range []*regexp.Regexp{htmlH1Pattern, htmlTitlePattern} {
		matches := pattern.FindStringSubmatch(trimmed)
		if len(matches) < 2 {
			continue
		}

		text := cleanHTMLSnippet(matches[1])
		if text != "" {
			return text
		}
	}

	text := cleanHTMLSnippet(trimmed)
	if len(text) > 160 {
		text = text[:160] + "..."
	}

	return text
}

func cleanHTMLSnippet(value string) string {
	value = html.UnescapeString(value)
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func extractSubscriptionShareLinks(value string) []string {
	matches := subscriptionShareLinkPattern.FindAllString(value, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		link := html.UnescapeString(strings.TrimSpace(match))
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}

	return out
}

func (s *Service) subscriptionByID(id string) (domain.Subscription, error) {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("load subscriptions: %w", err)
	}

	for _, sub := range subscriptions {
		if sub.ID == id {
			return sub, nil
		}
	}

	return domain.Subscription{}, fmt.Errorf("subscription %q not found", id)
}

func (s *Service) applyNodeSelection(ctx context.Context, sub domain.Subscription, node domain.Node, mode domain.SelectionMode) error {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	if s.backend != nil {
		if err := s.backend.ApplyConfig(ctx, backend.ConfigRequest{
			Mode:             mode,
			Nodes:            []domain.Node{node},
			SelectedNodeID:   node.ID,
			LogLevel:         settings.LogLevel,
			DNS:              settings.DNS,
			SOCKSPort:        10808,
			HTTPPort:         10809,
			TransparentProxy: settings.Firewall.Enabled && (len(settings.Firewall.TargetCIDRs) > 0 || len(settings.Firewall.SourceCIDRs) > 0),
			TransparentPort:  settings.Firewall.TransparentPort,
		}); err != nil {
			_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("apply backend config: %v", err))
			return fmt.Errorf("apply backend config: %w", err)
		}
		if err := s.ensureBackendRunning(ctx, sub.ID, node.ID, mode); err != nil {
			return err
		}
	}

	if s.firewall != nil {
		if settings.Firewall.Enabled && (len(settings.Firewall.TargetCIDRs) > 0 || len(settings.Firewall.SourceCIDRs) > 0) {
			if err := s.firewall.Apply(ctx, settings.Firewall); err != nil {
				_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("apply firewall: %v", err))
				return fmt.Errorf("apply firewall: %w", err)
			}
		} else if err := s.firewall.Disable(ctx); err != nil {
			_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("disable firewall: %v", err))
			return fmt.Errorf("disable firewall: %w", err)
		}
	}

	state, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	state.ActiveSubscriptionID = sub.ID
	state.ActiveNodeID = node.ID
	state.Mode = mode
	state.Connected = true
	state.LastSuccessAt = time.Now().UTC()
	state.LastFailureReason = ""
	if mode == domain.SelectionModeAuto {
		state.LastSwitchAt = time.Now().UTC()
	}

	if err := s.store.SaveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

func (s *Service) probeSubscription(ctx context.Context, sub domain.Subscription, health map[string]domain.NodeHealth) []probe.Result {
	results := make([]probe.Result, 0, len(sub.Nodes))
	for _, node := range sub.Nodes {
		result := s.checker.Check(ctx, node)
		updated := probe.UpdateHealth(health[node.ID], result.Healthy, result.Latency, result.Checked, errString(result.Err))
		updated.NodeID = node.ID
		updated.Score = probe.CalculateScore(updated, probe.DefaultScoreConfig()).Score
		health[node.ID] = updated
		result.Health = updated
		results = append(results, result)
	}

	return results
}

func stableSubscriptionID(sourceType domain.SourceType, source string) string {
	sum := sha1.Sum([]byte(string(sourceType) + "|" + source))
	return "sub-" + hex.EncodeToString(sum[:])[:10]
}

func sourceOrURL(sourceType domain.SourceType, req AddSubscriptionRequest) string {
	if sourceType == domain.SourceTypeURL {
		return strings.TrimSpace(req.URL)
	}
	return req.Raw
}

func resolveProviderName(reqName string, sourceType domain.SourceType, rawURL string, metadata subscriptionFetchMetadata) (string, domain.ProviderNameSource) {
	if name := strings.TrimSpace(reqName); name != "" {
		return name, domain.ProviderNameSourceManual
	}
	if name := strings.TrimSpace(metadata.ProviderName); name != "" {
		return name, domain.ProviderNameSourceHeader
	}
	if sourceType == domain.SourceTypeURL {
		return deriveProviderName(sourceType, rawURL), domain.ProviderNameSourceURL
	}
	return "Imported Subscription", domain.ProviderNameSourceDefault
}

func deriveProviderName(sourceType domain.SourceType, rawURL string) string {
	if sourceType == domain.SourceTypeURL {
		return domain.ProviderNameFromURL(rawURL)
	}

	return "Imported Subscription"
}

func refreshedProviderIdentity(sub domain.Subscription, metadata subscriptionFetchMetadata) (string, string, domain.ProviderNameSource) {
	currentName := firstNonEmpty(sub.DisplayName, sub.ProviderName)
	if canUpgradeLegacyProviderName(sub, currentName) {
		name, resolvedSource := resolveProviderName("", sub.SourceType, sub.Source, metadata)
		return name, name, resolvedSource
	}

	source := effectiveProviderNameSource(sub)
	if source == domain.ProviderNameSourceManual {
		return currentName, currentName, source
	}
	if name := strings.TrimSpace(metadata.ProviderName); name != "" {
		return name, name, domain.ProviderNameSourceHeader
	}
	if currentName != "" {
		return currentName, currentName, source
	}
	name, resolvedSource := resolveProviderName("", sub.SourceType, sub.Source, metadata)
	return name, name, resolvedSource
}

func effectiveProviderNameSource(sub domain.Subscription) domain.ProviderNameSource {
	if sub.ProviderNameSource != "" {
		return sub.ProviderNameSource
	}

	providerName := strings.TrimSpace(sub.ProviderName)
	displayName := strings.TrimSpace(sub.DisplayName)
	switch {
	case providerName == "" && displayName == "":
		return domain.ProviderNameSourceDefault
	case providerName == "":
		providerName = displayName
	case displayName == "":
		displayName = providerName
	}

	if providerName != displayName {
		return domain.ProviderNameSourceManual
	}

	derivedName := deriveProviderName(sub.SourceType, sub.Source)
	switch {
	case sub.SourceType == domain.SourceTypeURL && providerName == derivedName:
		return domain.ProviderNameSourceURL
	case providerName == "Imported Subscription":
		return domain.ProviderNameSourceDefault
	default:
		return domain.ProviderNameSourceManual
	}
}

func canUpgradeLegacyProviderName(sub domain.Subscription, currentName string) bool {
	if strings.TrimSpace(currentName) == "" {
		return true
	}
	if sub.ProviderNameSource != "" {
		return false
	}

	normalizedCurrent := normalizeProviderNameToken(currentName)
	for _, candidate := range legacyAutoProviderNameCandidates(sub) {
		if normalizeProviderNameToken(candidate) == normalizedCurrent {
			return true
		}
	}

	return false
}

func legacyAutoProviderNameCandidates(sub domain.Subscription) []string {
	candidates := make([]string, 0, 4)
	push := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		candidates = append(candidates, value)
	}

	push(deriveProviderName(sub.SourceType, sub.Source))
	if sub.SourceType != domain.SourceTypeURL {
		push("Imported Subscription")
		return candidates
	}

	host := subscriptionURLHost(sub.Source)
	push(host)
	if host == "" {
		return candidates
	}

	parts := strings.Split(strings.ToLower(host), ".")
	if len(parts) > 0 && parts[0] != "" {
		push(humanizeLegacyHostLabel(parts[0]))
	}

	return candidates
}

func subscriptionURLHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
}

func humanizeLegacyHostLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	label = strings.NewReplacer("-", " ", "_", " ").Replace(label)
	parts := strings.Fields(label)
	for idx, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[idx] = string(runes)
	}
	result := strings.Join(parts, " ")
	if result == "" {
		return ""
	}
	if !strings.Contains(strings.ToLower(result), "vpn") {
		result += " VPN"
	}
	return result
}

func normalizeProviderNameToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func decodeProfileTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if !strings.HasPrefix(strings.ToLower(value), "base64:") {
		return value
	}

	encoded := strings.TrimSpace(value[len("base64:"):])
	if encoded == "" {
		return ""
	}

	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		decoded, err := encoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		if title := strings.TrimSpace(string(decoded)); title != "" {
			return title
		}
	}

	return ""
}

func upsertSubscription(subscriptions []domain.Subscription, next domain.Subscription) []domain.Subscription {
	for idx := range subscriptions {
		if subscriptions[idx].ID == next.ID {
			subscriptions[idx] = next
			return subscriptions
		}
	}

	return append(subscriptions, next)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func parseStringList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func normalizeFirewallSources(sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if strings.EqualFold(source, "all") || source == "*" {
			return []string{"all"}
		}
		out = append(out, source)
	}
	return out
}

func syncSettingsToRuntime(settings *domain.Settings, state domain.RuntimeState) bool {
	expectedMode := state.Mode
	if expectedMode == "" {
		expectedMode = domain.SelectionModeDisconnected
	}
	expectedAuto := expectedMode == domain.SelectionModeAuto

	changed := settings.Mode != expectedMode || settings.AutoMode != expectedAuto
	settings.Mode = expectedMode
	settings.AutoMode = expectedAuto
	return changed
}

func (s *Service) reapplyCurrentConnection(ctx context.Context) error {
	state, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if !state.Connected || state.ActiveSubscriptionID == "" || state.ActiveNodeID == "" {
		if s.firewall != nil {
			if err := s.firewall.Disable(ctx); err != nil {
				return fmt.Errorf("disable firewall: %w", err)
			}
		}
		return nil
	}

	sub, err := s.subscriptionByID(state.ActiveSubscriptionID)
	if err != nil {
		return err
	}

	node, ok := sub.NodeByID(state.ActiveNodeID)
	if !ok {
		return fmt.Errorf("node %q not found in subscription %q", state.ActiveNodeID, state.ActiveSubscriptionID)
	}

	return s.applyNodeSelection(ctx, sub, node, state.Mode)
}

func (s *Service) ensureBackendRunning(ctx context.Context, subscriptionID, nodeID string, mode domain.SelectionMode) error {
	if s.backend == nil {
		return nil
	}

	status, err := s.backend.Status(ctx)
	if err != nil {
		_ = s.markConnectionFailed(ctx, subscriptionID, nodeID, mode, fmt.Sprintf("backend status check failed: %v", err))
		return fmt.Errorf("check backend status: %w", err)
	}
	if status.Running {
		return nil
	}

	reason := "backend is not running"
	if strings.TrimSpace(status.ServiceState) != "" && status.ServiceState != "unknown" {
		reason = fmt.Sprintf("backend is not running (%s)", status.ServiceState)
	}
	_ = s.markConnectionFailed(ctx, subscriptionID, nodeID, mode, reason)
	return fmt.Errorf(reason)
}

func (s *Service) markConnectionFailed(ctx context.Context, subscriptionID, nodeID string, mode domain.SelectionMode, reason string) error {
	if s.firewall != nil {
		if err := s.firewall.Disable(ctx); err != nil {
			return fmt.Errorf("disable firewall after backend failure: %w", err)
		}
	}

	state, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state after backend failure: %w", err)
	}

	state.ActiveSubscriptionID = subscriptionID
	state.ActiveNodeID = nodeID
	state.Mode = mode
	state.Connected = false
	state.LastFailureReason = reason

	if err := s.store.SaveState(state); err != nil {
		return fmt.Errorf("save state after backend failure: %w", err)
	}

	return nil
}

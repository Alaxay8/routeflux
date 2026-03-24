package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

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

// NewService creates an application service with sensible defaults.
func NewService(deps Dependencies) *Service {
	client := deps.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

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
		httpClient: client,
		checker:    checker,
		logger:     logger,
	}
}

// AddSubscription adds a new subscription and parses its nodes.
func (s *Service) AddSubscription(ctx context.Context, req AddSubscriptionRequest) (domain.Subscription, error) {
	if s.store == nil {
		return domain.Subscription{}, fmt.Errorf("store is not configured")
	}

	source, sourceType, err := s.resolveSubscriptionSource(ctx, req)
	if err != nil {
		return domain.Subscription{}, err
	}

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("load settings: %w", err)
	}

	providerName := strings.TrimSpace(req.Name)
	if providerName == "" {
		providerName = deriveProviderName(sourceType, req.URL)
	}

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
		ID:              stableSubscriptionID(sourceType, storedSource),
		SourceType:      sourceType,
		Source:          storedSource,
		ProviderName:    providerName,
		DisplayName:     providerName,
		LastUpdatedAt:   now,
		RefreshInterval: settings.RefreshInterval,
		ParserStatus:    "ok",
		Nodes:           nodes,
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
		state.LastRefreshAt[sub.ID] = now
		_ = s.store.SaveState(state)
	}

	return sub, nil
}

// RemoveSubscription removes a stored subscription and disconnects if it was active.
func (s *Service) RemoveSubscription(ctx context.Context, id string) error {
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
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return fmt.Errorf("load subscriptions: %w", err)
	}

	for idx := range subscriptions {
		if subscriptions[idx].ID == id {
			subscriptions[idx].DisplayName = name
			subscriptions[idx].ProviderName = name
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
	if sub.SourceType == domain.SourceTypeURL {
		content, err = s.fetchSubscription(ctx, sub.Source)
		if err != nil {
			sub.LastError = err.Error()
			sub.ParserStatus = "error"
			subscriptions[index] = sub
			_ = s.store.SaveSubscriptions(subscriptions)
			return domain.Subscription{}, fmt.Errorf("fetch subscription: %w", err)
		}
	}

	nodes, err := parser.ParseNodes(content, sub.ProviderName)
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
	sub.LastError = ""
	sub.ParserStatus = "ok"
	sub.LastUpdatedAt = time.Now().UTC()
	subscriptions[index] = sub
	if err := s.store.SaveSubscriptions(subscriptions); err != nil {
		return domain.Subscription{}, fmt.Errorf("save subscriptions: %w", err)
	}

	state, err := s.store.LoadState()
	if err == nil {
		state.LastRefreshAt[sub.ID] = sub.LastUpdatedAt
		_ = s.store.SaveState(state)
	}

	return sub, nil
}

// RefreshAll refreshes every stored subscription.
func (s *Service) RefreshAll(ctx context.Context) ([]domain.Subscription, error) {
	subscriptions, err := s.store.LoadSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("load subscriptions: %w", err)
	}

	updated := make([]domain.Subscription, 0, len(subscriptions))
	for _, sub := range subscriptions {
		refreshed, err := s.RefreshSubscription(ctx, sub.ID)
		if err != nil {
			return updated, err
		}
		updated = append(updated, refreshed)
	}

	return updated, nil
}

// ConnectManual pins a subscription and node and applies the backend config.
func (s *Service) ConnectManual(ctx context.Context, subscriptionID, nodeID string) error {
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
	if s.backend != nil {
		if err := s.backend.Stop(ctx); err != nil {
			return fmt.Errorf("stop backend: %w", err)
		}
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
				if _, err := s.ConnectAuto(context.Background(), state.ActiveSubscriptionID); err != nil {
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
				if err := s.ConnectManual(context.Background(), state.ActiveSubscriptionID, state.ActiveNodeID); err != nil {
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

func (s *Service) resolveSubscriptionSource(ctx context.Context, req AddSubscriptionRequest) (string, domain.SourceType, error) {
	switch {
	case strings.TrimSpace(req.URL) != "":
		content, err := s.fetchSubscription(ctx, req.URL)
		if err != nil {
			return "", "", fmt.Errorf("fetch subscription: %w", err)
		}
		return content, domain.SourceTypeURL, nil
	case strings.TrimSpace(req.Raw) != "":
		return req.Raw, domain.SourceTypeRaw, nil
	default:
		return "", "", fmt.Errorf("either url or raw payload is required")
	}
}

func (s *Service) fetchSubscription(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: unexpected status %s", rawURL, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	return string(body), nil
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

func deriveProviderName(sourceType domain.SourceType, rawURL string) string {
	if sourceType == domain.SourceTypeURL {
		if parsed, err := url.Parse(rawURL); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}

	return "Imported Subscription"
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

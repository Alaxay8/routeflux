package app

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/parser"
	"github.com/Alaxay8/routeflux/internal/probe"
	"github.com/Alaxay8/routeflux/internal/speedtest"
)

const inspectSpeedTimeout = 75 * time.Second

// Store defines the persisted state contract required by the service layer.
type Store interface {
	LoadSubscriptions() ([]domain.Subscription, error)
	SaveSubscriptions([]domain.Subscription) error
	LoadSettings() (domain.Settings, error)
	SaveSettings(domain.Settings) error
	LoadState() (domain.RuntimeState, error)
	SaveState(domain.RuntimeState) error
}

// Firewaller applies OpenWrt transparent proxy rules.
type Firewaller interface {
	Validate(ctx context.Context, settings domain.FirewallSettings) error
	Apply(ctx context.Context, settings domain.FirewallSettings) error
	Disable(ctx context.Context) error
}

// HostResolver resolves node hostnames before backend apply.
type HostResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
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
	store                   Store
	backend                 backend.Backend
	firewall                Firewaller
	httpClient              *http.Client
	subscriptionTLS12Client *http.Client
	checker                 probe.Checker
	speedTester             speedtest.Tester
	logger                  *slog.Logger
	resolver                HostResolver
	backendReadyChecks      int
	backendReadyDelay       time.Duration
	now                     func() time.Time
	autoHealthStateMu       sync.Mutex
	autoHealthState         *autoHealthStateCache
}

// Dependencies groups the service construction inputs.
type Dependencies struct {
	Store       Store
	Backend     backend.Backend
	Firewaller  Firewaller
	HTTPClient  *http.Client
	Checker     probe.Checker
	SpeedTester speedtest.Tester
	Logger      *slog.Logger
	Resolver    HostResolver
}

type subscriptionFetchMetadata struct {
	ProviderName string
	ExpiresAt    *time.Time
	Traffic      *domain.SubscriptionTraffic
}

type subscriptionFetchResult struct {
	Content  string
	Metadata subscriptionFetchMetadata
}

const (
	subscriptionFetchMaxAttempts          = 3
	subscriptionFetchBaseBackoff          = 250 * time.Millisecond
	subscriptionFetchUserAgent            = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	subscriptionMetadataFallbackUserAgent = "curl/8.7.1"
	subscriptionProfileTitleKey           = "Profile-Title"
	subscriptionUserInfoKey               = "Subscription-Userinfo"
	subscriptionAltUserInfoKey            = "X-Subscription-Userinfo"
	backendReadyMaxChecks                 = 8
	backendReadyCheckDelay                = 250 * time.Millisecond
)

var subscriptionShareLinkPattern = regexp.MustCompile(`(?i)(vless|vmess|trojan|ss)://[^"'<>]+`)
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

	resolver := deps.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	return &Service{
		store:                   deps.Store,
		backend:                 deps.Backend,
		firewall:                deps.Firewaller,
		httpClient:              ensureSubscriptionHTTPClient(deps.HTTPClient),
		subscriptionTLS12Client: ensureSubscriptionTLS12HTTPClient(deps.HTTPClient),
		checker:                 checker,
		speedTester:             deps.SpeedTester,
		logger:                  logger,
		resolver:                resolver,
		backendReadyChecks:      backendReadyMaxChecks,
		backendReadyDelay:       backendReadyCheckDelay,
		now:                     time.Now,
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
		SourceType:         sourceType,
		Source:             storedSource,
		ProviderName:       providerName,
		ProviderNameSource: providerNameSource,
		DisplayName:        providerName,
		LastUpdatedAt:      now,
		ExpiresAt:          metadata.ExpiresAt,
		Traffic:            metadata.Traffic,
		RefreshInterval:    settings.RefreshInterval,
		ParserStatus:       "ok",
		Nodes:              nodes,
	}

	sub.ID = resolveAddSubscriptionID(subscriptions, sub)
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
		_ = s.saveState(state)
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
	if err := s.saveState(state); err != nil {
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

// InspectXrayConfig renders the Xray config RouteFlux would generate for a node.
func (s *Service) InspectXrayConfig(subscriptionID, nodeID string) (json.RawMessage, error) {
	if s.backend == nil {
		return nil, fmt.Errorf("backend is not configured")
	}

	settings, err := s.store.LoadSettings()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	sub, node, err := s.subscriptionNode(subscriptionID, nodeID)
	if err != nil {
		return nil, err
	}

	rendered, err := s.backend.GenerateConfig(s.backendConfigRequest(settings, node, domain.SelectionModeManual, 10808, 10809, firewallEnabled(settings.Firewall)))
	if err != nil {
		return nil, fmt.Errorf("generate xray config for %s/%s: %w", sub.ID, node.ID, err)
	}

	return json.RawMessage(rendered), nil
}

// InspectSpeed runs an isolated router-side speed test for a node.
func (s *Service) InspectSpeed(ctx context.Context, subscriptionID, nodeID string) (speedtest.Result, error) {
	if s.backend == nil {
		return speedtest.Result{}, fmt.Errorf("backend is not configured")
	}
	if s.speedTester == nil {
		return speedtest.Result{}, fmt.Errorf("speed tester is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, inspectSpeedTimeout)
	defer cancel()

	settings, err := s.store.LoadSettings()
	if err != nil {
		return speedtest.Result{}, fmt.Errorf("load settings: %w", err)
	}

	_, node, err := s.subscriptionNode(subscriptionID, nodeID)
	if err != nil {
		return speedtest.Result{}, err
	}

	socksPort, err := pickFreeTCPPort()
	if err != nil {
		return speedtest.Result{}, fmt.Errorf("allocate speed test SOCKS port: %w", err)
	}
	httpPort, err := pickFreeTCPPort()
	if err != nil {
		return speedtest.Result{}, fmt.Errorf("allocate speed test HTTP port: %w", err)
	}

	rendered, err := s.backend.GenerateConfig(s.backendConfigRequest(settings, node, domain.SelectionModeManual, socksPort, httpPort, false))
	if err != nil {
		return speedtest.Result{}, fmt.Errorf("generate speed test config: %w", err)
	}

	return s.speedTester.Test(ctx, speedtest.Request{
		SubscriptionID: subscriptionID,
		NodeID:         nodeID,
		NodeName:       node.DisplayName(),
		Config:         rendered,
		HTTPProxyPort:  httpPort,
	})
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
	if metadata.ExpiresAt != nil {
		sub.ExpiresAt = metadata.ExpiresAt
	}
	if metadata.Traffic != nil {
		sub.Traffic = metadata.Traffic
	}
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
		_ = s.saveState(state)
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

	s.logInfo("manual connect requested", "subscription", sub.ID, "node", node.ID)
	if err := s.applyNodeSelection(ctx, sub, node, domain.SelectionModeManual); err != nil {
		return err
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = false
		settings.Mode = domain.SelectionModeManual
		_ = s.store.SaveSettings(settings)
	}

	s.logInfo("manual connect succeeded", "subscription", sub.ID, "node", node.ID)
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
	s.logInfo("auto selection requested", "subscription", sub.ID)

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.Node{}, fmt.Errorf("load settings: %w", err)
	}

	state, err := s.loadStateWithAutoHealthCache()
	if err != nil {
		return domain.Node{}, fmt.Errorf("load state: %w", err)
	}

	decision, err := s.evaluateAutoSelection(ctx, sub, settings, state)
	if err != nil {
		return domain.Node{}, err
	}

	s.logAutoDecision("auto selection decision", sub, decision)

	if !decision.HasHealthyCandidate {
		state.Health = decision.Health
		if err := s.saveState(state); err != nil {
			return domain.Node{}, fmt.Errorf("save state: %w", err)
		}
		return domain.Node{}, errors.New(decision.Reason)
	}

	selectedNode, err := s.commitAutoSelection(ctx, sub, decision)
	if err != nil {
		return domain.Node{}, err
	}

	if err == nil {
		settings.AutoMode = true
		settings.Mode = domain.SelectionModeAuto
		_ = s.store.SaveSettings(settings)
	}

	s.logInfo("auto connect succeeded", "subscription", sub.ID, "node", selectedNode.ID)
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
	if err := s.saveState(state); err != nil {
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

// RestoreRuntime reapplies a persisted active connection during daemon startup.
func (s *Service) RestoreRuntime(ctx context.Context) error {
	return runStoreWriteLocked(s, func() error {
		return s.restoreRuntime(ctx)
	})
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

func (s *Service) restoreRuntime(ctx context.Context) error {
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

	s.logInfo("restore runtime start", "subscription", state.ActiveSubscriptionID, "node", state.ActiveNodeID, "mode", state.Mode)
	if err := s.reapplyCurrentConnection(ctx); err != nil {
		reason := fmt.Sprintf("restore runtime: %v", err)
		s.logWarn("restore runtime failed", "subscription", state.ActiveSubscriptionID, "node", state.ActiveNodeID, "error", err.Error())
		if persistErr := s.persistRestoreFailure(ctx, reason); persistErr != nil {
			return fmt.Errorf("%s: %v", reason, persistErr)
		}
		return errors.New(reason)
	}

	settings, err := s.store.LoadSettings()
	if err == nil && syncSettingsToRuntime(&settings, state) {
		_ = s.store.SaveSettings(settings)
	}

	s.logInfo("restore runtime succeeded", "subscription", state.ActiveSubscriptionID, "node", state.ActiveNodeID, "mode", state.Mode)
	return nil
}

// GetFirewallSettings returns the transparent proxy routing settings.
func (s *Service) GetFirewallSettings() (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	return domain.CanonicalFirewallSettings(settings.Firewall), nil
}

// UpdateFirewallModeDraft stores selectors for one LuCI firewall mode without applying them.
func (s *Service) UpdateFirewallModeDraft(ctx context.Context, mode string, selectors []string) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.updateFirewallModeDraft(ctx, mode, selectors)
	})
}

func (s *Service) updateFirewallModeDraft(ctx context.Context, mode string, selectors []string) (domain.FirewallSettings, error) {
	_ = ctx

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "hosts":
		sources, err := domain.ParseFirewallSources(selectors)
		if err != nil {
			return domain.FirewallSettings{}, err
		}
		settings.Firewall.ModeDrafts.Hosts = domain.FirewallModeDraft{
			SourceCIDRs: sources,
		}
	case "targets":
		parsed, err := domain.ParseFirewallTargets(selectors, settings.Firewall.TargetServiceCatalog)
		if err != nil {
			return domain.FirewallSettings{}, err
		}
		settings.Firewall.ModeDrafts.Targets = domain.FirewallModeDraft{
			TargetServices: slices.Clone(parsed.Services),
			TargetCIDRs:    slices.Clone(parsed.CIDRs),
			TargetDomains:  slices.Clone(parsed.Domains),
		}
	default:
		return domain.FirewallSettings{}, fmt.Errorf("unsupported firewall draft mode %q", mode)
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	return settings.Firewall, nil
}

// UpdateFirewallSplitDraft stores split selectors without applying them.
func (s *Service) UpdateFirewallSplitDraft(ctx context.Context, proxySelectors, bypassSelectors, excludedSources []string) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.updateFirewallSplitDraft(ctx, proxySelectors, bypassSelectors, excludedSources)
	})
}

func (s *Service) updateFirewallSplitDraft(ctx context.Context, proxySelectors, bypassSelectors, excludedSources []string) (domain.FirewallSettings, error) {
	_ = ctx

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	proxyTargets, err := domain.ParseFirewallTargets(proxySelectors, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallSettings{}, err
	}
	bypassTargets, err := domain.ParseFirewallTargets(bypassSelectors, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallSettings{}, err
	}
	sources, err := domain.ParseFirewallSources(excludedSources)
	if err != nil {
		return domain.FirewallSettings{}, err
	}

	settings.Firewall.ModeDrafts.Split = domain.FirewallSplitDraft{
		Proxy:           domain.FirewallSelectorSetFromTargets(proxyTargets),
		Bypass:          domain.FirewallSelectorSetFromTargets(bypassTargets),
		ExcludedSources: sources,
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	return settings.Firewall, nil
}

// ClearFirewallModeDraft removes saved selectors for one LuCI firewall mode.
func (s *Service) ClearFirewallModeDraft(ctx context.Context, mode string) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.clearFirewallModeDraft(ctx, mode)
	})
}

func (s *Service) clearFirewallModeDraft(ctx context.Context, mode string) (domain.FirewallSettings, error) {
	_ = ctx

	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "hosts":
		settings.Firewall.ModeDrafts.Hosts = domain.FirewallModeDraft{}
	case "targets":
		settings.Firewall.ModeDrafts.Targets = domain.FirewallModeDraft{}
	case "split":
		settings.Firewall.ModeDrafts.Split = domain.FirewallSplitDraft{}
	default:
		return domain.FirewallSettings{}, fmt.Errorf("unsupported firewall draft mode %q", mode)
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	return settings.Firewall, nil
}

// ListFirewallTargetServices returns built-in and custom target services.
func (s *Service) ListFirewallTargetServices() ([]domain.FirewallTargetService, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	return domain.FirewallTargetServices(settings.Firewall.TargetServiceCatalog), nil
}

// GetFirewallTargetService returns one target service by name.
func (s *Service) GetFirewallTargetService(name string) (domain.FirewallTargetService, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallTargetService{}, fmt.Errorf("load settings: %w", err)
	}

	entry, ok := domain.LookupFirewallTargetService(settings.Firewall.TargetServiceCatalog, name)
	if !ok {
		return domain.FirewallTargetService{}, fmt.Errorf("target service %q not found", name)
	}

	return entry, nil
}

// SetFirewallTargetService creates or updates a custom target service.
func (s *Service) SetFirewallTargetService(ctx context.Context, name string, selectors []string) (domain.FirewallTargetService, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallTargetService, error) {
		return s.setFirewallTargetService(ctx, name, selectors)
	})
}

func (s *Service) setFirewallTargetService(ctx context.Context, name string, selectors []string) (domain.FirewallTargetService, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallTargetService{}, fmt.Errorf("load settings: %w", err)
	}

	normalizedName, definition, err := domain.ParseFirewallTargetDefinition(name, selectors, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallTargetService{}, err
	}

	if settings.Firewall.TargetServiceCatalog == nil {
		settings.Firewall.TargetServiceCatalog = make(map[string]domain.FirewallTargetDefinition)
	}
	settings.Firewall.TargetServiceCatalog[normalizedName] = definition

	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallTargetService{}, err
	}
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallTargetService{}, fmt.Errorf("save settings: %w", err)
	}

	if settings.Firewall.Enabled {
		if err := s.reapplyCurrentConnection(ctx); err != nil {
			return domain.FirewallTargetService{}, err
		}
	}

	entry, ok := domain.LookupFirewallTargetService(settings.Firewall.TargetServiceCatalog, normalizedName)
	if !ok {
		return domain.FirewallTargetService{}, fmt.Errorf("target service %q not found after save", normalizedName)
	}
	return entry, nil
}

// DeleteFirewallTargetService removes a custom target service.
func (s *Service) DeleteFirewallTargetService(ctx context.Context, name string) error {
	return runStoreWriteLocked(s, func() error {
		return s.deleteFirewallTargetService(ctx, name)
	})
}

func (s *Service) deleteFirewallTargetService(ctx context.Context, name string) error {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	entry, ok := domain.LookupFirewallTargetService(settings.Firewall.TargetServiceCatalog, name)
	if !ok {
		return fmt.Errorf("target service %q not found", name)
	}
	if entry.ReadOnly {
		return fmt.Errorf("target service %q is readonly and cannot be deleted", entry.Name)
	}
	if firewallSelectorSetReferencesService(settings.Firewall.Targets, settings.Firewall.TargetServiceCatalog, entry.Name) {
		return fmt.Errorf("target service %q is still used by firewall targets; remove it from firewall targets first", entry.Name)
	}
	if firewallSelectorSetReferencesService(settings.Firewall.Split.Proxy, settings.Firewall.TargetServiceCatalog, entry.Name) ||
		firewallSelectorSetReferencesService(settings.Firewall.Split.Bypass, settings.Firewall.TargetServiceCatalog, entry.Name) {
		return fmt.Errorf("target service %q is still used by split tunnelling; remove it from split tunnelling first", entry.Name)
	}
	if firewallTargetsReferenceService(settings.Firewall.ModeDrafts.Targets.TargetServices, settings.Firewall.TargetServiceCatalog, entry.Name) {
		return fmt.Errorf("target service %q is still referenced by the saved targets draft", entry.Name)
	}
	if firewallSelectorSetReferencesService(settings.Firewall.ModeDrafts.Split.Proxy, settings.Firewall.TargetServiceCatalog, entry.Name) ||
		firewallSelectorSetReferencesService(settings.Firewall.ModeDrafts.Split.Bypass, settings.Firewall.TargetServiceCatalog, entry.Name) {
		return fmt.Errorf("target service %q is still referenced by the saved split draft", entry.Name)
	}
	if referrer, ok := findReferencingTargetService(settings.Firewall.TargetServiceCatalog, entry.Name); ok {
		return fmt.Errorf("target service %q is still referenced by target service %q", entry.Name, referrer)
	}

	delete(settings.Firewall.TargetServiceCatalog, entry.Name)
	if len(settings.Firewall.TargetServiceCatalog) == 0 {
		settings.Firewall.TargetServiceCatalog = nil
	}

	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return err
	}
	if err := s.store.SaveSettings(settings); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}

	if settings.Firewall.Enabled {
		if err := s.reapplyCurrentConnection(ctx); err != nil {
			return err
		}
	}

	return nil
}

func firewallTargetsReferenceService(roots []string, catalog map[string]domain.FirewallTargetDefinition, target string) bool {
	for _, root := range roots {
		if targetServiceDependencyMatches(strings.TrimSpace(strings.ToLower(root)), target, catalog, make(map[string]struct{})) {
			return true
		}
	}
	return false
}

func firewallSelectorSetReferencesService(selectors domain.FirewallSelectorSet, catalog map[string]domain.FirewallTargetDefinition, target string) bool {
	return firewallTargetsReferenceService(selectors.Services, catalog, target)
}

func findReferencingTargetService(catalog map[string]domain.FirewallTargetDefinition, target string) (string, bool) {
	target = strings.TrimSpace(strings.ToLower(target))
	for name := range catalog {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == target {
			continue
		}
		if targetServiceDependencyMatches(name, target, catalog, make(map[string]struct{})) {
			return name, true
		}
	}
	return "", false
}

func targetServiceDependencyMatches(root, target string, catalog map[string]domain.FirewallTargetDefinition, visiting map[string]struct{}) bool {
	if root == target {
		return true
	}
	if _, ok := visiting[root]; ok {
		return false
	}
	definition, ok := catalog[root]
	if !ok {
		return false
	}

	visiting[root] = struct{}{}
	defer delete(visiting, root)

	for _, dependency := range definition.Services {
		dependency = strings.TrimSpace(strings.ToLower(dependency))
		if dependency == target {
			return true
		}
		if targetServiceDependencyMatches(dependency, target, catalog, visiting) {
			return true
		}
	}

	return false
}

// ConfigureFirewall updates firewall targets and enabled state.
func (s *Service) ConfigureFirewall(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.configureFirewallTargets(ctx, targets, enabled, port)
	})
}

func (s *Service) configureFirewall(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return s.configureFirewallTargets(ctx, targets, enabled, port)
}

// ConfigureFirewallAntiTargets routes all other LAN traffic through the transparent proxy
// while keeping selected targets direct.
func (s *Service) ConfigureFirewallAntiTargets(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.configureFirewallSplit(ctx, nil, targets, nil, enabled, port, domain.FirewallDefaultActionProxy)
	})
}

func (s *Service) configureFirewallTargets(ctx context.Context, targets []string, enabled bool, port int) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	parsedTargets, err := domain.ParseFirewallTargets(targets, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallSettings{}, err
	}

	settings.Firewall.Enabled = enabled
	settings.Firewall.Mode = domain.FirewallModeTargets
	settings.Firewall.Hosts = nil
	settings.Firewall.Targets = domain.FirewallSelectorSetFromTargets(parsedTargets)
	settings.Firewall.Split = domain.DefaultFirewallSplitSettings()
	settings.Firewall.ModeDrafts.Targets = domain.FirewallModeDraft{
		TargetServices: slices.Clone(parsedTargets.Services),
		TargetCIDRs:    slices.Clone(parsedTargets.CIDRs),
		TargetDomains:  slices.Clone(parsedTargets.Domains),
	}
	if port > 0 {
		settings.Firewall.TransparentPort = port
	}

	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallSettings{}, err
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// ConfigureFirewallSplit applies an explicit split tunnelling policy.
func (s *Service) ConfigureFirewallSplit(ctx context.Context, proxyTargets, bypassTargets, excludedSources []string, enabled bool, port int) (domain.FirewallSettings, error) {
	return runStoreWriteLockedResult(s, func() (domain.FirewallSettings, error) {
		return s.configureFirewallSplit(ctx, proxyTargets, bypassTargets, excludedSources, enabled, port, domain.FirewallDefaultActionDirect)
	})
}

func (s *Service) configureFirewallSplit(ctx context.Context, proxyTargets, bypassTargets, excludedSources []string, enabled bool, port int, defaultAction domain.FirewallDefaultAction) (domain.FirewallSettings, error) {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("load settings: %w", err)
	}

	proxyParsed, err := domain.ParseFirewallTargets(proxyTargets, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallSettings{}, err
	}
	bypassParsed, err := domain.ParseFirewallTargets(bypassTargets, settings.Firewall.TargetServiceCatalog)
	if err != nil {
		return domain.FirewallSettings{}, err
	}
	sources, err := domain.ParseFirewallSources(excludedSources)
	if err != nil {
		return domain.FirewallSettings{}, err
	}

	settings.Firewall.Enabled = enabled
	settings.Firewall.Mode = domain.FirewallModeSplit
	settings.Firewall.Hosts = nil
	settings.Firewall.Targets = domain.FirewallSelectorSet{}
	settings.Firewall.Split = domain.FirewallSplitSettings{
		Proxy:           domain.FirewallSelectorSetFromTargets(proxyParsed),
		Bypass:          domain.FirewallSelectorSetFromTargets(bypassParsed),
		ExcludedSources: sources,
		DefaultAction:   domain.NormalizeFirewallDefaultAction(defaultAction),
	}
	settings.Firewall.ModeDrafts.Split = domain.FirewallSplitDraft{
		Proxy:           domain.CloneFirewallSelectorSet(settings.Firewall.Split.Proxy),
		Bypass:          domain.CloneFirewallSelectorSet(settings.Firewall.Split.Bypass),
		ExcludedSources: slices.Clone(settings.Firewall.Split.ExcludedSources),
	}
	if port > 0 {
		settings.Firewall.TransparentPort = port
	}

	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallSettings{}, err
	}

	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

// ConfigureFirewallHosts routes all traffic from selected client IPs through the transparent proxy.
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

	parsedSources, err := domain.ParseFirewallSources(sources)
	if err != nil {
		return domain.FirewallSettings{}, err
	}

	settings.Firewall.Enabled = enabled
	settings.Firewall.Mode = domain.FirewallModeHosts
	settings.Firewall.Hosts = parsedSources
	settings.Firewall.Targets = domain.FirewallSelectorSet{}
	settings.Firewall.Split = domain.DefaultFirewallSplitSettings()
	settings.Firewall.ModeDrafts.Hosts = domain.FirewallModeDraft{
		SourceCIDRs: slices.Clone(settings.Firewall.Hosts),
	}
	if port > 0 {
		settings.Firewall.TransparentPort = port
	}

	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallSettings{}, err
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

	settings.Firewall = domain.CanonicalFirewallSettings(settings.Firewall)
	settings.Firewall.TransparentPort = port
	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallSettings{}, err
	}
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

	settings.Firewall = domain.CanonicalFirewallSettings(settings.Firewall)
	settings.Firewall.BlockQUIC = enabled
	if err := s.validateFirewall(ctx, settings.Firewall); err != nil {
		return domain.FirewallSettings{}, err
	}
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
	settings.Firewall.Mode = domain.FirewallModeDisabled
	settings.Firewall.Hosts = nil
	settings.Firewall.Targets = domain.FirewallSelectorSet{}
	settings.Firewall.Split = domain.DefaultFirewallSplitSettings()
	if err := s.store.SaveSettings(settings); err != nil {
		return domain.FirewallSettings{}, fmt.Errorf("save settings: %w", err)
	}

	if err := s.reapplyCurrentConnection(ctx); err != nil {
		return domain.FirewallSettings{}, err
	}

	return settings.Firewall, nil
}

func (s *Service) validateFirewall(ctx context.Context, settings domain.FirewallSettings) error {
	if s.firewall == nil {
		return nil
	}
	return s.firewall.Validate(ctx, domain.CanonicalFirewallSettings(settings))
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
			result.Metadata = s.enrichSubscriptionMetadata(ctx, rawURL, result.Metadata)
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

func (s *Service) enrichSubscriptionMetadata(ctx context.Context, rawURL string, metadata subscriptionFetchMetadata) subscriptionFetchMetadata {
	if !needsSubscriptionMetadataFallback(metadata) {
		return metadata
	}

	fallback, err := s.fetchSubscriptionMetadata(ctx, rawURL, subscriptionMetadataFallbackUserAgent)
	if err != nil {
		return metadata
	}

	return mergeSubscriptionMetadata(metadata, fallback)
}

func (s *Service) fetchSubscriptionOnce(ctx context.Context, rawURL string) (subscriptionFetchResult, bool, error) {
	result, retry, err := s.fetchSubscriptionOnceWithClient(ctx, rawURL, s.httpClient)
	if err == nil || !shouldRetrySubscriptionTLS12(rawURL, err) || s.subscriptionTLS12Client == nil {
		return result, retry, err
	}

	return s.fetchSubscriptionOnceWithClient(ctx, rawURL, s.subscriptionTLS12Client)
}

func (s *Service) fetchSubscriptionOnceWithClient(ctx context.Context, rawURL string, client *http.Client) (subscriptionFetchResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return subscriptionFetchResult{}, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/plain, application/json;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", subscriptionFetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return subscriptionFetchResult{}, false, fmt.Errorf("fetch %s: %w", rawURL, ctx.Err())
		}

		return subscriptionFetchResult{}, true, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		message := summarizeSubscriptionFailure(resp, body)
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
			ExpiresAt:    parseSubscriptionExpiry(resp.Header),
			Traffic:      parseSubscriptionTraffic(resp.Header),
		},
	}, false, nil
}

func (s *Service) fetchSubscriptionMetadata(ctx context.Context, rawURL, userAgent string) (subscriptionFetchMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return subscriptionFetchMetadata{}, fmt.Errorf("build metadata request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return subscriptionFetchMetadata{}, fmt.Errorf("fetch metadata %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return subscriptionFetchMetadata{}, fmt.Errorf("fetch metadata %s: unexpected status %s", rawURL, resp.Status)
	}

	return subscriptionFetchMetadata{
		ProviderName: decodeProfileTitle(resp.Header.Get(subscriptionProfileTitleKey)),
		ExpiresAt:    parseSubscriptionExpiry(resp.Header),
		Traffic:      parseSubscriptionTraffic(resp.Header),
	}, nil
}

func needsSubscriptionMetadataFallback(metadata subscriptionFetchMetadata) bool {
	return metadata.ExpiresAt == nil || metadata.Traffic == nil
}

func mergeSubscriptionMetadata(primary, fallback subscriptionFetchMetadata) subscriptionFetchMetadata {
	if strings.TrimSpace(primary.ProviderName) == "" {
		primary.ProviderName = fallback.ProviderName
	}
	if primary.ExpiresAt == nil {
		primary.ExpiresAt = fallback.ExpiresAt
	}
	if primary.Traffic == nil {
		primary.Traffic = fallback.Traffic
	}
	return primary
}

func parseSubscriptionExpiry(headers http.Header) *time.Time {
	if parsed := parseSubscriptionUserinfo(headers); parsed.ExpiresAt != nil {
		return parsed.ExpiresAt
	}

	return parseDDoSGuardCookieExpiry(headers)
}

func parseSubscriptionTraffic(headers http.Header) *domain.SubscriptionTraffic {
	return parseSubscriptionUserinfo(headers).Traffic
}

type parsedSubscriptionUserinfo struct {
	ExpiresAt *time.Time
	Traffic   *domain.SubscriptionTraffic
}

func parseSubscriptionUserinfo(headers http.Header) parsedSubscriptionUserinfo {
	var result parsedSubscriptionUserinfo

	for _, value := range []string{
		headers.Get(subscriptionUserInfoKey),
		headers.Get(subscriptionAltUserInfoKey),
	} {
		parsed := parseSubscriptionUserinfoValue(value)
		if result.ExpiresAt == nil {
			result.ExpiresAt = parsed.ExpiresAt
		}
		if result.Traffic == nil {
			result.Traffic = parsed.Traffic
		}
	}

	return result
}

func parseSubscriptionUserinfoValue(raw string) parsedSubscriptionUserinfo {
	var result parsedSubscriptionUserinfo
	var traffic domain.SubscriptionTraffic
	hasTraffic := false

	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}

		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || number < 0 {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(key)) {
		case "expire":
			if number > 0 {
				expiresAt := time.Unix(number, 0).UTC()
				result.ExpiresAt = &expiresAt
			}
		case "upload":
			traffic.UploadBytes = number
			hasTraffic = true
		case "download":
			traffic.DownloadBytes = number
			hasTraffic = true
		case "total":
			traffic.TotalBytes = number
			hasTraffic = true
		}
	}

	if hasTraffic {
		result.Traffic = &traffic
	}

	return result
}

func parseDDoSGuardCookieExpiry(headers http.Header) *time.Time {
	if !strings.Contains(strings.ToLower(headers.Get("Server")), "ddos-guard") {
		return nil
	}

	var latest time.Time
	found := false
	for _, value := range headers.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(value)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(cookie.Name)), "__ddg") {
			continue
		}
		if cookie.Expires.IsZero() {
			continue
		}

		expiresAt := cookie.Expires.UTC()
		if !found || expiresAt.After(latest) {
			latest = expiresAt
			found = true
		}
	}

	if !found {
		return nil
	}

	return &latest
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

func ensureSubscriptionTLS12HTTPClient(base *http.Client) *http.Client {
	client := ensureSubscriptionHTTPClient(base)
	transport, ok := cloneSubscriptionTransportWithTLSMaxVersion(client.Transport, tls.VersionTLS12)
	if !ok {
		return nil
	}

	copy := *client
	copy.Transport = transport
	return &copy
}

func cloneSubscriptionTransportWithTLSMaxVersion(base http.RoundTripper, maxVersion uint16) (*http.Transport, bool) {
	if base == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = cloneTLSConfigWithMaxVersion(transport.TLSClientConfig, maxVersion)
		return transport, true
	}

	transport, ok := base.(*http.Transport)
	if !ok {
		return nil, false
	}

	clone := transport.Clone()
	clone.TLSClientConfig = cloneTLSConfigWithMaxVersion(clone.TLSClientConfig, maxVersion)
	return clone, true
}

func cloneTLSConfigWithMaxVersion(base *tls.Config, maxVersion uint16) *tls.Config {
	if base == nil {
		return &tls.Config{MaxVersion: maxVersion}
	}

	clone := base.Clone()
	clone.MaxVersion = maxVersion
	return clone
}

func shouldRetrySubscriptionTLS12(rawURL string, err error) bool {
	if err == nil {
		return false
	}

	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "tls handshake timeout")
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

func summarizeSubscriptionFailure(resp *http.Response, body []byte) string {
	if message := summarizeDDoSGuardResponse(resp, body); message != "" {
		return message
	}

	return summarizeSubscriptionResponseBody(body)
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

func summarizeDDoSGuardResponse(resp *http.Response, body []byte) string {
	if !isDDoSGuardResponse(resp, body) {
		return ""
	}

	switch resp.StatusCode {
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "subscription endpoint is protected by DDoS-Guard and rejected the automated request"
	default:
		return "subscription endpoint is protected by DDoS-Guard"
	}
}

func isDDoSGuardResponse(resp *http.Response, body []byte) bool {
	if resp == nil {
		return false
	}

	if strings.Contains(strings.ToLower(resp.Header.Get("Server")), "ddos-guard") {
		return true
	}

	return strings.Contains(strings.ToLower(string(body)), "ddos-guard")
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

func (s *Service) subscriptionNode(subscriptionID, nodeID string) (domain.Subscription, domain.Node, error) {
	sub, err := s.subscriptionByID(subscriptionID)
	if err != nil {
		return domain.Subscription{}, domain.Node{}, err
	}

	node, ok := sub.NodeByID(nodeID)
	if !ok {
		return domain.Subscription{}, domain.Node{}, fmt.Errorf("node %q not found in subscription %q", nodeID, subscriptionID)
	}

	return sub, node, nil
}

func (s *Service) backendConfigRequest(settings domain.Settings, node domain.Node, mode domain.SelectionMode, socksPort, httpPort int, transparent bool) backend.ConfigRequest {
	req := backend.ConfigRequest{
		Mode:                        mode,
		Nodes:                       []domain.Node{node},
		SelectedNodeID:              node.ID,
		LogLevel:                    settings.LogLevel,
		DNS:                         settings.DNS,
		SOCKSPort:                   socksPort,
		HTTPPort:                    httpPort,
		TransparentProxy:            transparent,
		TransparentSelectiveCapture: transparentSelectiveCapture(settings.Firewall),
		TransparentBlockQUIC:        domain.EffectiveTransparentBlockQUIC(settings.Firewall, &node),
		TransparentPort:             settings.Firewall.TransparentPort,
	}

	switch domain.NormalizeFirewallMode(settings.Firewall.Mode) {
	case domain.FirewallModeTargets:
		req.TransparentDefaultAction = domain.FirewallDefaultActionDirect
		req.TransparentProxyDomains = domain.ExpandFirewallSelectorSetDomains(settings.Firewall.TargetServiceCatalog, settings.Firewall.Targets)
		req.TransparentProxyCIDRs = domain.ExpandFirewallSelectorSetCIDRs(settings.Firewall.TargetServiceCatalog, settings.Firewall.Targets)
	case domain.FirewallModeSplit:
		req.TransparentDefaultAction = domain.NormalizeFirewallDefaultAction(settings.Firewall.Split.DefaultAction)
		req.TransparentProxyDomains = domain.ExpandFirewallSelectorSetDomains(settings.Firewall.TargetServiceCatalog, settings.Firewall.Split.Proxy)
		req.TransparentProxyCIDRs = domain.ExpandFirewallSelectorSetCIDRs(settings.Firewall.TargetServiceCatalog, settings.Firewall.Split.Proxy)
		req.TransparentBypassDomains = domain.ExpandFirewallSelectorSetDomains(settings.Firewall.TargetServiceCatalog, settings.Firewall.Split.Bypass)
		req.TransparentBypassCIDRs = domain.ExpandFirewallSelectorSetCIDRs(settings.Firewall.TargetServiceCatalog, settings.Firewall.Split.Bypass)
	default:
		req.TransparentDefaultAction = domain.FirewallDefaultActionProxy
	}

	return req
}

func transparentSelectiveCapture(settings domain.FirewallSettings) bool {
	switch domain.CanonicalFirewallMode(settings) {
	case domain.FirewallModeTargets:
		return true
	case domain.FirewallModeSplit:
		return domain.NormalizeFirewallDefaultAction(settings.Split.DefaultAction) == domain.FirewallDefaultActionDirect
	default:
		return false
	}
}

func firewallEnabled(settings domain.FirewallSettings) bool {
	return domain.FirewallRoutingEnabled(settings)
}

func pickFreeTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	return addr.Port, nil
}

func (s *Service) applyNodeSelection(ctx context.Context, sub domain.Subscription, node domain.Node, mode domain.SelectionMode) error {
	settings, err := s.store.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	resolvedNode, err := s.resolveNodeAddress(ctx, node)
	if err != nil {
		return fmt.Errorf("resolve node address: %w", err)
	}

	if s.backend != nil {
		s.logInfo("apply backend config", "subscription", sub.ID, "node", node.ID, "mode", mode, "resolved_address", resolvedNode.Address)
		if err := s.backend.ApplyConfig(ctx, s.backendConfigRequest(settings, resolvedNode, mode, 10808, 10809, firewallEnabled(settings.Firewall))); err != nil {
			s.logWarn("apply backend config failed", "subscription", sub.ID, "node", node.ID, "mode", mode, "error", err.Error())
			_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("apply backend config: %v", err))
			return fmt.Errorf("apply backend config: %w", err)
		}
		if err := s.ensureBackendRunning(ctx, sub.ID, node.ID, mode); err != nil {
			return err
		}
	}

	if s.firewall != nil {
		if domain.FirewallRoutingEnabled(settings.Firewall) {
			s.logInfo(
				"apply firewall rules",
				"subscription", sub.ID,
				"node", node.ID,
				"firewall_mode", settings.Firewall.Mode,
				"targets", settings.Firewall.Targets,
				"split_proxy", settings.Firewall.Split.Proxy,
				"split_bypass", settings.Firewall.Split.Bypass,
				"split_excluded_sources", settings.Firewall.Split.ExcludedSources,
				"hosts", settings.Firewall.Hosts,
			)
			if err := s.firewall.Apply(ctx, settings.Firewall); err != nil {
				s.logWarn("apply firewall failed", "subscription", sub.ID, "node", node.ID, "error", err.Error())
				_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("apply firewall: %v", err))
				return fmt.Errorf("apply firewall: %w", err)
			}
			s.logInfo("firewall rules applied", "subscription", sub.ID, "node", node.ID)
		} else if err := s.firewall.Disable(ctx); err != nil {
			s.logWarn("disable firewall failed", "subscription", sub.ID, "node", node.ID, "error", err.Error())
			_ = s.markConnectionFailed(ctx, sub.ID, node.ID, mode, fmt.Sprintf("disable firewall: %v", err))
			return fmt.Errorf("disable firewall: %w", err)
		} else {
			s.logInfo("firewall rules disabled", "subscription", sub.ID, "node", node.ID)
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

	if err := s.saveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

func (s *Service) resolveNodeAddress(ctx context.Context, node domain.Node) (domain.Node, error) {
	if net.ParseIP(strings.TrimSpace(node.Address)) != nil {
		return node, nil
	}
	if s.resolver == nil {
		return node, nil
	}

	addrs, err := s.resolver.LookupIPAddr(ctx, node.Address)
	if err != nil {
		s.logger.Debug("resolve node address fallback", "host", node.Address, "error", err)
		return node, nil
	}

	for _, addr := range addrs {
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			node.Address = ipv4.String()
			return node, nil
		}
	}
	if len(addrs) == 0 {
		s.logger.Debug("resolve node address returned no addresses", "host", node.Address)
		return node, nil
	}

	node.Address = addrs[0].IP.String()
	return node, nil
}

func (s *Service) probeSubscription(ctx context.Context, sub domain.Subscription, health map[string]domain.NodeHealth, failureThreshold int) []probe.Result {
	results := make([]probe.Result, 0, len(sub.Nodes))
	for _, node := range sub.Nodes {
		result := s.checker.Check(ctx, node)
		updated := probe.UpdateHealth(health[node.ID], result.Healthy, result.Latency, result.Checked, errString(result.Err), failureThreshold)
		updated.NodeID = node.ID
		updated.Score = probe.CalculateScore(updated, probe.DefaultScoreConfig()).Score
		health[node.ID] = updated
		result.Health = updated
		s.logDebug("probe result", "subscription", sub.ID, "node", node.ID, "healthy", result.Healthy, "latency", result.Latency, "error", errString(result.Err))
		results = append(results, result)
	}

	return results
}

func stableSubscriptionID(sourceType domain.SourceType, source string) string {
	sum := sha1.Sum([]byte(string(sourceType) + "|" + source))
	return "sub-" + hex.EncodeToString(sum[:])[:10]
}

func resolveAddSubscriptionID(subscriptions []domain.Subscription, next domain.Subscription) string {
	signature := subscriptionSignature(next)
	for _, existing := range subscriptions {
		if existing.SourceType != next.SourceType || strings.TrimSpace(existing.Source) != strings.TrimSpace(next.Source) {
			continue
		}
		if subscriptionSignature(existing) == signature {
			return existing.ID
		}
	}

	baseID := stableSubscriptionID(next.SourceType, next.Source)
	if !subscriptionIDExists(subscriptions, baseID) {
		return baseID
	}

	candidate := stableSubscriptionID(next.SourceType, next.Source+"|"+signature)
	if !subscriptionIDExists(subscriptions, candidate) {
		return candidate
	}

	for attempt := 2; ; attempt++ {
		candidate = stableSubscriptionID(next.SourceType, fmt.Sprintf("%s|%s|%d", next.Source, signature, attempt))
		if !subscriptionIDExists(subscriptions, candidate) {
			return candidate
		}
	}
}

func subscriptionIDExists(subscriptions []domain.Subscription, id string) bool {
	return slices.ContainsFunc(subscriptions, func(sub domain.Subscription) bool {
		return sub.ID == id
	})
}

func subscriptionSignature(sub domain.Subscription) string {
	nodes := make([]domain.Node, len(sub.Nodes))
	copy(nodes, sub.Nodes)
	for idx := range nodes {
		nodes[idx].SubscriptionID = ""
	}

	payload, err := json.Marshal(struct {
		SourceType   domain.SourceType `json:"source_type"`
		Source       string            `json:"source"`
		ProviderName string            `json:"provider_name"`
		DisplayName  string            `json:"display_name"`
		Nodes        []domain.Node     `json:"nodes"`
	}{
		SourceType:   sub.SourceType,
		Source:       strings.TrimSpace(sub.Source),
		ProviderName: strings.TrimSpace(sub.ProviderName),
		DisplayName:  strings.TrimSpace(sub.DisplayName),
		Nodes:        nodes,
	})
	if err != nil {
		return stableSubscriptionID(sub.SourceType, strings.TrimSpace(sub.Source)+"|"+strings.TrimSpace(sub.ProviderName)+"|"+strings.TrimSpace(sub.DisplayName))
	}

	sum := sha1.Sum(payload)
	return hex.EncodeToString(sum[:])
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

	status, err := s.waitForBackendRunning(ctx, subscriptionID, nodeID, mode)
	if err != nil {
		s.logWarn("backend status check failed", "subscription", subscriptionID, "node", nodeID, "mode", mode, "error", err.Error())
		_ = s.markConnectionFailed(ctx, subscriptionID, nodeID, mode, fmt.Sprintf("backend status check failed: %v", err))
		return fmt.Errorf("check backend status: %w", err)
	}
	if status.Running {
		s.logInfo("backend running confirmed", "subscription", subscriptionID, "node", nodeID, "mode", mode, "service_state", status.ServiceState)
		return nil
	}

	reason := "backend is not running"
	if strings.TrimSpace(status.ServiceState) != "" && status.ServiceState != "unknown" {
		reason = fmt.Sprintf("backend is not running (%s)", status.ServiceState)
	}
	s.logWarn("backend reported not running", "subscription", subscriptionID, "node", nodeID, "mode", mode, "service_state", status.ServiceState)
	_ = s.markConnectionFailed(ctx, subscriptionID, nodeID, mode, reason)
	return errors.New(reason)
}

func (s *Service) waitForBackendRunning(ctx context.Context, subscriptionID, nodeID string, mode domain.SelectionMode) (backend.RuntimeStatus, error) {
	checks := s.backendReadyChecks
	if checks < 1 {
		checks = 1
	}

	var last backend.RuntimeStatus
	for attempt := 1; attempt <= checks; attempt++ {
		status, err := s.backend.Status(ctx)
		if err != nil {
			return backend.RuntimeStatus{}, err
		}
		last = status
		if status.Running {
			return status, nil
		}
		if !backendStateMayStillBeStarting(status.ServiceState) || attempt == checks {
			return status, nil
		}

		s.logInfo(
			"backend not ready yet",
			"subscription", subscriptionID,
			"node", nodeID,
			"mode", mode,
			"service_state", status.ServiceState,
			"attempt", attempt,
		)

		if err := sleepWithContext(ctx, s.backendReadyDelay); err != nil {
			return backend.RuntimeStatus{}, err
		}
	}

	return last, nil
}

func backendStateMayStillBeStarting(serviceState string) bool {
	normalized := strings.ToLower(strings.TrimSpace(serviceState))
	switch {
	case normalized == "", normalized == "unknown":
		return true
	case strings.Contains(normalized, "starting"):
		return true
	case strings.Contains(normalized, "no instances"):
		return true
	default:
		return false
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

	if err := s.saveState(state); err != nil {
		return fmt.Errorf("save state after backend failure: %w", err)
	}

	s.logWarn("connection marked failed", "subscription", subscriptionID, "node", nodeID, "mode", mode, "reason", reason)
	return nil
}

func (s *Service) persistRestoreFailure(ctx context.Context, reason string) error {
	if s.firewall != nil {
		if err := s.firewall.Disable(ctx); err != nil {
			return fmt.Errorf("disable firewall after restore failure: %w", err)
		}
	}

	state, err := s.store.LoadState()
	if err != nil {
		return fmt.Errorf("load state after restore failure: %w", err)
	}

	state.ActiveSubscriptionID = ""
	state.ActiveNodeID = ""
	state.Mode = domain.SelectionModeDisconnected
	state.Connected = false
	state.LastFailureReason = reason
	if err := s.saveState(state); err != nil {
		return fmt.Errorf("save state after restore failure: %w", err)
	}

	settings, err := s.store.LoadSettings()
	if err == nil {
		settings.AutoMode = false
		settings.Mode = domain.SelectionModeDisconnected
		if saveErr := s.store.SaveSettings(settings); saveErr != nil {
			return fmt.Errorf("save settings after restore failure: %w", saveErr)
		}
	}

	s.logWarn("restore failure persisted", "reason", reason)
	return nil
}

func (s *Service) logDebug(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(msg, args...)
	}
}

func (s *Service) logInfo(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Info(msg, args...)
	}
}

func (s *Service) logWarn(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Warn(msg, args...)
	}
}

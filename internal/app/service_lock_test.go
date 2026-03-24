package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/probe"
)

var errStoreWriteLockRequired = errors.New("store write lock required")
var errNestedStoreWriteLock = errors.New("nested store write lock")

func TestAddSubscriptionUsesStoreWriteLock(t *testing.T) {
	t.Parallel()

	store := &lockRequiredStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := NewService(Dependencies{Store: store})

	_, err := service.AddSubscription(context.Background(), AddSubscriptionRequest{
		Raw: "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=reality&sni=edge.example.com&fp=chrome&pbk=public-key-1&sid=ab12cd34&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality",
	})
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}

	if store.lockCalls != 1 {
		t.Fatalf("expected one store lock call, got %d", store.lockCalls)
	}
	if len(store.subs) != 1 {
		t.Fatalf("expected one stored subscription, got %d", len(store.subs))
	}
}

func TestRefreshAllUsesSingleStoreWriteLock(t *testing.T) {
	t.Parallel()

	store := &lockRequiredStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:                 "sub-1",
				SourceType:         domain.SourceTypeRaw,
				Source:             "vless://11111111-1111-1111-1111-111111111111@node1.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Edge%20Reality",
				ProviderName:       "Demo VPN",
				DisplayName:        "Demo VPN",
				ProviderNameSource: domain.ProviderNameSourceManual,
				RefreshInterval:    domain.NewDuration(time.Hour),
			},
		},
	}
	service := NewService(Dependencies{Store: store})

	subs, err := service.RefreshAll(context.Background())
	if err != nil {
		t.Fatalf("refresh all: %v", err)
	}

	if store.lockCalls != 1 {
		t.Fatalf("expected one store lock call, got %d", store.lockCalls)
	}
	if len(subs) != 1 || len(subs[0].Nodes) != 1 {
		t.Fatalf("unexpected refreshed subscriptions: %+v", subs)
	}
}

func TestSetSettingAutoModeUsesSingleStoreWriteLock(t *testing.T) {
	t.Parallel()

	store := &lockRequiredStore{
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: "sub-1",
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
		},
		subs: []domain.Subscription{
			{
				ID: "sub-1",
				Nodes: []domain.Node{
					{
						ID:       "node-1",
						Name:     "Germany",
						Protocol: domain.ProtocolVLESS,
						Address:  "de.example.com",
						Port:     443,
						UUID:     "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
	}

	service := NewService(Dependencies{
		Store:   store,
		Backend: &recordingBackend{},
		Checker: fakeChecker{
			results: map[string]probe.Result{
				"node-1": {
					NodeID:  "node-1",
					Healthy: true,
					Latency: 50 * time.Millisecond,
					Checked: time.Now().UTC(),
				},
			},
		},
	})

	settings, err := service.SetSetting("auto-mode", "true")
	if err != nil {
		t.Fatalf("set setting auto-mode: %v", err)
	}

	if store.lockCalls != 1 {
		t.Fatalf("expected one store lock call, got %d", store.lockCalls)
	}
	if !settings.AutoMode || settings.Mode != domain.SelectionModeAuto {
		t.Fatalf("unexpected settings after auto-mode enable: %+v", settings)
	}
}

type lockRequiredStore struct {
	subs      []domain.Subscription
	settings  domain.Settings
	state     domain.RuntimeState
	lockDepth int
	lockCalls int
}

func (s *lockRequiredStore) WithWriteLock(fn func() error) error {
	if s.lockDepth != 0 {
		return errNestedStoreWriteLock
	}

	s.lockDepth++
	s.lockCalls++
	defer func() {
		s.lockDepth--
	}()

	return fn()
}

func (s *lockRequiredStore) LoadSubscriptions() ([]domain.Subscription, error) {
	if err := s.requireWriteLock(); err != nil {
		return nil, err
	}
	return s.subs, nil
}

func (s *lockRequiredStore) SaveSubscriptions(subs []domain.Subscription) error {
	if err := s.requireWriteLock(); err != nil {
		return err
	}
	s.subs = subs
	return nil
}

func (s *lockRequiredStore) LoadSettings() (domain.Settings, error) {
	if err := s.requireWriteLock(); err != nil {
		return domain.Settings{}, err
	}
	return s.settings, nil
}

func (s *lockRequiredStore) SaveSettings(settings domain.Settings) error {
	if err := s.requireWriteLock(); err != nil {
		return err
	}
	s.settings = settings
	return nil
}

func (s *lockRequiredStore) LoadState() (domain.RuntimeState, error) {
	if err := s.requireWriteLock(); err != nil {
		return domain.RuntimeState{}, err
	}
	return s.state, nil
}

func (s *lockRequiredStore) SaveState(state domain.RuntimeState) error {
	if err := s.requireWriteLock(); err != nil {
		return err
	}
	s.state = state
	return nil
}

func (s *lockRequiredStore) requireWriteLock() error {
	if s.lockDepth == 0 {
		return errStoreWriteLockRequired
	}
	return nil
}

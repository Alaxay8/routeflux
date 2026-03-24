package app

import (
	"context"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestSchedulerRunOnceRefreshesDueSubscription(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
		subs: []domain.Subscription{
			{
				ID:                 "sub-due",
				SourceType:         domain.SourceTypeRaw,
				Source:             "vless://11111111-1111-1111-1111-111111111111@due.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Due",
				ProviderName:       "Due VPN",
				DisplayName:        "Due VPN",
				ProviderNameSource: domain.ProviderNameSourceManual,
				LastUpdatedAt:      now.Add(-2 * time.Hour),
				RefreshInterval:    domain.NewDuration(time.Hour),
			},
			{
				ID:                 "sub-fresh",
				SourceType:         domain.SourceTypeRaw,
				Source:             "vless://11111111-1111-1111-1111-111111111111@fresh.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Fresh",
				ProviderName:       "Fresh VPN",
				DisplayName:        "Fresh VPN",
				ProviderNameSource: domain.ProviderNameSourceManual,
				LastUpdatedAt:      now.Add(-10 * time.Minute),
				RefreshInterval:    domain.NewDuration(time.Hour),
			},
		},
	}

	service := NewService(Dependencies{Store: store})
	scheduler := NewScheduler(service)
	scheduler.now = func() time.Time { return now }

	scheduler.RunOnce(context.Background())

	subs, err := service.ListSubscriptions()
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}

	if subs[0].LastUpdatedAt.Before(now) {
		t.Fatalf("expected due subscription to be refreshed, got %s", subs[0].LastUpdatedAt)
	}
	if !subs[1].LastUpdatedAt.Equal(now.Add(-10 * time.Minute)) {
		t.Fatalf("expected fresh subscription to stay untouched, got %s", subs[1].LastUpdatedAt)
	}
}

func TestSchedulerRunOnceRefreshesAndReconnectsActiveSubscription(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	activeNode := domain.Node{
		SubscriptionID: "sub-1",
		Name:           "Germany",
		ProviderName:   "Demo VPN",
		Protocol:       domain.ProtocolVLESS,
		Address:        "de.example.com",
		Port:           443,
		UUID:           "11111111-1111-1111-1111-111111111111",
		Encryption:     "none",
		Security:       "tls",
		ServerName:     "edge.example.com",
		Transport:      "ws",
		Path:           "/proxy",
		Host:           "cdn.example.com",
	}
	activeNode.ID = domain.StableNodeID(activeNode)

	store := &memoryStore{
		settings: domain.DefaultSettings(),
		state: domain.RuntimeState{
			ActiveSubscriptionID: "sub-1",
			ActiveNodeID:         activeNode.ID,
			Mode:                 domain.SelectionModeManual,
			Connected:            true,
		},
		subs: []domain.Subscription{
			{
				ID:                 "sub-1",
				SourceType:         domain.SourceTypeRaw,
				Source:             "vless://11111111-1111-1111-1111-111111111111@de.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Germany",
				ProviderName:       "Demo VPN",
				DisplayName:        "Demo VPN",
				ProviderNameSource: domain.ProviderNameSourceManual,
				LastUpdatedAt:      now.Add(-2 * time.Hour),
				RefreshInterval:    domain.NewDuration(time.Hour),
				Nodes:              []domain.Node{activeNode},
			},
		},
	}

	runtimeBackend := &recordingBackend{}
	service := NewService(Dependencies{
		Store:   store,
		Backend: runtimeBackend,
	})
	scheduler := NewScheduler(service)
	scheduler.now = func() time.Time { return now }

	scheduler.RunOnce(context.Background())

	if len(runtimeBackend.requests) != 1 {
		t.Fatalf("expected one backend apply during reconnect, got %d", len(runtimeBackend.requests))
	}
	if !store.state.Connected || store.state.ActiveSubscriptionID != "sub-1" || store.state.ActiveNodeID != activeNode.ID {
		t.Fatalf("unexpected state after refresh and reconnect: %+v", store.state)
	}
}

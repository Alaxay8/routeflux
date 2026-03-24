package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/store"
)

func TestDaemonOnceRefreshesDueSubscription(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	now := time.Now().UTC().Add(-2 * time.Hour)

	if err := fileStore.SaveSubscriptions([]domain.Subscription{
		{
			ID:                 "sub-1",
			SourceType:         domain.SourceTypeRaw,
			Source:             "vless://11111111-1111-1111-1111-111111111111@due.example.com:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Due",
			ProviderName:       "Due VPN",
			DisplayName:        "Due VPN",
			ProviderNameSource: domain.ProviderNameSourceManual,
			LastUpdatedAt:      now,
			RefreshInterval:    domain.NewDuration(time.Hour),
		},
	}); err != nil {
		t.Fatalf("save subscriptions: %v", err)
	}
	if err := fileStore.SaveSettings(domain.DefaultSettings()); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := fileStore.SaveState(domain.DefaultRuntimeState()); err != nil {
		t.Fatalf("save state: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--root", root, "daemon", "--once", "--tick", "10ms"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute daemon once: %v", err)
	}

	subs, err := fileStore.LoadSubscriptions()
	if err != nil {
		t.Fatalf("load subscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("unexpected subscriptions: %+v", subs)
	}
	if !subs[0].LastUpdatedAt.After(now) {
		t.Fatalf("expected subscription to be refreshed, got %s", subs[0].LastUpdatedAt)
	}
}

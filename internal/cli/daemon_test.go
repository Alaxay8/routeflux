package cli

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestDaemonOnceRestoresPersistedConnectionBeforeSchedulerLoop(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	if err := fileStore.SaveSubscriptions([]domain.Subscription{
		{
			ID:                 "sub-1",
			SourceType:         domain.SourceTypeRaw,
			Source:             "vless://11111111-1111-1111-1111-111111111111@203.0.113.10:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#Edge",
			ProviderName:       "Edge VPN",
			DisplayName:        "Edge VPN",
			ProviderNameSource: domain.ProviderNameSourceManual,
			LastUpdatedAt:      time.Now().UTC(),
			RefreshInterval:    domain.NewDuration(24 * time.Hour),
			Nodes: []domain.Node{
				{
					ID:             "node-1",
					SubscriptionID: "sub-1",
					Name:           "Edge",
					Protocol:       domain.ProtocolVLESS,
					Address:        "203.0.113.10",
					Port:           443,
					UUID:           "11111111-1111-1111-1111-111111111111",
				},
			},
		},
	}); err != nil {
		t.Fatalf("save subscriptions: %v", err)
	}
	if err := fileStore.SaveSettings(domain.DefaultSettings()); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := fileStore.SaveState(domain.RuntimeState{
		SchemaVersion:        1,
		ActiveSubscriptionID: "sub-1",
		ActiveNodeID:         "node-1",
		Mode:                 domain.SelectionModeManual,
		Connected:            true,
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	serviceScript := filepath.Join(root, "xray-service.sh")
	serviceBody := "#!/bin/sh\ncase \"$1\" in\nreload|start|stop)\n  exit 0\n  ;;\nstatus)\n  echo running\n  exit 0\n  ;;\n*)\n  exit 1\n  ;;\nesac\n"
	if err := os.WriteFile(serviceScript, []byte(serviceBody), 0o755); err != nil {
		t.Fatalf("write xray service script: %v", err)
	}

	xrayBinary := filepath.Join(root, "xray")
	xrayBody := "#!/bin/sh\nif [ \"$1\" = \"-test\" ] && [ \"$2\" = \"-config\" ] && [ -f \"$3\" ]; then\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(xrayBinary, []byte(xrayBody), 0o755); err != nil {
		t.Fatalf("write xray binary: %v", err)
	}

	oldService := os.Getenv("ROUTEFLUX_XRAY_SERVICE")
	oldBinary := os.Getenv("ROUTEFLUX_XRAY_BINARY")
	t.Cleanup(func() {
		if oldService == "" {
			_ = os.Unsetenv("ROUTEFLUX_XRAY_SERVICE")
		} else {
			_ = os.Setenv("ROUTEFLUX_XRAY_SERVICE", oldService)
		}
		if oldBinary == "" {
			_ = os.Unsetenv("ROUTEFLUX_XRAY_BINARY")
		} else {
			_ = os.Setenv("ROUTEFLUX_XRAY_BINARY", oldBinary)
		}
	})
	if err := os.Setenv("ROUTEFLUX_XRAY_SERVICE", serviceScript); err != nil {
		t.Fatalf("set xray service env: %v", err)
	}
	if err := os.Setenv("ROUTEFLUX_XRAY_BINARY", xrayBinary); err != nil {
		t.Fatalf("set xray binary env: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--root", root, "daemon", "--once", "--tick", "10ms"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute daemon once: %v", err)
	}

	state, err := fileStore.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !state.Connected {
		t.Fatal("expected restore to keep runtime connected")
	}

	if _, err := os.Stat(filepath.Join(root, "xray-config.json")); err != nil {
		t.Fatalf("expected restored xray config to be written: %v", err)
	}
}

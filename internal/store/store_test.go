package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/store"
)

func TestAtomicWriteJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	settings := domain.DefaultSettings()
	settings.LogLevel = "debug"

	if err := store.AtomicWriteJSON(path, settings); err != nil {
		t.Fatalf("atomic write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected file content")
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("unexpected temp files: %v", matches)
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	t.Parallel()

	fs := store.NewFileStore(t.TempDir())

	sub := domain.Subscription{
		ID:              "sub-1",
		ProviderName:    "Example",
		SourceType:      domain.SourceTypeRaw,
		Source:          "vless://...",
		LastUpdatedAt:   time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC),
		RefreshInterval: domain.NewDuration(time.Hour),
		Nodes: []domain.Node{
			{ID: "node-1", Name: "Node 1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
		},
	}

	state := domain.RuntimeState{
		SchemaVersion:        1,
		Mode:                 domain.SelectionModeManual,
		Connected:            true,
		ActiveSubscriptionID: sub.ID,
		ActiveNodeID:         "node-1",
	}

	settings := domain.DefaultSettings()

	if err := fs.SaveSubscriptions([]domain.Subscription{sub}); err != nil {
		t.Fatalf("save subscriptions: %v", err)
	}

	if err := fs.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := fs.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	subs, err := fs.LoadSubscriptions()
	if err != nil {
		t.Fatalf("load subscriptions: %v", err)
	}

	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("unexpected subscriptions: %+v", subs)
	}

	gotState, err := fs.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if gotState.ActiveNodeID != "node-1" {
		t.Fatalf("unexpected state: %+v", gotState)
	}

	gotSettings, err := fs.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if gotSettings.LogLevel != settings.LogLevel {
		t.Fatalf("unexpected settings: %+v", gotSettings)
	}
}

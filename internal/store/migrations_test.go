package store_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/store"
)

func TestLoadSettingsMigratesMissingSchemaVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "refresh_interval": "2h",
  "health_check_interval": "45s",
  "dns": {
    "mode": "remote",
    "transport": "plain",
    "servers": ["8.8.8.8"]
  },
  "mode": "manual",
  "log_level": "debug"
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if settings.SchemaVersion != domain.DefaultSettings().SchemaVersion {
		t.Fatalf("unexpected schema version: %d", settings.SchemaVersion)
	}
	if settings.RefreshInterval.Duration() != 2*time.Hour {
		t.Fatalf("unexpected refresh interval: %s", settings.RefreshInterval)
	}
	if settings.HealthCheckInterval.Duration() != 45*time.Second {
		t.Fatalf("unexpected health check interval: %s", settings.HealthCheckInterval)
	}
	if settings.DNS.Mode != domain.DNSModeRemote {
		t.Fatalf("unexpected dns mode: %s", settings.DNS.Mode)
	}
	if len(settings.DNS.Servers) != 1 || settings.DNS.Servers[0] != "8.8.8.8" {
		t.Fatalf("unexpected dns servers: %+v", settings.DNS.Servers)
	}
	if settings.Firewall.TransparentPort != domain.DefaultSettings().Firewall.TransparentPort {
		t.Fatalf("expected default firewall port, got %d", settings.Firewall.TransparentPort)
	}
}

func TestLoadSettingsPreservesLegacyFirewallTargetsWithoutDomains(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 2,
  "firewall": {
    "enabled": true,
    "target_cidrs": ["1.1.1.1", "8.8.8.8/32"]
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if !reflect.DeepEqual(settings.Firewall.TargetCIDRs, []string{"1.1.1.1", "8.8.8.8/32"}) {
		t.Fatalf("unexpected target cidrs: %+v", settings.Firewall.TargetCIDRs)
	}
	if len(settings.Firewall.TargetServices) != 0 {
		t.Fatalf("expected no target services, got %+v", settings.Firewall.TargetServices)
	}
	if len(settings.Firewall.TargetDomains) != 0 {
		t.Fatalf("expected no target domains, got %+v", settings.Firewall.TargetDomains)
	}
}

func TestLoadSettingsDecodesTargetServices(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 3,
  "firewall": {
    "enabled": true,
    "target_services": ["youtube", "telegram"],
    "target_domains": ["example.com"]
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if !reflect.DeepEqual(settings.Firewall.TargetServices, []string{"youtube", "telegram"}) {
		t.Fatalf("unexpected target services: %+v", settings.Firewall.TargetServices)
	}
	if !reflect.DeepEqual(settings.Firewall.TargetDomains, []string{"example.com"}) {
		t.Fatalf("unexpected target domains: %+v", settings.Firewall.TargetDomains)
	}
}

func TestLoadSettingsDecodesTargetServiceCatalog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 4,
  "firewall": {
    "target_service_catalog": {
      "openai": {
        "domains": ["openai.com", "chatgpt.com"],
        "cidrs": ["104.18.0.0/15"]
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	want := map[string]domain.FirewallTargetDefinition{
		"openai": {
			Domains: []string{"openai.com", "chatgpt.com"},
			CIDRs:   []string{"104.18.0.0/15"},
		},
	}
	if !reflect.DeepEqual(settings.Firewall.TargetServiceCatalog, want) {
		t.Fatalf("unexpected target service catalog:\nwant: %+v\n got: %+v", want, settings.Firewall.TargetServiceCatalog)
	}
}

func TestLoadSettingsRejectsFutureSchemaVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"schema_version":999}`), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	_, err := fileStore.LoadSettings()
	if err == nil {
		t.Fatal("expected future schema version to fail")
	}
	if !strings.Contains(err.Error(), "unsupported settings schema version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadStateMigratesMissingSchemaVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	stateJSON := `{
  "active_subscription_id": "sub-1",
  "active_node_id": "node-1",
  "mode": "auto",
  "connected": true,
  "last_switch_at": "2026-03-25T08:15:00Z"
}`
	if err := os.WriteFile(filepath.Join(root, "state.json"), []byte(stateJSON), 0o644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	state, err := fileStore.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if state.SchemaVersion != domain.DefaultRuntimeState().SchemaVersion {
		t.Fatalf("unexpected schema version: %d", state.SchemaVersion)
	}
	if !state.Connected {
		t.Fatal("expected state to stay connected")
	}
	if state.ActiveSubscriptionID != "sub-1" || state.ActiveNodeID != "node-1" {
		t.Fatalf("unexpected active selection: %+v", state)
	}
	if state.Health == nil {
		t.Fatal("expected health map to be initialized")
	}
	if state.LastRefreshAt == nil {
		t.Fatal("expected refresh map to be initialized")
	}
}

func TestLoadStateRejectsFutureSchemaVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)
	if err := os.WriteFile(filepath.Join(root, "state.json"), []byte(`{"schema_version":999}`), 0o644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	_, err := fileStore.LoadState()
	if err == nil {
		t.Fatal("expected future schema version to fail")
	}
	if !strings.Contains(err.Error(), "unsupported state schema version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSettingsRecoversCorruptJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var logs bytes.Buffer
	fileStore := store.NewFileStore(root).WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	corruptPath := filepath.Join(root, "settings.json")
	if err := os.WriteFile(corruptPath, []byte(`{"refresh_interval":`), 0o644); err != nil {
		t.Fatalf("write corrupt settings: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if !reflect.DeepEqual(settings, domain.DefaultSettings()) {
		t.Fatalf("expected default settings after recovery, got %+v", settings)
	}

	matches, err := filepath.Glob(filepath.Join(root, "settings.corrupt-*.json"))
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup, got %v", matches)
	}
	backupData, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupData) != `{"refresh_interval":` {
		t.Fatalf("unexpected backup contents: %q", backupData)
	}
	if !strings.Contains(logs.String(), "recovered corrupt persisted file") {
		t.Fatalf("expected recovery warning, got %q", logs.String())
	}
}

func TestLoadStateRecoversCorruptJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var logs bytes.Buffer
	fileStore := store.NewFileStore(root).WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	corruptPath := filepath.Join(root, "state.json")
	if err := os.WriteFile(corruptPath, []byte(`{"connected":"oops"}`), 0o644); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	state, err := fileStore.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !reflect.DeepEqual(state, domain.DefaultRuntimeState()) {
		t.Fatalf("expected default state after recovery, got %+v", state)
	}

	matches, err := filepath.Glob(filepath.Join(root, "state.corrupt-*.json"))
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup, got %v", matches)
	}
	if !strings.Contains(logs.String(), "recovered corrupt persisted file") {
		t.Fatalf("expected recovery warning, got %q", logs.String())
	}
}

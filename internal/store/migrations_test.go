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
	if settings.Firewall.BlockQUIC {
		t.Fatal("expected legacy settings to migrate to QUIC proxying by default")
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

	if settings.Firewall.Mode != domain.FirewallModeTargets {
		t.Fatalf("unexpected firewall mode: %q", settings.Firewall.Mode)
	}
	if !reflect.DeepEqual(settings.Firewall.Targets.CIDRs, []string{"1.1.1.1", "8.8.8.8/32"}) {
		t.Fatalf("unexpected target cidrs: %+v", settings.Firewall.Targets.CIDRs)
	}
	if len(settings.Firewall.Targets.Services) != 0 {
		t.Fatalf("expected no target services, got %+v", settings.Firewall.Targets.Services)
	}
	if len(settings.Firewall.Targets.Domains) != 0 {
		t.Fatalf("expected no target domains, got %+v", settings.Firewall.Targets.Domains)
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

	if settings.Firewall.Mode != domain.FirewallModeTargets {
		t.Fatalf("unexpected firewall mode: %q", settings.Firewall.Mode)
	}
	if !reflect.DeepEqual(settings.Firewall.Targets.Services, []string{"youtube", "telegram"}) {
		t.Fatalf("unexpected target services: %+v", settings.Firewall.Targets.Services)
	}
	if !reflect.DeepEqual(settings.Firewall.Targets.Domains, []string{"example.com"}) {
		t.Fatalf("unexpected target domains: %+v", settings.Firewall.Targets.Domains)
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

func TestLoadSettingsMigratesLegacyAntiTargetModeToSplit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 5,
  "firewall": {
    "target_mode": "bypass",
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

	if settings.Firewall.Mode != domain.FirewallModeSplit {
		t.Fatalf("unexpected firewall mode: %q", settings.Firewall.Mode)
	}
	if settings.Firewall.Split.DefaultAction != domain.FirewallDefaultActionProxy {
		t.Fatalf("unexpected split default action: %q", settings.Firewall.Split.DefaultAction)
	}
	if !reflect.DeepEqual(settings.Firewall.Split.Bypass.Domains, []string{"example.com"}) {
		t.Fatalf("unexpected split bypass domains: %+v", settings.Firewall.Split.Bypass.Domains)
	}
	if !reflect.DeepEqual(settings.Firewall.ModeDrafts, domain.FirewallModeDrafts{}) {
		t.Fatalf("expected empty mode drafts for schema 5, got %+v", settings.Firewall.ModeDrafts)
	}
}

func TestLoadSettingsDecodesFirewallModeDraftsAndCompositeCatalog(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 6,
  "firewall": {
    "mode_drafts": {
      "hosts": {
        "source_cidrs": ["192.168.1.150"]
      },
      "targets": {
        "target_services": ["daily"],
        "target_domains": ["example.com"],
        "target_cidrs": ["1.1.1.1"]
      },
      "anti_target": {
        "target_services": ["banking"],
        "target_domains": ["bank.example"],
        "target_cidrs": ["203.0.113.10"]
      }
    },
    "target_service_catalog": {
      "daily": {
        "services": ["youtube", "openai"],
        "domains": ["oaistatic.com"],
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

	if want := []string{"192.168.1.150"}; !reflect.DeepEqual(settings.Firewall.ModeDrafts.Hosts.SourceCIDRs, want) {
		t.Fatalf("unexpected hosts draft: %+v", settings.Firewall.ModeDrafts.Hosts.SourceCIDRs)
	}
	if want := []string{"daily"}; !reflect.DeepEqual(settings.Firewall.ModeDrafts.Targets.TargetServices, want) {
		t.Fatalf("unexpected targets draft services: %+v", settings.Firewall.ModeDrafts.Targets.TargetServices)
	}
	if want := []string{"banking"}; !reflect.DeepEqual(settings.Firewall.ModeDrafts.Split.Bypass.Services, want) {
		t.Fatalf("unexpected split bypass draft services: %+v", settings.Firewall.ModeDrafts.Split.Bypass.Services)
	}
	if want := []string{"bank.example"}; !reflect.DeepEqual(settings.Firewall.ModeDrafts.Split.Bypass.Domains, want) {
		t.Fatalf("unexpected split bypass draft domains: %+v", settings.Firewall.ModeDrafts.Split.Bypass.Domains)
	}
	if want := []string{"203.0.113.10"}; !reflect.DeepEqual(settings.Firewall.ModeDrafts.Split.Bypass.CIDRs, want) {
		t.Fatalf("unexpected split bypass draft cidrs: %+v", settings.Firewall.ModeDrafts.Split.Bypass.CIDRs)
	}
	if want := map[string]domain.FirewallTargetDefinition{
		"daily": {
			Services: []string{"youtube", "openai"},
			Domains:  []string{"oaistatic.com"},
			CIDRs:    []string{"104.18.0.0/15"},
		},
	}; !reflect.DeepEqual(settings.Firewall.TargetServiceCatalog, want) {
		t.Fatalf("unexpected target service catalog:\nwant: %+v\n got: %+v", want, settings.Firewall.TargetServiceCatalog)
	}
}

func TestLoadSettingsMigratesLegacyBlockQUICToDisabled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 6,
  "firewall": {
    "enabled": true,
    "source_cidrs": ["192.168.1.150"],
    "block_quic": true
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if settings.Firewall.Mode != domain.FirewallModeHosts {
		t.Fatalf("unexpected firewall mode: %q", settings.Firewall.Mode)
	}
	if !reflect.DeepEqual(settings.Firewall.Hosts, []string{"192.168.1.150"}) {
		t.Fatalf("unexpected migrated hosts: %+v", settings.Firewall.Hosts)
	}
	if settings.Firewall.BlockQUIC {
		t.Fatal("expected schema 6 settings to migrate to block_quic=false")
	}
}

func TestLoadSettingsPreservesBlockQUICForCurrentSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 7,
  "firewall": {
    "enabled": true,
    "source_cidrs": ["192.168.1.150"],
    "block_quic": true
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if settings.Firewall.Mode != domain.FirewallModeHosts {
		t.Fatalf("unexpected firewall mode: %q", settings.Firewall.Mode)
	}
	if !reflect.DeepEqual(settings.Firewall.Hosts, []string{"192.168.1.150"}) {
		t.Fatalf("unexpected migrated hosts: %+v", settings.Firewall.Hosts)
	}
	if !settings.Firewall.BlockQUIC {
		t.Fatal("expected current schema settings to preserve block_quic=true")
	}
}

func TestLoadSettingsPreservesDisableIPv6ForCurrentSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 8,
  "firewall": {
    "disable_ipv6": true
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if !settings.Firewall.DisableIPv6 {
		t.Fatal("expected current schema settings to preserve disable_ipv6=true")
	}
}

func TestLoadSettingsPreservesZapretSettingsForCurrentSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 10,
  "zapret": {
    "enabled": true,
    "selectors": {
      "services": ["youtube"],
      "domains": ["example.com"],
      "cidrs": ["1.1.1.1/32"]
    },
    "failback_success_threshold": 5
  }
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if !settings.Zapret.Enabled {
		t.Fatal("expected zapret.enabled=true")
	}
	if want := []string{"youtube"}; !reflect.DeepEqual(settings.Zapret.Selectors.Services, want) {
		t.Fatalf("unexpected zapret services: %+v", settings.Zapret.Selectors.Services)
	}
	if want := []string{"example.com"}; !reflect.DeepEqual(settings.Zapret.Selectors.Domains, want) {
		t.Fatalf("unexpected zapret domains: %+v", settings.Zapret.Selectors.Domains)
	}
	if len(settings.Zapret.Selectors.CIDRs) != 0 {
		t.Fatalf("expected zapret cidrs to be dropped, got %+v", settings.Zapret.Selectors.CIDRs)
	}
	if settings.Zapret.FailbackSuccessThreshold != 5 {
		t.Fatalf("unexpected zapret failback threshold: %d", settings.Zapret.FailbackSuccessThreshold)
	}
}

func TestLoadSettingsPreservesAutoExcludedNodesForCurrentSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fileStore := store.NewFileStore(root)

	settingsJSON := `{
  "schema_version": 10,
  "auto_excluded_nodes": [
    "sub-1/node-2",
    " sub-1/node-2 ",
    "sub-1/node-1"
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatalf("write settings file: %v", err)
	}

	settings, err := fileStore.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if want := []string{"sub-1/node-1", "sub-1/node-2"}; !reflect.DeepEqual(settings.AutoExcludedNodes, want) {
		t.Fatalf("unexpected auto excluded nodes: %+v", settings.AutoExcludedNodes)
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
	if state.ActiveTransport != domain.TransportModeProxy {
		t.Fatalf("expected connected legacy state to migrate to proxy transport, got %s", state.ActiveTransport)
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
	if info, err := os.Stat(matches[0]); err != nil {
		t.Fatalf("stat backup: %v", err)
	} else if got := info.Mode().Perm(); got != store.SecretFilePerm {
		t.Fatalf("unexpected backup mode: got %o want %o", got, store.SecretFilePerm)
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
	if info, err := os.Stat(matches[0]); err != nil {
		t.Fatalf("stat backup: %v", err)
	} else if got := info.Mode().Perm(); got != store.SecretFilePerm {
		t.Fatalf("unexpected backup mode: got %o want %o", got, store.SecretFilePerm)
	}
	if !strings.Contains(logs.String(), "recovered corrupt persisted file") {
		t.Fatalf("expected recovery warning, got %q", logs.String())
	}
}

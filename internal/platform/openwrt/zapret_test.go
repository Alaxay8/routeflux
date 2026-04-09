package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZapretManagerApplyAndDisableRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	serviceState := filepath.Join(root, "zapret.state")
	servicePath := filepath.Join(root, "init.d", "zapret")
	configPath := filepath.Join(root, "opt", "zapret", "config")
	configBackupPath := filepath.Join(root, "routeflux", "zapret-config.routeflux.bak")
	hostlistPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-hosts-user.txt")
	hostlistBackupPath := hostlistPath + ".routeflux.bak"
	ipListPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-ip-user.txt")
	ipListBackupPath := ipListPath + ".routeflux.bak"
	markerPath := filepath.Join(root, "routeflux", "zapret-managed.json")

	writeExecutableFile(t, servicePath, "#!/bin/sh\nset -eu\nstate=\""+serviceState+"\"\ncase \"${1:-}\" in\nstatus)\n  if [ -f \"$state\" ]; then\n    echo running\n    exit 0\n  fi\n  echo stopped\n  exit 1\n  ;;\nstart|restart)\n  : > \"$state\"\n  echo running\n  ;;\nstop)\n  rm -f \"$state\"\n  echo stopped\n  ;;\n*)\n  exit 1\n  ;;\nesac\n")
	writeTextFile(t, configPath, "MODE_FILTER=hostlist\nNFQWS_OPT=\"--filter-tcp=443 <HOSTLIST>\"\n", 0o644)
	writeTextFile(t, hostlistPath, "user.example.com\n", 0o644)
	writeTextFile(t, ipListPath, "203.0.113.0/24\n", 0o644)

	manager := ZapretManager{
		ServicePath:        servicePath,
		ConfigPath:         configPath,
		ConfigBackupPath:   configBackupPath,
		HostlistPath:       hostlistPath,
		HostlistBackupPath: hostlistBackupPath,
		IPListPath:         ipListPath,
		IPListBackupPath:   ipListBackupPath,
		MarkerPath:         markerPath,
	}

	status, err := manager.Apply(context.Background(), []string{"youtube.com", "googlevideo.com"}, []string{"91.108.0.0/16", "149.154.0.0/16"})
	if err != nil {
		t.Fatalf("apply zapret: %v", err)
	}
	if !status.Active || !status.Managed || !status.Installed {
		t.Fatalf("unexpected apply status: %+v", status)
	}

	hostlistData, err := os.ReadFile(hostlistPath)
	if err != nil {
		t.Fatalf("read managed hostlist: %v", err)
	}
	if !strings.Contains(string(hostlistData), "youtube.com") || !strings.Contains(string(hostlistData), "googlevideo.com") {
		t.Fatalf("unexpected managed hostlist: %s", hostlistData)
	}
	ipListData, err := os.ReadFile(ipListPath)
	if err != nil {
		t.Fatalf("read managed ip list: %v", err)
	}
	if !strings.Contains(string(ipListData), "91.108.0.0/16") || !strings.Contains(string(ipListData), "149.154.0.0/16") {
		t.Fatalf("unexpected managed ip list: %s", ipListData)
	}
	if _, err := os.Stat(hostlistBackupPath); err != nil {
		t.Fatalf("expected hostlist backup file: %v", err)
	}
	if _, err := os.Stat(ipListBackupPath); err != nil {
		t.Fatalf("expected ip list backup file: %v", err)
	}
	if _, err := os.Stat(configBackupPath); err != nil {
		t.Fatalf("expected config backup file: %v", err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read managed config: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, zapretConfigManagedStart) {
		t.Fatalf("expected managed config marker: %s", configText)
	}
	if !strings.Contains(configText, `ROUTEFLUX_ZAPRET_IPLIST="`+ipListPath+`"`) {
		t.Fatalf("expected managed config to declare ip list path: %s", configText)
	}
	if !strings.Contains(configText, "--ipset=$ROUTEFLUX_ZAPRET_IPLIST") {
		t.Fatalf("expected managed config to include ipset path: %s", configText)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected marker file: %v", err)
	}

	if err := manager.Disable(context.Background()); err != nil {
		t.Fatalf("disable zapret: %v", err)
	}

	restoredData, err := os.ReadFile(hostlistPath)
	if err != nil {
		t.Fatalf("read restored hostlist: %v", err)
	}
	if string(restoredData) != "user.example.com\n" {
		t.Fatalf("unexpected restored hostlist: %q", restoredData)
	}
	restoredIPData, err := os.ReadFile(ipListPath)
	if err != nil {
		t.Fatalf("read restored ip list: %v", err)
	}
	if string(restoredIPData) != "203.0.113.0/24\n" {
		t.Fatalf("unexpected restored ip list: %q", restoredIPData)
	}
	restoredConfigData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(restoredConfigData) != "MODE_FILTER=hostlist\nNFQWS_OPT=\"--filter-tcp=443 <HOSTLIST>\"\n" {
		t.Fatalf("unexpected restored config: %q", restoredConfigData)
	}
	if _, err := os.Stat(hostlistBackupPath); !os.IsNotExist(err) {
		t.Fatalf("expected hostlist backup removal, got err=%v", err)
	}
	if _, err := os.Stat(ipListBackupPath); !os.IsNotExist(err) {
		t.Fatalf("expected ip list backup removal, got err=%v", err)
	}
	if _, err := os.Stat(configBackupPath); !os.IsNotExist(err) {
		t.Fatalf("expected config backup removal, got err=%v", err)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("expected marker removal, got err=%v", err)
	}
}

func TestZapretManagerStatusDetectsExternalUnmanagedService(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	serviceState := filepath.Join(root, "zapret.state")
	servicePath := filepath.Join(root, "init.d", "zapret")
	configPath := filepath.Join(root, "opt", "zapret", "config")
	configBackupPath := filepath.Join(root, "routeflux", "zapret-config.routeflux.bak")
	hostlistPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-hosts-user.txt")
	ipListPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-ip-user.txt")

	writeExecutableFile(t, servicePath, "#!/bin/sh\nset -eu\nstate=\""+serviceState+"\"\ncase \"${1:-}\" in\nstatus)\n  if [ -f \"$state\" ]; then\n    echo running\n    exit 0\n  fi\n  echo stopped\n  exit 1\n  ;;\nstart|restart)\n  : > \"$state\"\n  echo running\n  ;;\nstop)\n  rm -f \"$state\"\n  echo stopped\n  ;;\n*)\n  exit 1\n  ;;\nesac\n")
	writeTextFile(t, configPath, "MODE_FILTER=hostlist\n", 0o644)
	writeTextFile(t, serviceState, "running\n", 0o644)

	manager := ZapretManager{
		ServicePath:        servicePath,
		ConfigPath:         configPath,
		ConfigBackupPath:   configBackupPath,
		HostlistPath:       hostlistPath,
		HostlistBackupPath: hostlistPath + ".bak",
		IPListPath:         ipListPath,
		IPListBackupPath:   ipListPath + ".bak",
		MarkerPath:         filepath.Join(root, "routeflux", "zapret-managed.json"),
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("status zapret: %v", err)
	}
	if status.Active || !status.ServiceActive || status.Managed {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !strings.Contains(status.LastReason, "outside RouteFlux") {
		t.Fatalf("unexpected last reason: %q", status.LastReason)
	}
}

func TestZapretManagerApplyAdoptsRunningService(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	serviceState := filepath.Join(root, "zapret.state")
	servicePath := filepath.Join(root, "init.d", "zapret")
	configPath := filepath.Join(root, "opt", "zapret", "config")
	configBackupPath := filepath.Join(root, "routeflux", "zapret-config.routeflux.bak")
	hostlistPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-hosts-user.txt")
	hostlistBackupPath := hostlistPath + ".routeflux.bak"
	ipListPath := filepath.Join(root, "opt", "zapret", "ipset", "zapret-ip-user.txt")
	ipListBackupPath := ipListPath + ".routeflux.bak"
	markerPath := filepath.Join(root, "routeflux", "zapret-managed.json")

	writeExecutableFile(t, servicePath, "#!/bin/sh\nset -eu\nstate=\""+serviceState+"\"\ncase \"${1:-}\" in\nstatus)\n  if [ -f \"$state\" ]; then\n    echo running\n    exit 0\n  fi\n  echo stopped\n  exit 1\n  ;;\nstart|restart)\n  : > \"$state\"\n  echo running\n  ;;\nstop)\n  rm -f \"$state\"\n  echo stopped\n  ;;\n*)\n  exit 1\n  ;;\nesac\n")
	writeTextFile(t, configPath, "MODE_FILTER=hostlist\nNFQWS_OPT=\"--filter-tcp=443 <HOSTLIST>\"\n", 0o644)
	writeTextFile(t, serviceState, "running\n", 0o644)
	writeTextFile(t, hostlistPath, "external.example.com\n", 0o644)
	writeTextFile(t, ipListPath, "203.0.113.0/24\n", 0o644)

	manager := ZapretManager{
		ServicePath:        servicePath,
		ConfigPath:         configPath,
		ConfigBackupPath:   configBackupPath,
		HostlistPath:       hostlistPath,
		HostlistBackupPath: hostlistBackupPath,
		IPListPath:         ipListPath,
		IPListBackupPath:   ipListBackupPath,
		MarkerPath:         markerPath,
	}

	status, err := manager.Apply(context.Background(), []string{"youtube.com"}, []string{"91.108.0.0/16"})
	if err != nil {
		t.Fatalf("apply zapret takeover: %v", err)
	}
	if !status.Active || !status.Managed || !status.ServiceActive {
		t.Fatalf("unexpected takeover status: %+v", status)
	}

	backupData, err := os.ReadFile(hostlistBackupPath)
	if err != nil {
		t.Fatalf("read backup hostlist: %v", err)
	}
	if string(backupData) != "external.example.com\n" {
		t.Fatalf("unexpected backup hostlist: %q", backupData)
	}
	backupIPData, err := os.ReadFile(ipListBackupPath)
	if err != nil {
		t.Fatalf("read backup ip list: %v", err)
	}
	if string(backupIPData) != "203.0.113.0/24\n" {
		t.Fatalf("unexpected backup ip list: %q", backupIPData)
	}
}

func TestBuildManagedZapretConfigSkipsIPProfileWithoutCIDRs(t *testing.T) {
	t.Parallel()

	base := "MODE_FILTER=hostlist\nNFQWS_OPT=\"--filter-tcp=443 <HOSTLIST>\"\n"
	got := buildManagedZapretConfig(base, "/opt/zapret/ipset/zapret-ip-user.txt", false)
	if got != base {
		t.Fatalf("expected original config without managed block, got %q", got)
	}
}

func writeExecutableFile(t *testing.T, path, content string) {
	t.Helper()
	writeTextFile(t, path, content, 0o755)
}

func writeTextFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

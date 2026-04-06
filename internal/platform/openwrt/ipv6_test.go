package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIPv6ManagerApplyDisablePersistsAndUpdatesRuntime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procRoot := filepath.Join(dir, "proc")
	writeIPv6ProcValue(t, procRoot, "all", "0\n")
	writeIPv6ProcValue(t, procRoot, "default", "0\n")
	writeIPv6ProcValue(t, procRoot, "br-lan", "0\n")

	manager := IPv6Manager{
		ProcRoot:         procRoot,
		SysctlConfigPath: filepath.Join(dir, "etc", "sysctl.d", "99-routeflux-ipv6.conf"),
	}

	if err := manager.Apply(context.Background(), true); err != nil {
		t.Fatalf("apply ipv6 disable: %v", err)
	}

	configData, err := os.ReadFile(manager.SysctlConfigPath)
	if err != nil {
		t.Fatalf("read sysctl config: %v", err)
	}
	text := string(configData)
	for _, want := range []string{
		"net.ipv6.conf.all.disable_ipv6=1",
		"net.ipv6.conf.default.disable_ipv6=1",
	} {
		if !containsLine(text, want) {
			t.Fatalf("expected sysctl config to contain %q\n%s", want, text)
		}
	}

	for _, iface := range []string{"all", "default", "br-lan"} {
		value, err := os.ReadFile(filepath.Join(procRoot, "sys", "net", "ipv6", "conf", iface, "disable_ipv6"))
		if err != nil {
			t.Fatalf("read runtime value for %s: %v", iface, err)
		}
		if string(value) != "1\n" {
			t.Fatalf("unexpected runtime value for %s: %q", iface, value)
		}
	}
}

func TestIPv6ManagerStatusReportsEnabledInterfaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procRoot := filepath.Join(dir, "proc")
	writeIPv6ProcValue(t, procRoot, "all", "1\n")
	writeIPv6ProcValue(t, procRoot, "default", "1\n")
	writeIPv6ProcValue(t, procRoot, "br-lan", "0\n")
	writeIPv6ProcValue(t, procRoot, "wan", "1\n")

	configPath := filepath.Join(dir, "etc", "sysctl.d", "99-routeflux-ipv6.conf")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("net.ipv6.conf.all.disable_ipv6=1\n"), 0o644); err != nil {
		t.Fatalf("write sysctl config: %v", err)
	}

	manager := IPv6Manager{
		ProcRoot:         procRoot,
		SysctlConfigPath: configPath,
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("ipv6 status: %v", err)
	}
	if !status.Available {
		t.Fatal("expected ipv6 status to be available")
	}
	if status.PersistentDisabled != true {
		t.Fatal("expected persistent disable flag to be detected")
	}
	if status.RuntimeDisabled {
		t.Fatal("expected runtime ipv6 to remain enabled while one interface is still active")
	}
	if !reflect.DeepEqual(status.EnabledInterfaces, []string{"br-lan"}) {
		t.Fatalf("unexpected enabled interfaces: %+v", status.EnabledInterfaces)
	}
}

func writeIPv6ProcValue(t *testing.T, procRoot, iface, value string) {
	t.Helper()

	path := filepath.Join(procRoot, "sys", "net", "ipv6", "conf", iface, "disable_ipv6")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir proc path for %s: %v", iface, err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("write proc value for %s: %v", iface, err)
	}
}

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if line == want {
			return true
		}
	}
	return false
}

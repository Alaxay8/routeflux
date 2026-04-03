package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsViewIncludesIPv6FailStateActions(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"handleDisableIPv6: function(ev) {",
		"[ 'firewall', 'set', 'ipv6', 'disable' ]",
		"_('IPv6 fail-state detected.')",
		"_('Disable IPv6 in RouteFlux')",
		"E('h3', {}, [ _('IPv6 State') ])",
		"this.renderCard(_('IPv6 Fail-State')",
		"this.renderCard(_('Enabled Interfaces')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view must contain IPv6 fail-state marker %q", want)
		}
	}
}

func readDiagnosticsViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "diagnostics.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

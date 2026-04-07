package luci_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirewallViewUsesSimplifiedRoutingCopy(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"RouteFlux - Routing",
		"RouteFlux status, the active connection, and the basic routing actions you need every day.",
		"System DNS",
		"RouteFlux Recommended DNS",
		"Keep Direct",
		"Excluded Devices",
		"Switch to Off or Bypass and save to replace it from LuCI.",
		"The current DNS profile was created outside this simplified LuCI flow.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing marker %q", want)
		}
	}
}

func TestFirewallViewDefinesReadableContrastTheme(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"--routeflux-routing-ink",
		"--routeflux-routing-panel-bg",
		"routeflux-routing-choice-selected",
		".routeflux-routing-panel .cbi-value-title",
		".routeflux-routing-inline > .cbi-button-action",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing readability marker %q", want)
		}
	}
}

func TestFirewallViewKeepsIntroOnThemeColors(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	if strings.Contains(source, "#routeflux-routing-root { --routeflux-routing-ink:#10263f; --routeflux-routing-ink-muted:#44566b; --routeflux-routing-ink-soft:#62758a; --routeflux-routing-panel-bg:linear-gradient(160deg, rgba(243, 248, 255, 0.98) 0%, rgba(230, 239, 249, 0.98) 56%, rgba(220, 232, 245, 0.98) 100%); --routeflux-routing-surface-bg:linear-gradient(180deg, rgba(255, 255, 255, 0.97) 0%, rgba(246, 250, 254, 0.97) 100%); --routeflux-routing-surface-strong:linear-gradient(180deg, #17324d 0%, #10243a 100%); color:var(--routeflux-routing-ink); }") {
		t.Fatal("routing root must not override intro text color")
	}
}

func TestFirewallViewRemovesAdvancedRoutingControls(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, forbidden := range []string{
		"routeflux-firewall-mode",
		"Transparent Port",
		"Block QUIC",
		"Disable IPv6",
		"routeflux-firewall-help",
		"firewall explain",
		"Targets",
		"Hosts",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("routing view must not contain %q", forbidden)
		}
	}
}

func TestFirewallViewPersistsOnlyOffAndBypassModes(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"'firewall', 'set', 'bypass'",
		"'firewall', 'draft', 'bypass'",
		"'firewall', 'disable'",
		"'dns', 'set', 'mode', 'system'",
		"'dns', 'set', 'default'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view must contain %q", want)
		}
	}
}

func TestLuCIMenuKeepsSubscriptionsRoutingZapretDiagnosticsAndAbout(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "root", "usr", "share", "luci", "menu.d", "luci-app-routeflux.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var payload map[string]struct {
		Title  string `json:"title"`
		Action struct {
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"action"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal menu json: %v", err)
	}

	if len(payload) != 6 {
		t.Fatalf("expected root + 5 LuCI entries, got %d", len(payload))
	}

	rootEntry, ok := payload["admin/services/routeflux"]
	if !ok {
		t.Fatal("missing RouteFlux root menu entry")
	}
	if rootEntry.Action.Path != "admin/services/routeflux/firewall" {
		t.Fatalf("root RouteFlux alias path mismatch: %q", rootEntry.Action.Path)
	}

	if _, exists := payload["admin/services/routeflux/overview"]; exists {
		t.Fatal("overview menu entry must be removed")
	}

	subscriptionsEntry, ok := payload["admin/services/routeflux/subscriptions"]
	if !ok {
		t.Fatal("missing subscriptions menu entry")
	}
	if subscriptionsEntry.Title != "Subscriptions" {
		t.Fatalf("unexpected subscriptions title %q", subscriptionsEntry.Title)
	}

	routingEntry, ok := payload["admin/services/routeflux/firewall"]
	if !ok {
		t.Fatal("missing routing menu entry")
	}
	if routingEntry.Title != "Routing" {
		t.Fatalf("unexpected routing title %q", routingEntry.Title)
	}

	zapretEntry, ok := payload["admin/services/routeflux/zapret"]
	if !ok {
		t.Fatal("missing zapret menu entry")
	}
	if zapretEntry.Title != "Zapret" {
		t.Fatalf("unexpected zapret title %q", zapretEntry.Title)
	}

	diagnosticsEntry, ok := payload["admin/services/routeflux/diagnostics"]
	if !ok {
		t.Fatal("missing diagnostics menu entry")
	}
	if diagnosticsEntry.Title != "Diagnostics" {
		t.Fatalf("unexpected diagnostics title %q", diagnosticsEntry.Title)
	}

	aboutEntry, ok := payload["admin/services/routeflux/about"]
	if !ok {
		t.Fatal("missing about menu entry")
	}
	if aboutEntry.Title != "About" {
		t.Fatalf("unexpected about title %q", aboutEntry.Title)
	}

	for _, forbidden := range []string{
		"admin/services/routeflux/overview",
		"admin/services/routeflux/dns",
		"admin/services/routeflux/settings",
		"admin/services/routeflux/logs",
		"admin/services/routeflux/services",
	} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("menu must not keep removed entry %q", forbidden)
		}
	}
}

func readFirewallViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "firewall.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

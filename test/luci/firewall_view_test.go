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

func TestFirewallViewUsesLightActionButtonsInsteadOfDarkNavy(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		".routeflux-routing-inline > .cbi-button-action { min-width:132px; min-height:52px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b;",
		".routeflux-routing-actions .cbi-button { min-height:48px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing light action marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"background:var(--routeflux-routing-surface-strong); color:#eef8ff;",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("routing view must not keep dark action marker %q", forbidden)
		}
	}
}

func TestFirewallViewUsesReadableLightInputsAndPlaceholders(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		".routeflux-theme-light .routeflux-routing-inline > .cbi-input-text, .routeflux-theme-light .routeflux-routing-inline > .cbi-input-select { border-color:rgba(125, 146, 170, 0.2); background:linear-gradient(180deg, rgba(251, 252, 254, 0.99) 0%, rgba(244, 248, 252, 0.99) 100%); color:#162638;",
		".routeflux-theme-light .routeflux-routing-inline .cbi-input-text::placeholder { color:#63768c; opacity:1; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing light input marker %q", want)
		}
	}
}

func TestFirewallViewUsesStructuredKeepDirectSelectorShell(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"routeflux-routing-selector-shell",
		"routeflux-routing-selector-head",
		"Direct selectors",
		"routeflux-routing-selector-meta",
		"routeflux-routing-item-value-code",
		"stay direct only while bypass mode is active",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing Keep Direct selector marker %q", want)
		}
	}
}

func TestFirewallViewRemovesDarkShellAroundRoutingPanels(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"#routeflux-routing-root.routeflux-theme-dark::before, #routeflux-routing-root.routeflux-theme-dark::after { display:none; }",
		"#routeflux-routing-root .routeflux-routing-layout { display:grid; gap:14px; padding:0; border:0; background:transparent; box-shadow:none;",
		"#routeflux-routing-root .routeflux-routing-layout::before { display:none; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing shell cleanup marker %q", want)
		}
	}
}

func TestFirewallViewUsesGreenSelectedChoiceState(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"routeflux-routing-choice-indicator",
		"routeflux-routing-choice-control",
		"rgba(34, 197, 94, 0.52)",
		"rgba(220, 252, 231, 0.99)",
		"content:\"\\\\2713\"",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing green choice marker %q", want)
		}
	}
}

func TestFirewallViewUsesPremiumDarkThemeChoicesAndSelectors(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"#routeflux-routing-root.routeflux-theme-dark { --routeflux-routing-ink:#eef4ff; --routeflux-routing-ink-muted:#a8b8ce; --routeflux-routing-ink-soft:#8ea0b8;",
		".routeflux-theme-dark .routeflux-routing-choice { border-color:rgba(145, 175, 220, 0.16); background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%);",
		".routeflux-theme-dark .routeflux-routing-choice-selected { border-color:rgba(34, 197, 94, 0.42); background:linear-gradient(180deg, rgba(13, 35, 28, 0.96) 0%, rgba(10, 24, 21, 1) 100%);",
		".routeflux-theme-dark .routeflux-routing-inline > .cbi-input-text, .routeflux-theme-dark .routeflux-routing-inline > .cbi-input-select { border-color:rgba(145, 175, 220, 0.16); background:rgba(6, 12, 22, 0.72); color:#eef4ff;",
		".routeflux-theme-dark .routeflux-routing-selector-shell { display:grid; gap:14px; padding:16px 18px; border:1px solid rgba(145, 175, 220, 0.14); border-radius:18px; background:rgba(8, 15, 26, 0.5);",
		".routeflux-theme-dark .routeflux-routing-item { background:linear-gradient(180deg, rgba(11, 18, 30, 0.94) 0%, rgba(8, 14, 24, 0.98) 100%); border-color:rgba(145, 175, 220, 0.14);",
		".routeflux-theme-dark .routeflux-routing-summary-shell { background:rgba(8, 15, 26, 0.58); border-color:rgba(145, 175, 220, 0.16);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("routing view missing dark-theme marker %q", want)
		}
	}
}

func TestFirewallViewReRendersAfterRoutingChoiceChange(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	modeStart := strings.Index(source, "handleModeChange: function(ev) {")
	dnsStart := strings.Index(source, "handleDNSChoiceChange: function(ev) {")
	serviceStart := strings.Index(source, "handleServiceChoiceChange: function(ev) {")
	if modeStart < 0 || dnsStart < 0 || serviceStart < 0 {
		t.Fatal("routing view missing expected change handlers")
	}

	modeBlock := source[modeStart:dnsStart]
	dnsBlock := source[dnsStart:serviceStart]

	if !strings.Contains(modeBlock, "this.renderIntoRoot();") {
		t.Fatal("handleModeChange must re-render the routing cards")
	}

	if !strings.Contains(dnsBlock, "this.renderIntoRoot();") {
		t.Fatal("handleDNSChoiceChange must re-render the dns cards")
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

func TestLuCIMenuKeepsSubscriptionsRoutingZapretDiagnosticsSettingsAndAbout(t *testing.T) {
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
		Order  int    `json:"order"`
		Title  string `json:"title"`
		Action struct {
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"action"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal menu json: %v", err)
	}

	if len(payload) != 7 {
		t.Fatalf("expected root + 6 LuCI entries, got %d", len(payload))
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

	settingsEntry, ok := payload["admin/services/routeflux/settings"]
	if !ok {
		t.Fatal("missing settings menu entry")
	}
	if settingsEntry.Title != "Settings" {
		t.Fatalf("unexpected settings title %q", settingsEntry.Title)
	}
	if settingsEntry.Order != 50 {
		t.Fatalf("unexpected settings order %d", settingsEntry.Order)
	}

	aboutEntry, ok := payload["admin/services/routeflux/about"]
	if !ok {
		t.Fatal("missing about menu entry")
	}
	if aboutEntry.Title != "About" {
		t.Fatalf("unexpected about title %q", aboutEntry.Title)
	}
	if aboutEntry.Order != 60 {
		t.Fatalf("unexpected about order %d", aboutEntry.Order)
	}

	for _, forbidden := range []string{
		"admin/services/routeflux/overview",
		"admin/services/routeflux/dns",
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

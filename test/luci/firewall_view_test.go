package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirewallViewTextInputsUseDirectHandlers(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"'input': function(ev) {\n\t\t\t\t\t\t\t\tthis.handleSelectorInputChange(key, ev);",
		"'input': function(ev) {\n\t\t\t\t\t\t\tthis.handleListInputChange(key, ev);",
		"'input': function(ev) {\n\t\t\t\t\t\t\t\tthis.handlePortInput(ev);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("firewall view missing direct input handler marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"'input': ui.createHandlerFn(this, 'handleSelectorInputChange', key)",
		"'input': ui.createHandlerFn(this, 'handleListInputChange', key)",
		"'input': ui.createHandlerFn(this, 'handlePortInput')",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("firewall view must not bind text input with createHandlerFn: %q", forbidden)
		}
	}
}

func TestFirewallViewActionButtonsPreventDefault(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, functionName := range []string{
		"handleAddService",
		"handleAddSelector",
		"handleAddListEntry",
		"handleRemoveSelector",
		"handleRemoveListEntry",
		"handleSaveSettings",
		"handleDisable",
	} {
		block := extractFunctionBlock(t, source, functionName)
		if !strings.Contains(block, "ev.preventDefault();") {
			t.Fatalf("%s must prevent default button submission", functionName)
		}
	}
}

func TestFirewallViewActionButtonsUseButtonType(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, handlerName := range []string{
		"'handleRemoveSelector', key, 'services'",
		"'handleRemoveListEntry', key",
		"'handleAddService', key",
		"'handleAddSelector', key",
		"'handleAddListEntry', key",
		"'handleSaveSettings'",
		"'handleDisable'",
	} {
		block := extractButtonBlock(t, source, handlerName)
		if !strings.Contains(block, "'type': 'button'") {
			t.Fatalf("button bound to %s must declare type=button", handlerName)
		}
	}
}

func TestFirewallViewPromotesBypassModeInLuCI(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"E('option', { 'value': 'bypass'",
		"_('Bypass')",
		"_('Keep Direct')",
		"_('Excluded Devices')",
		"current firewall config uses advanced split tunnelling created outside LuCI",
		"'firewall', 'draft', 'bypass'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("firewall view must contain %q", want)
		}
	}

	for _, forbidden := range []string{
		"E('option', { 'value': 'split'",
		"_('Split Tunnelling')",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("firewall view must not keep %q in main LuCI mode selector or copy", forbidden)
		}
	}
}

func TestFirewallViewUsesReadableBypassEditorStyling(t *testing.T) {
	t.Parallel()

	source := readFirewallViewSource(t)

	for _, want := range []string{
		"className += ' ' + trim(settings.className);",
		"var descriptionClassName = 'cbi-value-description';",
		"descriptionClassName += ' ' + trim(settings.descriptionClassName);",
		"'className': 'routeflux-firewall-editor-emphasis routeflux-firewall-editor-bypass'",
		"'descriptionClassName': 'routeflux-firewall-editor-description-strong'",
		"routeflux-firewall-editor-kicker",
		".routeflux-firewall-editor-head .cbi-value-description { color:var(--text-color-medium, #4f5f70);",
		".routeflux-firewall-editor-bypass { border-color:rgba(37, 99, 128, 0.36); background:linear-gradient(180deg, rgba(228, 238, 244, 0.98) 0%, rgba(214, 226, 235, 0.98) 100%); box-shadow:0 16px 30px rgba(22, 50, 74, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.62); }",
		".routeflux-firewall-editor-bypass .routeflux-firewall-editor-head h4 { color:#16324a !important; }",
		".routeflux-firewall-editor-bypass .routeflux-firewall-editor-grid .cbi-value-title { color:#284357 !important; }",
		".routeflux-firewall-editor-description-strong { color:#16324a !important; font-weight:500; }",
		".routeflux-firewall-inline .cbi-input-text::placeholder { color:rgba(71, 85, 105, 0.72); opacity:1; }",
		".routeflux-firewall-toggle { display:flex; gap:10px; align-items:flex-start; font-weight:600; color:var(--text-color-high, #17263a); }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("firewall view must contain readable styling marker %q", want)
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

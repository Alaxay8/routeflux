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

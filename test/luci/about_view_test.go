package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAboutViewExposesWhatsNewTab(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)

	for _, want := range []string{
		"handleShowWhatsNew",
		"showWhatsNewModal",
		"What\\'s New",
		"Changes included after the %s release, rewritten as practical user-facing updates.",
		"Update RouteFlux from LuCI",
		"Anti-target routing is more reliable",
		"Anti-target mode is now available",
		"v0.1.5",
		"routefluxUI.showModal(_('What\\'s New')",
		"routeflux-modal-whats-new",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view must contain %q", want)
		}
	}
}

func TestAboutViewWhatsNewHandlerPreventsDefault(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)
	block := extractAboutFunctionBlock(t, source, "handleShowWhatsNew")
	if !strings.Contains(block, "ev.preventDefault();") {
		t.Fatal("handleShowWhatsNew must prevent default button submission")
	}
	if !strings.Contains(block, "this.showWhatsNewModal();") {
		t.Fatal("handleShowWhatsNew must open the what's new modal")
	}
}

func TestAboutViewButtonsUseButtonType(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)

	for _, want := range []string{
		"ui.createHandlerFn(this, 'handleShowWhatsNew')",
		"ui.createHandlerFn(this, 'handleUpgrade')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view must contain %q", want)
		}
	}

	if count := strings.Count(source, "'type': 'button'"); count < 2 {
		t.Fatalf("about view must keep button type=button on tab and upgrade actions, got %d", count)
	}
}

func TestAboutViewWhatsNewModalHasCloseOnly(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)
	block := extractAboutFunctionBlock(t, source, "showWhatsNewModal")

	for _, want := range []string{
		"ui.hideModal();",
		"routefluxUI.showModal(_('What\\'s New')",
		"'actions': actions",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("showWhatsNewModal must contain %q", want)
		}
	}

	if strings.Contains(block, "Export JSON") {
		t.Fatal("what's new modal must not expose export action")
	}
}

func readAboutViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "about.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

func extractAboutFunctionBlock(t *testing.T, source, functionName string) string {
	t.Helper()

	startMarker := functionName + ": function"
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("function %s not found", functionName)
	}

	end := strings.Index(source[start:], "\n\t},")
	if end < 0 {
		t.Fatalf("end of function %s not found", functionName)
	}

	return source[start : start+end]
}

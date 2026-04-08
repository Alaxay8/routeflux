package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZapretViewIncludesFallbackControlsAndWarnings(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"RouteFlux - Zapret",
		"Automatic fallback",
		"Failback Success Threshold",
		"Zapret Test",
		"Start ZapretTest",
		"Return to Previous Route",
		"Available presets",
		"Refresh page",
		"running (external)",
		"take over that service the next time fallback or test mode starts",
		"switch this router into Zapret even while proxy nodes stay healthy",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing marker %q", want)
		}
	}
}

func TestZapretViewUsesDedicatedCommands(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"'--json', 'zapret', 'get'",
		"'--json', 'zapret', 'status'",
		"'--json', 'services', 'list'",
		"'zapret', 'set', 'enabled'",
		"'zapret', 'set', 'selectors'",
		"'zapret', 'set', 'failback-success-threshold'",
		"'zapret', 'test', 'start'",
		"'zapret', 'test', 'stop'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view must contain %q", want)
		}
	}
}

func TestZapretViewUsesGreenSelectedChoiceState(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"routeflux-zapret-choice-indicator",
		"routeflux-zapret-choice-control",
		"rgba(34, 197, 94, 0.52)",
		"rgba(220, 252, 231, 0.99)",
		"content:\"\\\\2713\"",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing green choice marker %q", want)
		}
	}
}

func readZapretViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "zapret.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

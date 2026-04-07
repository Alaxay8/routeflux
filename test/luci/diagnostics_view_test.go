package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsViewUsesCalmerHighLevelCopy(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"RouteFlux - Diagnostics",
		"A calmer snapshot of the current RouteFlux runtime.",
		"Quick status",
		"Advanced details",
		"Zapret and backend details",
		"File checks",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing marker %q", want)
		}
	}
}

func TestDiagnosticsViewMovesLowLevelChecksIntoDetails(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"routeflux-diagnostics-advanced",
		"summary', {}, [ _('IPv6 details') ]",
		"summary', {}, [ _('File checks') ]",
		"summary', {}, [ _('Zapret and backend details') ]",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing advanced marker %q", want)
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

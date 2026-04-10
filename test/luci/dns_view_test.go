package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDNSViewExplainsRelationshipToRouting(t *testing.T) {
	t.Parallel()

	source := readDNSViewSource(t)

	for _, want := range []string{
		"RouteFlux - DNS",
		"Manage all four RouteFlux DNS modes from LuCI.",
		"Routing only exposes System DNS and the Recommended DNS preset",
		"Recommended DNS preset",
		"Apply Recommended Preset",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("dns view missing marker %q", want)
		}
	}
}

func readDNSViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "dns.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

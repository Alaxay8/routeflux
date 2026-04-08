package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverviewViewShowsActivePingCard(t *testing.T) {
	t.Parallel()

	source := readOverviewViewSource(t)

	for _, want := range []string{
		"Active Ping",
		"Not checked",
		"Last known",
		"routeflux.overview.ping.latest",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("overview view missing ping marker %q", want)
		}
	}
}

func readOverviewViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "overview.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAboutViewUsesLatestInstallScriptUpgradeFlow(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)

	for _, want := range []string{
		"RouteFlux - About",
		"Update to new version",
		"/bin/sh",
		"wget -O /tmp/routeflux-install.sh \"https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh\" && sh /tmp/routeflux-install.sh",
		"Existing /etc/routeflux state is preserved by the installer.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view missing marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"'--upgrade'",
		"this.execText([ '--upgrade' ])",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("about view must not keep legacy upgrade flow marker %q", forbidden)
		}
	}
}

func TestAboutViewFormatsBuildDateAndSimplifiesWhatsNew(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)

	for _, want := range []string{
		"function formatBuildDate(value)",
		"routefluxUI.renderSummaryCard(_('Build Date'), formattedBuildDate)",
		"var versionText = 'RouteFlux ' + version + '\\nCommit: ' + commit + '\\nBuilt: ' + formattedBuildDate;",
		"Simplified LuCI interface",
		"LuCI now focuses on the everyday Subscriptions, Routing, and About flow with a cleaner and more compact interface.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view missing marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"Subscription expiration date is now shown",
		"Update RouteFlux from LuCI",
		"Bypass mode and target bundles",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("about view must not keep old what's new marker %q", forbidden)
		}
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

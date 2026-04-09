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
		"/usr/libexec/routeflux-self-update",
		"Existing /etc/routeflux state is preserved by the installer.",
		"function extractSelfUpdateStatus(output)",
		"status !== 'up-to-date'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view missing marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"'--upgrade'",
		"this.execText([ '--upgrade' ])",
		"fs.exec('/bin/sh'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("about view must not keep legacy upgrade flow marker %q", forbidden)
		}
	}
}

func TestAboutUpdateHelperRunsExactInstallCommand(t *testing.T) {
	t.Parallel()

	source := readSelfUpdateHelperSource(t)

	for _, want := range []string{
		"#!/bin/sh",
		"set -eu",
		"wget -O /tmp/routeflux-install.sh \"https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh\" && sh /tmp/routeflux-install.sh",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("self-update helper missing marker %q", want)
		}
	}
}

func TestRouteFluxACLAllowsOnlySelfUpdateHelper(t *testing.T) {
	t.Parallel()

	source := readACLSource(t)

	if !strings.Contains(source, "\"/usr/libexec/routeflux-self-update\": [ \"exec\" ]") {
		t.Fatal("acl must allow routeflux self-update helper")
	}

	for _, forbidden := range []string{
		"\"/bin/sh *\": [ \"exec\" ]",
		"\"/bin/sh\": [ \"exec\" ]",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("acl must not allow shell execution marker %q", forbidden)
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

func TestAboutViewUsesRouteFluxButtonsInsteadOfLegacyThemeClasses(t *testing.T) {
	t.Parallel()

	source := readAboutViewSource(t)

	for _, want := range []string{
		"'class': 'cbi-button cbi-button-action'",
		"'class': 'cbi-button cbi-button-apply'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("about view missing RouteFlux button marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"'class': 'btn cbi-button'",
		"'class': 'btn cbi-button cbi-button-action important'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("about view must not keep legacy theme class marker %q", forbidden)
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

func readSelfUpdateHelperSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "openwrt", "root", "usr", "libexec", "routeflux-self-update")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

func readACLSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "root", "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

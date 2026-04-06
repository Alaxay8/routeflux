package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsViewKeepsSafeGeneratedXrayPreview(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, forbidden := range []string{
		"handleInspectPreview",
		"showInspectPreviewModal",
		"'inspect', 'xray-safe'",
		"Export JSON",
		"copyTextToClipboard",
		"handleSpeedTest",
		"'inspect', 'speed'",
		"Speed Test",
		"Sort by last availability",
		"routeflux.subscriptions.sort_by_last_availability",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("subscriptions view must not keep advanced action marker %q", forbidden)
		}
	}
}

func TestSubscriptionsViewShowsExpirationDateAndRemoveAction(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"Expiration date",
		"handleRemoveSubscription",
		"cbi-button-negative",
		"[ _('Remove') ]",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing marker %q", want)
		}
	}
}

func TestSubscriptionsViewShowsRemainingTrafficMeter(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"Remaining traffic",
		"renderTrafficSummary",
		"routeflux-traffic-meter",
		"routeflux-traffic-meter-fill",
		"Unlimited",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing traffic marker %q", want)
		}
	}
}

func TestSubscriptionsViewKeepsOverviewSummaryAndCoreActions(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"RouteFlux - Subscriptions",
		"RouteFlux status, the active connection, and the basic subscription actions you need every day.",
		"Refresh Active",
		"Disconnect",
		"Active Provider",
		"Active Profile",
		"Active Node",
		"handleDisconnect",
		"handleRefreshActive",
		"handleConnectAuto",
		"handleConnectNode",
		"handleAdd",
		"handleRefreshSubscription",
		"handleRemoveSubscription",
		"handleRemoveAll",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing summary/core action marker %q", want)
		}
	}
}

func readSubscriptionsViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "subscriptions.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

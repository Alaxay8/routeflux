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

	for _, want := range []string{
		"handleInspectPreview",
		"showInspectPreviewModal",
		"Loading generated Xray JSON preview...",
		"'inspect', 'xray-safe'",
		"Sensitive values are redacted. DNS and DoH settings remain visible in this preview.",
		"Export JSON",
		"copyTextToClipboard",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view must expose safe Xray preview marker %q", want)
		}
	}

	if strings.Contains(source, "'inspect', 'xray'") {
		t.Fatal("subscriptions view must not call raw inspect xray")
	}
}

func TestSubscriptionsViewKeepsSpeedTestAction(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	if !strings.Contains(source, "'inspect', 'speed'") {
		t.Fatal("subscriptions view must keep speed test action")
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

func TestSubscriptionsViewKeepsRemoveActionVisibleWhenButtonsWrap(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-subscription-controls { display:grid; gap:8px; justify-items:end; min-width:0; max-width:100%; }",
		".routeflux-subscription-actions { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px; align-items:flex-start; max-width:100%; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing responsive action layout marker %q", want)
		}
	}

	if strings.Contains(source, ".routeflux-subscription-card { margin-bottom:16px; overflow:hidden; }") {
		t.Fatal("subscriptions view must not clip subscription actions with overflow:hidden")
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

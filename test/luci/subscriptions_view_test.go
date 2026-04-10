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
		"handleCopySubscriptionID",
		"Subscription ID copied to clipboard.",
		"routeflux-meta-copy-button",
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

func TestSubscriptionsViewShowsCompactStackColumnInNodeTable(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"formatSecurityLabel",
		"renderNodeStackCell",
		"responsiveTableCell(_('Stack')",
		"E('th', { 'class': 'th' }, [ _('Stack') ])",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing compact stack marker %q", want)
		}
	}
}

func TestSubscriptionsViewUsesStaticNodeTableLayout(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"overflow-x:visible",
		".routeflux-node-table { width:100%; min-width:0; table-layout:fixed; }",
		"routeflux-node-stack-vertical",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing static node table marker %q", want)
		}
	}
}

func TestSubscriptionsViewUsesDistinctVerticalStackChips(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"routeflux-node-stack-chip-protocol",
		"routeflux-node-stack-chip-transport",
		"routeflux-node-stack-chip-security",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing vertical stack chip marker %q", want)
		}
	}
}

func TestSubscriptionsViewShowsPingControlsAndStates(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"Check Ping",
		"Ping",
		"Recheck",
		"Last known",
		"Not checked",
		"routeflux.subscriptions.ping.latest",
		"'inspect', 'ping'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing ping marker %q", want)
		}
	}
}

func TestSubscriptionsViewPlacesRecheckInActionStack(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"routeflux-node-action-stack",
		"routeflux-node-actions-secondary",
		"handleRecheckPing",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing action stack marker %q", want)
		}
	}
}

func TestSubscriptionsViewStacksNodeActionsOnSmartphones(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-ping-actions, .routeflux-node-actions { flex-direction:column;",
		".routeflux-ping-actions .cbi-button, .routeflux-node-actions .cbi-button { width:100%; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing smartphone node action marker %q", want)
		}
	}
}

func TestSubscriptionsViewResetsNodeColumnWidthsOnSmartphones(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-node-table .routeflux-node-row > .td { width:100%; min-width:0;",
		".routeflux-node-table .routeflux-node-row > .td::before { content:attr(data-title);",
		"white-space:nowrap;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing smartphone node width reset marker %q", want)
		}
	}
}

func TestSubscriptionsViewCentersNodeCardsOnSmartphones(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-node-table .routeflux-node-row { margin-bottom:12px; padding:12px 14px;",
		"text-align:center;",
		".routeflux-node-table .routeflux-node-row > .td::before { content:attr(data-title); display:block;",
		".routeflux-node-stack, .routeflux-node-stack-vertical { justify-items:center; }",
		".routeflux-ping-cell, .routeflux-node-action-stack { justify-items:center; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing smartphone centering marker %q", want)
		}
	}
}

func TestSubscriptionsViewCentersHeroCardsAndSummaryBlocksOnSmartphones(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-subscriptions-hero, .routeflux-subscription-card, .routeflux-provider-group-header, .routeflux-auto-exclusions, .routeflux-node-details summary { text-align:center; }",
		".routeflux-overview-grid { justify-items:center; }",
		".routeflux-overview-grid .routeflux-card { width:100%; text-align:center; }",
		".routeflux-overview-grid .routeflux-card-accent, .routeflux-overview-grid .routeflux-card-label, .routeflux-overview-grid .routeflux-card-value { text-align:center; justify-self:center; margin-left:auto; margin-right:auto; }",
		".routeflux-page-hero-meta, .routeflux-subscription-controls { justify-items:center; }",
		".routeflux-page-hero-meta-item, .routeflux-page-hero-meta-label, .routeflux-page-hero-meta-value, .routeflux-action-status-group { text-align:center; justify-self:center; }",
		".routeflux-subscription-badges, .routeflux-node-status-badges, .routeflux-auto-exclusions-list, .routeflux-subscription-actions, .routeflux-ping-actions, .routeflux-node-actions { justify-content:center; }",
		".routeflux-traffic-meter, .routeflux-node-action-stack, .routeflux-node-heading-actions-label { margin-left:auto; margin-right:auto; }",
		".routeflux-node-heading-actions, .routeflux-node-cell-actions { text-align:center; padding-right:0; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing smartphone symmetry marker %q", want)
		}
	}
}

func TestSubscriptionsViewCentersSubscriptionMetaTableOnMobile(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-meta-table .tr { padding:10px 0; border-top:1px solid rgba(145, 175, 220, 0.1); text-align:center; }",
		".routeflux-subscription-card .routeflux-meta-table, .routeflux-subscription-card .routeflux-meta-table .tr, .routeflux-subscription-card .routeflux-meta-table .td, .routeflux-subscription-card .routeflux-meta-table .td.left { text-align:center !important; }",
		".routeflux-meta-table .td.routeflux-meta-label, .routeflux-subscription-card .routeflux-meta-table .td.routeflux-meta-label.left { width:100%; padding-bottom:4px; text-align:center !important; }",
		".routeflux-meta-table .td.routeflux-meta-value, .routeflux-subscription-card .routeflux-meta-table .td.routeflux-meta-value.left { padding-top:0; text-align:center !important; }",
		".routeflux-meta-copy-shell { display:grid; justify-items:center; gap:10px; width:100%; }",
		".routeflux-meta-copy-value { width:100%; text-align:center; }",
		".routeflux-meta-copy-button { margin:0 auto; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing centered mobile meta-table marker %q", want)
		}
	}
}

func TestSubscriptionsViewUsesSofterLightAccentsAndReadablePingState(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		".routeflux-theme-light .routeflux-ping-primary-live { color:#0f766e; }",
		".routeflux-theme-light .routeflux-ping-primary-down { color:#b91c1c; }",
		".routeflux-theme-light .routeflux-ping-primary-seed { color:#475569; }",
		".routeflux-theme-light .routeflux-ping-status-group { color:#1d4ed8; }",
		".routeflux-theme-light .routeflux-add-kicker { background:rgba(37, 99, 235, 0.08); color:#1d4ed8; }",
		".routeflux-theme-light .routeflux-add-field-shell { border-color:rgba(125, 146, 170, 0.18); background:linear-gradient(180deg, rgba(250, 252, 254, 0.98) 0%, rgba(243, 247, 251, 0.98) 100%);",
		".routeflux-theme-light .routeflux-subscription-badges .label.notice, .routeflux-theme-light .routeflux-node-active-badge .label.notice { border-color:rgba(22, 163, 74, 0.22); background:rgba(22, 163, 74, 0.1); color:#166534; }",
		".routeflux-theme-light .routeflux-provider-group-header { padding:12px 14px; border:1px solid rgba(125, 146, 170, 0.14); border-radius:16px; background:linear-gradient(180deg, rgba(250, 252, 254, 0.96) 0%, rgba(243, 247, 251, 0.96) 100%);",
		".routeflux-theme-light .routeflux-provider-group-title { color:#162638; }",
		".routeflux-theme-light .routeflux-provider-group-meta { color:#52667c; }",
		".routeflux-theme-light .routeflux-node-table { background:rgba(249, 251, 253, 0.92); border-color:rgba(125, 146, 170, 0.18); }",
		".routeflux-theme-light .routeflux-node-table .th { background:rgba(125, 146, 170, 0.08); color:#5c7085; }",
		".routeflux-theme-light .routeflux-subscription-actions .cbi-button-action, .routeflux-theme-light .routeflux-node-actions .cbi-button-action { border-color:rgba(37, 99, 235, 0.18); background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b;",
		".routeflux-theme-light .routeflux-subscription-actions .cbi-button-apply { border-color:rgba(37, 99, 235, 0.34); background:linear-gradient(180deg, #2563eb 0%, #1d4ed8 100%); color:#f8fbff;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing soft light marker %q", want)
		}
	}
}

func TestSubscriptionsViewSortsNodesByActiveThenPingLatency(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"nodePingSortMeta: function(subscriptionId, nodeId, status)",
		"compareNodeTableEntries: function(left, right)",
		"sortedEntries = nodes.map(L.bind(function(node, index)",
		"sortedEntries.sort(L.bind(this.compareNodeTableEntries, this));",
		"'ping_sort_bucket': pingSort.bucket",
		"'ping_latency_ms': pingSort.latency_ms",
		"'original_index': index",
		"return left.original_index - right.original_index;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing node sorting marker %q", want)
		}
	}
}

func TestSubscriptionsViewSupportsAutoExcludedNodes(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"autoExcludedNodeKey: function(subscriptionId, nodeId)",
		"isNodeAutoExcluded: function(status, subscriptionId, nodeId)",
		"handleToggleAutoExcluded: function(subscriptionId, nodeId, shouldExclude, ev)",
		"'settings', 'set', 'auto.excluded-nodes'",
		"Auto exclusions",
		"Auto mode skips these nodes when selecting the best route.",
		"Exclude",
		"Allow in Auto",
		"Auto excluded",
		"routeflux-auto-exclusions",
		"routeflux-node-auto-badge",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing auto exclusion marker %q", want)
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

func TestSubscriptionsViewUsesStyledAddSubscriptionPanel(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"routeflux-add-panel",
		"routeflux-add-panel-head",
		"routeflux-add-kicker",
		"routeflux-add-field-shell",
		"routeflux-add-format-list",
		"routeflux-add-format-badge",
		"Accepted input",
		"http(s) URL",
		"VLESS / VMess / Trojan / SS",
		"base64 payload",
		"Xray / 3x-ui JSON",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing add panel marker %q", want)
		}
	}
}

func TestSubscriptionsViewUsesFlagshipDarkShellMarkers(t *testing.T) {
	t.Parallel()

	source := readSubscriptionsViewSource(t)

	for _, want := range []string{
		"routeflux-page-shell routeflux-page-shell-subscriptions",
		"routeflux-page-hero",
		"routeflux-page-hero-actions",
		"routeflux-surface",
		"routeflux-data-table",
		"routeflux-section-heading",
		"routeflux-provider-group-header",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("subscriptions view missing flagship dark shell marker %q", want)
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

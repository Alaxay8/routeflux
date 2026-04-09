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

func TestDiagnosticsViewRemovesDarkShellAroundLightPanels(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"#routeflux-diagnostics-root.routeflux-theme-dark::before, #routeflux-diagnostics-root.routeflux-theme-dark::after { display:none; }",
		"#routeflux-diagnostics-root .routeflux-diagnostics-layout { display:grid; gap:14px; padding:0; border:0; background:transparent; box-shadow:none;",
		"#routeflux-diagnostics-root .routeflux-diagnostics-layout::before { display:none; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing shell cleanup marker %q", want)
		}
	}
}

func TestDiagnosticsViewUsesReadableLightPanelCopy(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"--routeflux-diagnostics-ink-muted:#405468",
		"--routeflux-diagnostics-ink-soft:#576d82",
		".routeflux-diagnostics-panel .cbi-section-descr { margin:0; color:var(--routeflux-diagnostics-ink-muted); font-size:15px; font-weight:500;",
		".routeflux-diagnostics-summary-list { margin:0; padding-left:18px; color:var(--routeflux-diagnostics-ink-soft); line-height:1.65; font-size:15px; }",
		".routeflux-diagnostics-summary-list li { color:var(--routeflux-diagnostics-ink); font-weight:500; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing readable copy marker %q", want)
		}
	}
}

func TestDiagnosticsViewUsesLightActionButtonsInsteadOfDarkNavy(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"--routeflux-diagnostics-ink:#102234",
		"--routeflux-diagnostics-ink-muted:#405468",
		"--routeflux-diagnostics-ink-soft:#576d82",
		".routeflux-diagnostics-actions .cbi-button { min-height:48px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing light action marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"color:#eef8ff;",
		"background:var(--routeflux-diagnostics-surface-strong);",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("diagnostics view must not keep dark action marker %q", forbidden)
		}
	}
}

func TestDiagnosticsViewKeepsFileChecksLightAndUndarkened(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"routeflux-diagnostics-file-table",
		".routeflux-diagnostics-file-table { background:rgba(250, 252, 254, 0.76); border-color:rgba(125, 145, 168, 0.24); box-shadow:none; }",
		".routeflux-diagnostics-file-table .th { background:rgba(125, 145, 168, 0.08); color:var(--routeflux-diagnostics-ink-soft); }",
		".routeflux-diagnostics-file-table .td { background:transparent; color:var(--routeflux-diagnostics-ink); }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing file table light marker %q", want)
		}
	}
}

func TestDiagnosticsViewUsesPremiumDarkThemePanels(t *testing.T) {
	t.Parallel()

	source := readDiagnosticsViewSource(t)

	for _, want := range []string{
		"#routeflux-diagnostics-root.routeflux-theme-dark { --routeflux-diagnostics-ink:#eef4ff; --routeflux-diagnostics-ink-muted:#a8b8ce; --routeflux-diagnostics-ink-soft:#8ea0b8;",
		".routeflux-theme-dark .routeflux-diagnostics-panel { border-color:rgba(145, 175, 220, 0.16); background:var(--routeflux-diagnostics-panel-bg);",
		".routeflux-theme-dark .routeflux-diagnostics-summary-shell { background:rgba(8, 15, 26, 0.58); border-color:rgba(145, 175, 220, 0.16);",
		".routeflux-theme-dark .routeflux-diagnostics-file-table { background:rgba(8, 15, 26, 0.5); border-color:rgba(145, 175, 220, 0.16);",
		".routeflux-theme-dark .routeflux-diagnostics-actions .cbi-button-action { border-color:rgba(120, 160, 214, 0.2); background:rgba(12, 20, 34, 0.82); color:#a8d7ff;",
		".routeflux-theme-dark .routeflux-diagnostics-advanced summary { color:#eef4ff; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("diagnostics view missing dark-theme marker %q", want)
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

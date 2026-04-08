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

func TestZapretViewRemovesDarkShellAroundLightPanels(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"#routeflux-zapret-root.routeflux-theme-dark::before, #routeflux-zapret-root.routeflux-theme-dark::after { display:none; }",
		"#routeflux-zapret-root .routeflux-zapret-layout { display:grid; gap:14px; padding:0; border:0; background:transparent; box-shadow:none;",
		"#routeflux-zapret-root .routeflux-zapret-layout::before { display:none; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing shell cleanup marker %q", want)
		}
	}
}

func TestZapretViewUsesReadableLightPanelCopy(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"--routeflux-zapret-ink-muted:#3e5368",
		"--routeflux-zapret-ink-soft:#5c7085",
		".routeflux-zapret-panel .cbi-section-descr { color:var(--routeflux-zapret-ink-muted); font-size:15px; font-weight:500;",
		".routeflux-zapret-summary-list { margin:0; padding-left:18px; color:var(--routeflux-zapret-ink-soft); line-height:1.65; font-size:15px; }",
		".routeflux-zapret-summary-list li { color:var(--routeflux-zapret-ink); font-weight:500; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing readable copy marker %q", want)
		}
	}
}

func TestZapretViewUsesLightActionButtonsInsteadOfDarkNavy(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		"--routeflux-zapret-ink:#102234",
		"--routeflux-zapret-ink-muted:#3e5368",
		"--routeflux-zapret-ink-soft:#5c7085",
		".routeflux-zapret-inline > .cbi-button-action, .routeflux-zapret-actions .cbi-button { min-height:52px; padding:0 18px; border:1px solid rgba(37, 99, 235, 0.18); border-radius:15px; background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#17324b;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing light action marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"color:#eef8ff;",
		"background:var(--routeflux-zapret-surface-strong);",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("zapret view must not keep dark action marker %q", forbidden)
		}
	}
}

func TestZapretViewUsesReadableLightInputsAndPlaceholders(t *testing.T) {
	t.Parallel()

	source := readZapretViewSource(t)

	for _, want := range []string{
		".routeflux-theme-light .routeflux-zapret-inline > .cbi-input-text, .routeflux-theme-light .routeflux-zapret-inline > .cbi-input-select { border-color:rgba(125, 146, 170, 0.2); background:linear-gradient(180deg, rgba(251, 252, 254, 0.99) 0%, rgba(244, 248, 252, 0.99) 100%); color:#162638;",
		".routeflux-theme-light .routeflux-zapret-inline > .cbi-input-text::placeholder { color:#63768c; opacity:1; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zapret view missing light input marker %q", want)
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

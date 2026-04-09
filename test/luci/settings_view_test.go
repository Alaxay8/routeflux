package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsViewShowsOnlyAppearanceControls(t *testing.T) {
	t.Parallel()

	source := readSettingsViewSource(t)

	for _, want := range []string{
		"RouteFlux - Settings",
		"Appearance",
		"Choose how RouteFlux pages should look inside LuCI.",
		"routeflux-settings-choice",
		"routeflux-settings-choice-selected",
		"routeflux-settings-choice-control",
		"routeflux-settings-appearance",
		"Dark",
		"Light",
		"'type': 'radio'",
		"'id': 'routeflux-settings-save'",
		"'type': 'button'",
		"content:\"\\\\2713\"",
		"handleAppearanceChange: function(ev)",
		"this.appearanceDraft = trim(ev && ev.currentTarget && ev.currentTarget.value).toLowerCase() === 'light' ? 'light' : 'dark';",
		"routefluxUI.setThemePreference(nextTheme);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("settings view missing appearance marker %q", want)
		}
	}
}

func TestSettingsViewRemovesLegacyRuntimeConfigurationControls(t *testing.T) {
	t.Parallel()

	source := readSettingsViewSource(t)

	for _, forbidden := range []string{
		"Refresh Interval",
		"Health Check Interval",
		"Switch Cooldown",
		"Latency Threshold",
		"Auto Mode",
		"Log Level",
		"Configuration",
		"routeflux-settings-refresh-interval",
		"routeflux-settings-health-check-interval",
		"routeflux-settings-switch-cooldown",
		"routeflux-settings-latency-threshold",
		"routeflux-settings-auto-mode",
		"routeflux-settings-log-level",
		"replaceChild(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("settings view must not keep legacy marker %q", forbidden)
		}
	}
}

func TestSettingsViewUsesReadableLightThemeChoices(t *testing.T) {
	t.Parallel()

	source := readSettingsViewSource(t)

	for _, want := range []string{
		".routeflux-theme-light .routeflux-settings-choice-control:focus-visible + .routeflux-settings-choice-indicator { outline:2px solid rgba(37, 99, 235, 0.22); outline-offset:3px; }",
		".routeflux-theme-light .routeflux-settings-choice-title { color:#162638; }",
		".routeflux-theme-light .routeflux-settings-choice-description { color:#41566d; }",
		".routeflux-theme-light .routeflux-settings-choice-selected .routeflux-settings-choice-note { background:rgba(22, 163, 74, 0.1); color:#166534; }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("settings view missing light-theme marker %q", want)
		}
	}
}

func readSettingsViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "settings.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

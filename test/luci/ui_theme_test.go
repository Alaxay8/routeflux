package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteFluxUISharedThemeSupportsDarkPremiumShell(t *testing.T) {
	t.Parallel()

	source := readRouteFluxUISource(t)

	for _, want := range []string{
		"routeflux-theme-dark",
		"routeflux-page-shell",
		"routeflux-page-hero",
		"routeflux-page-hero-actions",
		"routeflux-surface",
		"routeflux-data-table",
		"routeflux-button-primary",
		"routeflux-section-heading",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("shared RouteFlux UI missing theme marker %q", want)
		}
	}
}

func TestRouteFluxUISharedThemeSupportsLightModeAndPersistence(t *testing.T) {
	t.Parallel()

	source := readRouteFluxUISource(t)

	for _, want := range []string{
		"routeflux-theme-light",
		"routeflux.ui.theme.preference",
		"currentTheme: function()",
		"setThemePreference: function(value)",
		"withThemeClass: function(className)",
		"window.localStorage",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("shared RouteFlux UI missing light theme marker %q", want)
		}
	}
}

func TestRouteFluxUISharedThemeUsesReadableLightPalette(t *testing.T) {
	t.Parallel()

	source := readRouteFluxUISource(t)

	for _, want := range []string{
		"--routeflux-bg:#f3f6fb",
		"--routeflux-surface:#f8fbfd",
		"--routeflux-text-primary:#162638",
		"--routeflux-text-secondary:#41566d",
		"--routeflux-text-muted:#6a7c91",
		".routeflux-theme-light .routeflux-page-shell .cbi-section-descr, .routeflux-theme-light .routeflux-page-shell .cbi-value-description { color:var(--routeflux-text-secondary);",
		".routeflux-theme-light .routeflux-page-shell .cbi-button-apply, .routeflux-theme-light .routeflux-button-primary { border-color:rgba(37, 99, 235, 0.34); background:linear-gradient(180deg, #2563eb 0%, #1d4ed8 100%);",
		".routeflux-theme-light .routeflux-page-shell .cbi-button-action, .routeflux-theme-light .routeflux-button-secondary { border-color:rgba(37, 99, 235, 0.18); background:linear-gradient(180deg, rgba(243, 248, 253, 0.98) 0%, rgba(232, 240, 248, 0.98) 100%); color:#1d4ed8;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("shared RouteFlux UI missing readable light marker %q", want)
		}
	}
}

func readRouteFluxUISource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "routeflux", "ui.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

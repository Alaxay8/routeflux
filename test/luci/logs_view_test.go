package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogsViewExposesCopyButtonsForAllLogPanels(t *testing.T) {
	t.Parallel()

	source := readLogsViewSource(t)

	for _, want := range []string{
		"copyTextToClipboard: function(text)",
		"handleCopyLog: function(key, ev)",
		"ui.createHandlerFn(this, 'handleCopyLog', key)",
		"this.renderLogSection(\n\t\t\t_('RouteFlux')",
		"this.renderLogSection(\n\t\t\t_('Xray')",
		"this.renderLogSection(\n\t\t\t_('System Tail')",
		"Log copied to clipboard.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("logs view must contain %q", want)
		}
	}
}

func TestLogsViewPauseActionStopsPollingAndResetsOnRender(t *testing.T) {
	t.Parallel()

	source := readLogsViewSource(t)

	for _, want := range []string{
		"handleTogglePauseLogs: function(ev)",
		"setPauseButtonState: function(isPaused)",
		"ui.createHandlerFn(this, 'handleTogglePauseLogs')",
		"'id': 'routeflux-logs-pause-button'",
		"[ _('Pause') ]",
		"this.isLogsPaused = false;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("logs view must contain %q", want)
		}
	}

	startPolling := extractFunctionBlock(t, source, "startPolling")
	if !strings.Contains(startPolling, "this.isLogsPaused === true") {
		t.Fatal("startPolling must skip background updates while logs are paused")
	}
}

func TestLogsViewButtonsPreventDefaultAndUseButtonType(t *testing.T) {
	t.Parallel()

	source := readLogsViewSource(t)

	for _, functionName := range []string{
		"handleRefreshPage",
		"handleCleanLogs",
		"handleTogglePauseLogs",
		"handleCopyLog",
	} {
		block := extractFunctionBlock(t, source, functionName)
		if !strings.Contains(block, "ev.preventDefault();") {
			t.Fatalf("%s must prevent default button submission", functionName)
		}
	}

	for _, handlerName := range []string{
		"'handleRefreshPage'",
		"'handleTogglePauseLogs'",
		"'handleCleanLogs'",
		"'handleCopyLog', key",
	} {
		block := extractButtonBlock(t, source, handlerName)
		if !strings.Contains(block, "'type': 'button'") {
			t.Fatalf("button bound to %s must declare type=button", handlerName)
		}
	}

	pauseButton := extractButtonBlock(t, source, "'handleTogglePauseLogs'")
	if !strings.Contains(pauseButton, "'class': 'cbi-button cbi-button-action'") {
		t.Fatal("pause button must use the same action styling as refresh")
	}
}

func readLogsViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "logs.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

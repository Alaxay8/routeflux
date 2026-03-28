package luci_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServicesPageActionButtonsPreventDefault(t *testing.T) {
	t.Parallel()

	source := readServicesViewSource(t)

	for _, functionName := range []string{
		"handleSaveService",
		"handleDeleteService",
		"handleEditService",
		"handleClearForm",
	} {
		block := extractFunctionBlock(t, source, functionName)
		if !strings.Contains(block, "ev.preventDefault();") {
			t.Fatalf("%s must prevent default button submission", functionName)
		}
	}
}

func TestServicesPageActionButtonsUseButtonType(t *testing.T) {
	t.Parallel()

	source := readServicesViewSource(t)

	for _, handlerName := range []string{
		"'handleEditService'",
		"'handleDeleteService'",
		"'handleSaveService'",
		"'handleClearForm'",
	} {
		block := extractButtonBlock(t, source, handlerName)
		if !strings.Contains(block, "'type': 'button'") {
			t.Fatalf("button bound to %s must declare type=button", handlerName)
		}
	}
}

func readServicesViewSource(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	path := filepath.Join(root, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "services.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}

func extractFunctionBlock(t *testing.T, source, functionName string) string {
	t.Helper()

	startMarker := functionName + ": function"
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("function %s not found", functionName)
	}

	end := strings.Index(source[start:], "\n\t},")
	if end < 0 {
		t.Fatalf("end of function %s not found", functionName)
	}

	return source[start : start+end]
}

func extractButtonBlock(t *testing.T, source, handlerName string) string {
	t.Helper()

	end := strings.Index(source, handlerName)
	if end < 0 {
		t.Fatalf("handler %s not found", handlerName)
	}

	start := strings.LastIndex(source[:end], "E('button', {")
	if start < 0 {
		t.Fatalf("button block for %s not found", handlerName)
	}

	return source[start:end]
}

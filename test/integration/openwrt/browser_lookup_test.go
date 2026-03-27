package openwrt_test

import (
	"errors"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestResolveBrowserBinaryPrefersEnvOverride(t *testing.T) {
	t.Parallel()

	path, err := resolveBrowserBinary(
		func(name string) (string, bool) {
			if name == "ROUTEFLUX_OPENWRT_BROWSER_BIN" {
				return "/custom/chrome", true
			}
			return "", false
		},
		func(name string) (string, error) {
			t.Fatalf("unexpected LookPath call for %q", name)
			return "", nil
		},
	)
	if err != nil {
		t.Fatalf("resolve browser binary: %v", err)
	}
	if path != "/custom/chrome" {
		t.Fatalf("got %q, want /custom/chrome", path)
	}
}

func TestResolveBrowserBinaryResolvesEnvCommandName(t *testing.T) {
	t.Parallel()

	path, err := resolveBrowserBinary(
		func(name string) (string, bool) {
			if name == "CHROME_BIN" {
				return "chromium-browser", true
			}
			return "", false
		},
		func(name string) (string, error) {
			if name == "chromium-browser" {
				return "/usr/bin/chromium-browser", nil
			}
			return "", errors.New("not found")
		},
	)
	if err != nil {
		t.Fatalf("resolve browser binary: %v", err)
	}
	if path != "/usr/bin/chromium-browser" {
		t.Fatalf("got %q, want /usr/bin/chromium-browser", path)
	}
}

func TestResolveBrowserBinaryFindsSnapChromium(t *testing.T) {
	t.Parallel()

	path, err := resolveBrowserBinary(
		func(string) (string, bool) { return "", false },
		func(name string) (string, error) {
			if name == "/snap/bin/chromium" {
				return "/snap/bin/chromium", nil
			}
			return "", errors.New("not found")
		},
	)
	if err != nil {
		t.Fatalf("resolve browser binary: %v", err)
	}
	if path != "/snap/bin/chromium" {
		t.Fatalf("got %q, want /snap/bin/chromium", path)
	}
}

func TestResolveBrowserBinaryErrorsWhenNoBrowserFound(t *testing.T) {
	t.Parallel()

	_, err := resolveBrowserBinary(
		func(string) (string, bool) { return "", false },
		func(string) (string, error) { return "", errors.New("not found") },
	)
	if err == nil {
		t.Fatal("expected resolve browser binary to fail")
	}
}

func TestCDPBindingsSupportLoopbackIPAddressSpace(t *testing.T) {
	t.Parallel()

	if got := network.IPAddressSpaceLoopback.String(); got != "Loopback" {
		t.Fatalf("got %q, want Loopback", got)
	}
}

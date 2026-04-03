package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/pkg/api"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestInspectPathDetectsExecutableFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "routeflux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}

	status := inspectPath(path)

	if !status.Exists {
		t.Fatal("expected path to exist")
	}
	if !status.Executable {
		t.Fatal("expected file to be executable")
	}
	if status.Directory {
		t.Fatal("expected regular file, not directory")
	}
}

func TestInspectPathDetectsBrokenSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "routeflux")
	target := filepath.Join(dir, "missing-target")
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	status := inspectPath(path)

	if !status.Exists {
		t.Fatal("expected symlink path to exist")
	}
	if !status.IsSymlink {
		t.Fatal("expected symlink to be detected")
	}
	if status.Executable {
		t.Fatal("expected broken symlink to be non-executable")
	}
	if !strings.Contains(strings.ToLower(status.Error), "no such file or directory") {
		t.Fatalf("expected missing target error, got %q", status.Error)
	}
}

func TestRenderDiagnosticsTextIncludesTransparentQUICPolicy(t *testing.T) {
	t.Parallel()

	text := renderDiagnosticsText(diagnosticsSnapshot{
		Status: api.StatusResponse{
			State: domain.DefaultRuntimeState(),
		},
		TransparentQUICPolicy: "proxied",
	})

	if !strings.Contains(text, "transparent-quic-policy=proxied") {
		t.Fatalf("expected diagnostics text to include transparent quic policy, got %q", text)
	}
}

func TestRenderDiagnosticsTextIncludesIPv6FailState(t *testing.T) {
	t.Parallel()

	text := renderDiagnosticsText(diagnosticsSnapshot{
		Status: api.StatusResponse{
			State: domain.DefaultRuntimeState(),
		},
		TransparentQUICPolicy: "proxied",
		IPv6: diagnosticsIPv6Status{
			FailState:          true,
			RuntimeDisabled:    false,
			PersistentDisabled: false,
			EnabledInterfaces:  []string{"br-lan"},
			Message:            "Transparent routing does not intercept IPv6 traffic.",
		},
	})

	for _, want := range []string{
		"ipv6-fail-state=true",
		"ipv6-runtime-disabled=false",
		"ipv6-enabled-interfaces=br-lan",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected diagnostics text to include %q, got %q", want, text)
		}
	}
}

func TestDiagnosticsTransparentQUICPolicyMarksIncompatibleNodeAsBlocked(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings().Firewall
	settings.Enabled = true
	settings.Mode = domain.FirewallModeHosts
	settings.Hosts = []string{"192.168.1.150"}

	policy := diagnosticsTransparentQUICPolicy(settings, &domain.Node{
		Protocol:  domain.ProtocolVLESS,
		Transport: "tcp",
		Security:  "reality",
		Flow:      "xtls-rprx-vision",
	})

	if policy != "blocked-incompatible-node" {
		t.Fatalf("expected incompatible node policy, got %q", policy)
	}
}

func TestBuildDiagnosticsIPv6StatusFailsWhenTransparentRoutingLeavesIPv6Enabled(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings().Firewall
	settings.Enabled = true
	settings.Mode = domain.FirewallModeHosts
	settings.Hosts = []string{"192.168.1.150"}

	status := buildDiagnosticsIPv6Status(settings, domain.IPv6Status{
		Available:          true,
		RuntimeDisabled:    false,
		PersistentDisabled: false,
		EnabledInterfaces:  []string{"br-lan"},
	})

	if !status.FailState {
		t.Fatal("expected ipv6 diagnostics to mark transparent routing leak risk as failed")
	}
	if !strings.Contains(status.Message, "does not intercept IPv6") {
		t.Fatalf("unexpected ipv6 diagnostics message: %q", status.Message)
	}
}

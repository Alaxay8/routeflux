package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestDNSRuntimeManagerSystemResolversFromRunningDNSMasq(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procRoot := filepath.Join(dir, "proc")
	configPath := filepath.Join(dir, "etc", "dnsmasq.conf")
	confDir := filepath.Join(dir, "dnsmasq.d")
	resolvFile := filepath.Join(dir, "resolv.conf.auto")
	pidDir := filepath.Join(procRoot, "123")

	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir proc pid dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatalf("mkdir conf dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("dnsmasq\n"), 0o644); err != nil {
		t.Fatalf("write comm: %v", err)
	}
	cmdline := strings.Join([]string{
		"dnsmasq",
		"--conf-file=" + configPath,
	}, "\x00") + "\x00"
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("conf-dir="+confDir+"\nresolv-file="+resolvFile+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(resolvFile, []byte("nameserver 185.154.74.2\nnameserver 8.8.8.8\n"), 0o644); err != nil {
		t.Fatalf("write resolv file: %v", err)
	}

	manager := DNSRuntimeManager{ProcRoot: procRoot}
	resolvers, err := manager.SystemResolvers(context.Background())
	if err != nil {
		t.Fatalf("system resolvers: %v", err)
	}
	if got, want := strings.Join(resolvers, ","), "185.154.74.2,8.8.8.8"; got != want {
		t.Fatalf("unexpected system resolvers: got %q want %q", got, want)
	}
}

func TestDNSRuntimeManagerApplyWritesDNSOverrideSnippetAndRestarts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	procRoot := filepath.Join(dir, "proc")
	configPath := filepath.Join(dir, "etc", "dnsmasq.conf")
	confDir := filepath.Join(dir, "dnsmasq.d")
	resolvFile := filepath.Join(dir, "resolv.conf.auto")
	pidDir := filepath.Join(procRoot, "123")
	servicePath := writeExecutable(t, filepath.Join(dir, "dnsmasq-service"), "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \""+logPath+"\"\nexit 0\n")

	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir proc pid dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatalf("mkdir conf dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("dnsmasq\n"), 0o644); err != nil {
		t.Fatalf("write comm: %v", err)
	}
	cmdline := strings.Join([]string{
		"dnsmasq",
		"--conf-file=" + configPath,
	}, "\x00") + "\x00"
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("conf-dir="+confDir+"\nresolv-file="+resolvFile+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(resolvFile, []byte("nameserver 185.154.74.2\nnameserver 8.8.8.8\n"), 0o644); err != nil {
		t.Fatalf("write resolv file: %v", err)
	}

	manager := DNSRuntimeManager{
		ProcRoot:           procRoot,
		DNSMasqServicePath: servicePath,
	}
	settings := domain.DNSSettings{
		Mode:          domain.DNSModeSplit,
		Transport:     domain.DNSTransportDoH,
		Servers:       []string{"1.1.1.1", "1.0.0.1"},
		DirectDomains: []string{"domain:lan", "full:router.lan"},
	}

	if err := manager.Apply(context.Background(), settings, "127.0.0.1", 1053); err != nil {
		t.Fatalf("apply dns runtime: %v", err)
	}

	snippetPath := filepath.Join(confDir, "routeflux-dns.conf")
	data, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("read snippet: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"no-resolv",
		"server=127.0.0.1#1053",
		"server=/lan/185.154.74.2",
		"server=/lan/8.8.8.8",
		"server=/router.lan/185.154.74.2",
		"server=/router.lan/8.8.8.8",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("snippet missing %q\n%s", want, text)
		}
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	if !strings.Contains(string(calls), "restart") {
		t.Fatalf("expected dnsmasq restart, got %q", calls)
	}
}

func TestBuildDNSMasqRouteFluxDNSConfigRejectsAdvancedMatchers(t *testing.T) {
	t.Parallel()

	_, err := buildDNSMasqRouteFluxDNSConfig(domain.DNSSettings{
		Mode:          domain.DNSModeSplit,
		DirectDomains: []string{"regexp:.*"},
	}, []string{"8.8.8.8"}, "127.0.0.1", 1053)
	if err == nil {
		t.Fatal("expected advanced matcher to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported direct domain matcher") {
		t.Fatalf("unexpected error: %v", err)
	}
}

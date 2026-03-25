package xray

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestFormatDNSServersSupportsPlainAndDoH(t *testing.T) {
	t.Parallel()

	plain, err := formatDNSServers([]string{" 1.1.1.1 ", ""}, domain.DNSTransportPlain)
	if err != nil {
		t.Fatalf("format plain dns servers: %v", err)
	}
	if !reflect.DeepEqual(plain, []string{"1.1.1.1"}) {
		t.Fatalf("unexpected plain dns servers: %+v", plain)
	}

	doh, err := formatDNSServers([]string{"dns.google", "https://dns.quad9.net/dns-query"}, domain.DNSTransportDoH)
	if err != nil {
		t.Fatalf("format doh dns servers: %v", err)
	}
	if !reflect.DeepEqual(doh, []string{"https://dns.google/dns-query", "https://dns.quad9.net/dns-query"}) {
		t.Fatalf("unexpected doh dns servers: %+v", doh)
	}
}

func TestDNSBootstrapDomainsAndDirectDestinations(t *testing.T) {
	t.Parallel()

	servers := []string{
		"https://dns.google/dns-query",
		"https://dns.google/dns-query",
		"1.1.1.1",
		"https://1.0.0.1/dns-query",
		"https://dns.quad9.net/dns-query",
	}

	if got := dnsBootstrapDomains(servers); !reflect.DeepEqual(got, []string{"full:dns.google", "full:dns.quad9.net"}) {
		t.Fatalf("unexpected bootstrap domains: %+v", got)
	}

	ips, domains := directDNSDestinations(servers)
	if !reflect.DeepEqual(ips, []string{"1.1.1.1", "1.0.0.1"}) {
		t.Fatalf("unexpected direct dns ips: %+v", ips)
	}
	if !reflect.DeepEqual(domains, []string{"full:dns.google", "full:dns.quad9.net"}) {
		t.Fatalf("unexpected direct dns domains: %+v", domains)
	}
}

func TestOutboundForNodeSupportsVMessAndTrojan(t *testing.T) {
	t.Parallel()

	vmessOutbound, err := outboundForNode(domain.Node{
		Protocol:   domain.ProtocolVMess,
		Address:    "vmess.example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "aes-128-gcm",
		Security:   "tls",
		ServerName: "edge.example.com",
		Transport:  "grpc",
		Path:       "service",
	})
	if err != nil {
		t.Fatalf("vmess outbound: %v", err)
	}
	if vmessOutbound.Protocol != "vmess" {
		t.Fatalf("unexpected vmess outbound protocol: %q", vmessOutbound.Protocol)
	}

	trojanOutbound, err := outboundForNode(domain.Node{
		Protocol:   domain.ProtocolTrojan,
		Address:    "trojan.example.com",
		Port:       443,
		Password:   "secret",
		Security:   "tls",
		ServerName: "trojan.example.com",
		Transport:  "ws",
		Path:       "/trojan",
		Host:       "cdn.example.com",
	})
	if err != nil {
		t.Fatalf("trojan outbound: %v", err)
	}
	if trojanOutbound.Protocol != "trojan" {
		t.Fatalf("unexpected trojan outbound protocol: %q", trojanOutbound.Protocol)
	}

	if _, err := outboundForNode(domain.Node{Protocol: "unknown"}); err == nil {
		t.Fatal("expected unsupported protocol to fail")
	}
}

func TestGeneratorHandlesSplitDNSAndSelectedNodeErrors(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Nodes: []domain.Node{
			{
				ID:          "node-1",
				Protocol:    domain.ProtocolVLESS,
				Address:     "node1.example.com",
				Port:        443,
				UUID:        "11111111-1111-1111-1111-111111111111",
				Encryption:  "none",
				Security:    "reality",
				ServerName:  "edge.example.com",
				Fingerprint: "chrome",
				PublicKey:   "public-key-1",
				ShortID:     "ab12cd34",
			},
		},
		SelectedNodeID: "node-1",
		DNS: domain.DNSSettings{
			Mode:          domain.DNSModeSplit,
			Transport:     domain.DNSTransportDoH,
			Servers:       []string{"dns.google", "1.1.1.1"},
			Bootstrap:     []string{"9.9.9.9"},
			DirectDomains: []string{"domain:lan"},
		},
	}

	rendered, err := NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	if !strings.Contains(string(rendered), "\"dns\"") {
		t.Fatalf("expected dns block in config, got %s", rendered)
	}

	req.SelectedNodeID = "missing-node"
	if _, err := NewGenerator().Generate(req); err == nil {
		t.Fatal("expected missing selected node to fail")
	}
}

func TestInitdControllerCommandsAndRuntimeBackendDelegation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	scriptPath := writeExecutable(t, filepath.Join(dir, "xray-service.sh"), "#!/bin/sh\nprintf '%s\n' \"$1\" >> \""+logPath+"\"\nif [ \"$1\" = \"status\" ]; then\n  echo running\nfi\nexit 0\n")

	controller := InitdController{ScriptPath: scriptPath}
	if err := controller.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := controller.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := controller.Reload(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := controller.Status(context.Background()); err != nil {
		t.Fatalf("status: %v", err)
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read calls log: %v", err)
	}
	for _, want := range []string{"start", "stop", "reload", "status"} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("expected controller calls to contain %q, got %q", want, calls)
		}
	}

	tracker := &lifecycleController{status: backend.RuntimeStatus{Running: true, ServiceState: "running"}}
	runtimeBackend := NewRuntimeBackend(filepath.Join(dir, "config.json"), tracker).WithLogger(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := runtimeBackend.Start(context.Background()); err != nil {
		t.Fatalf("runtime start: %v", err)
	}
	if err := runtimeBackend.Stop(context.Background()); err != nil {
		t.Fatalf("runtime stop: %v", err)
	}
	if err := runtimeBackend.Reload(context.Background()); err != nil {
		t.Fatalf("runtime reload: %v", err)
	}
	if _, err := runtimeBackend.Status(context.Background()); err != nil {
		t.Fatalf("runtime status: %v", err)
	}
	if tracker.startCalls != 1 || tracker.stopCalls != 1 || tracker.reloadCalls != 1 {
		t.Fatalf("unexpected lifecycle controller calls: %+v", tracker)
	}
}

type lifecycleController struct {
	startCalls  int
	stopCalls   int
	reloadCalls int
	status      backend.RuntimeStatus
}

func (c *lifecycleController) Start(context.Context) error {
	c.startCalls++
	return nil
}

func (c *lifecycleController) Stop(context.Context) error {
	c.stopCalls++
	return nil
}

func (c *lifecycleController) Reload(context.Context) error {
	c.reloadCalls++
	return nil
}

func (c *lifecycleController) Status(context.Context) (backend.RuntimeStatus, error) {
	return c.status, nil
}

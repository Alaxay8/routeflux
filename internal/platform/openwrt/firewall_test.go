package openwrt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestBuildNFTablesRules(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetCIDRs:     []string{"1.1.1.1", "8.8.8.0/24"},
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	wants := []string{
		"table inet routeflux",
		"set target_v4",
		"set source_v4",
		"set bypass_v4",
		"1.1.1.1",
		"8.8.8.0/24",
		"ip daddr @bypass_v4 return",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"chain prerouting_mangle",
		"ip saddr @source_v4 udp dport 443",
		"meta mark set 0x1",
		"tproxy ip to :12345",
		"chain prerouting",
		"chain output",
		"priority -100",
	}
	for _, want := range wants {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesSupportsTargetDomainsWithoutStaticCIDRs(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetDomains:   []string{"youtube.com"},
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	wants := []string{
		"set target_v4",
		"set source_v4",
		"set bypass_v4",
		"type ipv4_addr",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"ip daddr @bypass_v4 return",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"ip saddr @source_v4 udp dport 443",
		"chain output",
	}
	for _, want := range wants {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesSupportsTargetServicesWithPresetCIDRs(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetServices:  []string{"telegram"},
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	for _, want := range []string{
		"91.108.0.0/16",
		"149.154.0.0/16",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"ip saddr @source_v4 udp dport 443",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesSupportsCustomTargetServices(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetServices:  []string{"openai"},
		TargetServiceCatalog: map[string]domain.FirewallTargetDefinition{
			"openai": {
				CIDRs: []string{"104.18.0.0/15"},
			},
		},
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	for _, want := range []string{
		"104.18.0.0/15",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"ip saddr @source_v4 udp dport 443",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	_, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetCIDRs:     []string{"not-an-ip"},
	})
	if err == nil {
		t.Fatal("expected invalid target to fail")
	}
}

func TestBuildNFTablesRulesInterceptsUDPForTargetDomainsWithoutDroppingQUIC(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetDomains:   []string{"youtube.com"},
		BlockQUIC:       true,
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	for _, want := range []string{
		"chain prerouting_mangle",
		"ip saddr @source_v4 udp dport 443",
		"meta mark set 0x1",
		"tproxy ip to :12345",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
	if strings.Contains(rules, "udp dport 443 drop") {
		t.Fatalf("rules should not drop quic once udp interception is enabled\n%s", rules)
	}
}

func TestBuildNFTablesRulesForBypassTargetsSkipsTargetSetAndOutputChain(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetMode:      domain.FirewallTargetModeBypass,
		TargetDomains:   []string{"youtube.com"},
		BlockQUIC:       true,
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	for _, want := range []string{
		"set source_v4",
		"set bypass_v4",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"chain prerouting_mangle",
		"ip saddr @source_v4 udp dport 443",
		"meta mark set 0x1",
		"tproxy ip to :12345",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}

	for _, unwanted := range []string{
		"set target_v4",
		"chain output",
	} {
		if strings.Contains(rules, unwanted) {
			t.Fatalf("rules unexpectedly contain %q\n%s", unwanted, rules)
		}
	}
}

func TestBuildNFTablesRulesForSourceHosts(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		SourceCIDRs:     []string{"192.168.1.150"},
		BlockQUIC:       true,
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	wants := []string{
		"set bypass_v4",
		"127.0.0.0/8",
		"192.168.0.0/16",
		"ip daddr @bypass_v4 return",
		"set source_v4",
		"192.168.1.150",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"chain prerouting_mangle",
		"ip saddr @source_v4 udp dport 443",
		"meta mark set 0x1",
		"tproxy ip to :12345",
	}
	for _, want := range wants {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesForSourceHostsBypassesLocalDestinationsFirst(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		SourceCIDRs:     []string{"192.168.1.150"},
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	bypassIndex := strings.Index(rules, "ip daddr @bypass_v4 return")
	redirectIndex := strings.Index(rules, "ip saddr @source_v4 tcp dport != 12345 redirect to :12345")
	if bypassIndex < 0 {
		t.Fatalf("rules missing bypass rule\n%s", rules)
	}
	if redirectIndex < 0 {
		t.Fatalf("rules missing source redirect rule\n%s", rules)
	}
	if bypassIndex > redirectIndex {
		t.Fatalf("expected bypass rule before redirect rule\n%s", rules)
	}
}

func TestBuildNFTablesRulesForSourceHostRange(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		SourceCIDRs:     []string{"192.168.1.150-192.168.1.159"},
		BlockQUIC:       true,
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	wants := []string{
		"set source_v4",
		"192.168.1.150/31",
		"192.168.1.152/29",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"ip saddr @source_v4 udp dport 443",
	}
	for _, want := range wants {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestBuildNFTablesRulesForAllSourceHosts(t *testing.T) {
	t.Parallel()

	rules, err := BuildNFTablesRules(domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		SourceCIDRs:     []string{"all"},
		BlockQUIC:       true,
	})
	if err != nil {
		t.Fatalf("build rules: %v", err)
	}

	wants := []string{
		"set source_v4",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"ip saddr @source_v4 tcp dport != 12345 redirect to :12345",
		"chain prerouting_mangle",
		"ip saddr @source_v4 udp dport 443",
		"meta mark set 0x1",
		"tproxy ip to :12345",
	}
	for _, want := range wants {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules missing %q\n%s", want, rules)
		}
	}
}

func TestFirewallDisableIgnoresMissingNFTBinary(t *testing.T) {
	t.Parallel()

	rulesPath := filepath.Join(t.TempDir(), "routeflux-firewall.nft")
	if err := os.WriteFile(rulesPath, []byte("table inet routeflux {}"), 0o644); err != nil {
		t.Fatalf("write rules file: %v", err)
	}

	manager := FirewallManager{
		NFTPath:   filepath.Join(t.TempDir(), "missing-nft"),
		RulesPath: rulesPath,
	}

	if err := manager.Disable(context.Background()); err != nil {
		t.Fatalf("disable firewall: %v", err)
	}

	if _, err := os.Stat(rulesPath); !os.IsNotExist(err) {
		t.Fatalf("expected rules file to be removed, stat err=%v", err)
	}
}

func TestFirewallManagerApplyConfiguresUDPPolicyRouting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	nftPath := writeExecutable(t, filepath.Join(dir, "nft"), "#!/bin/sh\nprintf 'nft %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	ipPath := writeExecutable(t, filepath.Join(dir, "ip"), "#!/bin/sh\nprintf 'ip %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	manager := FirewallManager{
		NFTPath:   nftPath,
		IPPath:    ipPath,
		RulesPath: filepath.Join(dir, "routeflux-firewall.nft"),
	}

	settings := domain.FirewallSettings{
		Enabled:         true,
		TransparentPort: 12345,
		TargetMode:      domain.FirewallTargetModeBypass,
		TargetDomains:   []string{"youtube.com"},
		BlockQUIC:       true,
	}

	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatalf("apply firewall: %v", err)
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}

	for _, want := range []string{
		"ip rule del fwmark 0x1/0x1 table 100 priority 1000",
		"ip route del local 0.0.0.0/0 dev lo table 100",
		"ip route add local 0.0.0.0/0 dev lo table 100",
		"ip rule add fwmark 0x1/0x1 table 100 priority 1000",
	} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("calls missing %q\n%s", want, calls)
		}
	}
}

func TestFirewallManagerDisableRemovesUDPPolicyRouting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	nftPath := writeExecutable(t, filepath.Join(dir, "nft"), "#!/bin/sh\nprintf 'nft %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	ipPath := writeExecutable(t, filepath.Join(dir, "ip"), "#!/bin/sh\nprintf 'ip %s\\n' \"$*\" >> \""+logPath+"\"\nexit 0\n")
	manager := FirewallManager{
		NFTPath:   nftPath,
		IPPath:    ipPath,
		RulesPath: filepath.Join(dir, "routeflux-firewall.nft"),
	}

	if err := manager.Disable(context.Background()); err != nil {
		t.Fatalf("disable firewall: %v", err)
	}

	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}

	for _, want := range []string{
		"ip rule del fwmark 0x1/0x1 table 100 priority 1000",
		"ip route del local 0.0.0.0/0 dev lo table 100",
	} {
		if !strings.Contains(string(calls), want) {
			t.Fatalf("calls missing %q\n%s", want, calls)
		}
	}
}

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
		"chain output",
	}
	for _, want := range wants {
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

func TestBuildNFTablesRulesBlocksQUICForTargetDomains(t *testing.T) {
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

	if !strings.Contains(rules, "ip saddr @source_v4 udp dport 443 drop") {
		t.Fatalf("rules missing target quic drop\n%s", rules)
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
		"ip saddr @source_v4 udp dport 443 drop",
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
		"ip saddr @source_v4 udp dport 443 drop",
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

package openwrt

import (
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
		"1.1.1.1",
		"8.8.8.0/24",
		"tcp dport != 12345 redirect to :12345",
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

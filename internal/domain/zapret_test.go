package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultSettingsIncludeZapretDefaults(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	if settings.Zapret.Enabled {
		t.Fatal("expected zapret to be disabled by default")
	}
	if settings.Zapret.FailbackSuccessThreshold != 3 {
		t.Fatalf("unexpected default zapret failback threshold: %d", settings.Zapret.FailbackSuccessThreshold)
	}
}

func TestDefaultRuntimeStateUsesDirectTransport(t *testing.T) {
	t.Parallel()

	state := DefaultRuntimeState()
	if state.ActiveTransport != TransportModeDirect {
		t.Fatalf("unexpected default transport: %s", state.ActiveTransport)
	}
}

func TestNormalizeZapretSettingsCanonicalizesDeprecatedServiceAliases(t *testing.T) {
	t.Parallel()

	got := NormalizeZapretSettings(ZapretSettings{
		Enabled: true,
		Selectors: FirewallSelectorSet{
			Services: []string{"telegram-web", "telegram"},
			CIDRs:    []string{"1.1.1.1/32"},
		},
	})

	if want := []string{"telegram"}; !reflect.DeepEqual(got.Selectors.Services, want) {
		t.Fatalf("unexpected canonical zapret services: %+v", got.Selectors.Services)
	}
	if len(got.Selectors.CIDRs) != 0 {
		t.Fatalf("expected direct zapret cidrs to be dropped, got %+v", got.Selectors.CIDRs)
	}
}

func TestParseZapretSelectorsSupportsServicesAndDomains(t *testing.T) {
	t.Parallel()

	selectors, err := ParseZapretSelectors([]string{
		"youtube",
		"YOUTUBE",
		"example.com",
		"EXAMPLE.com",
	}, nil)
	if err != nil {
		t.Fatalf("parse zapret selectors: %v", err)
	}

	if want := []string{"youtube"}; !reflect.DeepEqual(selectors.Services, want) {
		t.Fatalf("unexpected zapret services: %+v", selectors.Services)
	}
	if want := []string{"example.com"}; !reflect.DeepEqual(selectors.Domains, want) {
		t.Fatalf("unexpected zapret domains: %+v", selectors.Domains)
	}
	if len(selectors.CIDRs) != 0 {
		t.Fatalf("expected no zapret cidrs, got %+v", selectors.CIDRs)
	}
}

func TestParseZapretSelectorsRejectsIPv4Selectors(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"1.1.1.1", "1.1.1.0/24", "1.1.1.1-1.1.1.9"} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := ParseZapretSelectors([]string{input}, nil)
			if err == nil {
				t.Fatal("expected zapret parser to reject IPv4 selectors")
			}
			if !strings.Contains(err.Error(), "only service aliases and fully qualified domains are supported") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseZapretSelectorsAllowsServicesWithCIDRsOnly(t *testing.T) {
	t.Parallel()

	selectors, err := ParseZapretSelectors([]string{"direct-ip-only"}, map[string]FirewallTargetDefinition{
		"direct-ip-only": {
			CIDRs: []string{"91.108.0.0/16"},
		},
	})
	if err != nil {
		t.Fatalf("expected CIDR-only zapret service to pass, got %v", err)
	}
	if want := []string{"direct-ip-only"}; !reflect.DeepEqual(selectors.Services, want) {
		t.Fatalf("unexpected zapret services: %+v", selectors.Services)
	}
}

func TestExpandZapretSelectorDomainsUsesServiceCatalog(t *testing.T) {
	t.Parallel()

	got := ExpandZapretSelectorDomains(map[string]FirewallTargetDefinition{
		"openai": {
			Domains: []string{"openai.com", "chatgpt.com"},
		},
	}, FirewallSelectorSet{
		Services: []string{"openai"},
		Domains:  []string{"oaistatic.com"},
		CIDRs:    []string{"1.1.1.1/32"},
	})

	want := []string{"openai.com", "chatgpt.com", "oaistatic.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected expanded zapret domains:\nwant: %+v\n got: %+v", want, got)
	}
}

func TestExpandZapretSelectorCIDRsUsesServiceCatalog(t *testing.T) {
	t.Parallel()

	got := ExpandZapretSelectorCIDRs(map[string]FirewallTargetDefinition{
		"telegram": {
			CIDRs: []string{"91.108.0.0/16", "149.154.0.0/16"},
		},
	}, FirewallSelectorSet{
		Services: []string{"telegram"},
		CIDRs:    []string{"1.1.1.1/32"},
	})

	want := []string{"91.108.0.0/16", "149.154.0.0/16", "1.1.1.1/32"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected expanded zapret cidrs:\nwant: %+v\n got: %+v", want, got)
	}
}

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
	if want := []string{"1.1.1.1/32"}; !reflect.DeepEqual(got.Selectors.CIDRs, want) {
		t.Fatalf("unexpected preserved zapret cidrs: %+v", got.Selectors.CIDRs)
	}
}

func TestCanonicalZapretSettingsWithCatalogExpandsLegacyAliasesToDomains(t *testing.T) {
	t.Parallel()

	got := CanonicalZapretSettingsWithCatalog(ZapretSettings{
		Enabled: true,
		Selectors: FirewallSelectorSet{
			Services: []string{"youtube", "youtube"},
			Domains:  []string{"example.com"},
			CIDRs:    []string{"1.1.1.1/32"},
		},
	}, nil)

	if want := []string{"youtube"}; !reflect.DeepEqual(got.Selectors.Services, want) {
		t.Fatalf("unexpected canonical zapret services:\nwant: %+v\n got: %+v", want, got.Selectors.Services)
	}
	if want := []string{"1.1.1.1/32"}; !reflect.DeepEqual(got.Selectors.CIDRs, want) {
		t.Fatalf("unexpected canonical zapret cidrs:\nwant: %+v\n got: %+v", want, got.Selectors.CIDRs)
	}
	if want := []string{"example.com", "ggpht.com", "googlevideo.com", "youtu.be", "youtube-nocookie.com", "youtube.com", "youtube.googleapis.com", "youtubei.googleapis.com", "ytimg.com"}; !reflect.DeepEqual(got.Selectors.Domains, want) {
		t.Fatalf("unexpected canonical zapret domains:\nwant: %+v\n got: %+v", want, got.Selectors.Domains)
	}
}

func TestParseZapretSelectorsSupportsIPv4Selectors(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"1.1.1.1", "1.1.1.0/24", "1.1.1.1-1.1.1.9"} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			selectors, err := ParseZapretSelectors([]string{input}, nil)
			if err != nil {
				t.Fatalf("parse zapret selectors: %v", err)
			}
			if want := []string{input}; !reflect.DeepEqual(selectors.CIDRs, want) {
				t.Fatalf("unexpected zapret cidrs: %+v", selectors.CIDRs)
			}
		})
	}
}

func TestParseZapretSelectorsSupportsDomainsAndIPv4Selectors(t *testing.T) {
	t.Parallel()

	selectors, err := ParseZapretSelectors([]string{
		"example.com",
		"EXAMPLE.com",
		"api.example.com",
		"1.1.1.1",
		"1.1.1.0/24",
	}, nil)
	if err != nil {
		t.Fatalf("parse zapret selectors: %v", err)
	}
	if len(selectors.Services) != 0 {
		t.Fatalf("expected no zapret services, got %+v", selectors.Services)
	}
	if want := []string{"example.com", "api.example.com"}; !reflect.DeepEqual(selectors.Domains, want) {
		t.Fatalf("unexpected zapret domains: %+v", selectors.Domains)
	}
	if want := []string{"1.1.1.1", "1.1.1.0/24"}; !reflect.DeepEqual(selectors.CIDRs, want) {
		t.Fatalf("unexpected zapret cidrs: %+v", selectors.CIDRs)
	}
}

func TestParseZapretSelectorsAcceptsKnownAliases(t *testing.T) {
	t.Parallel()

	selectors, err := ParseZapretSelectors([]string{"youtube", "YouTube"}, nil)
	if err != nil {
		t.Fatalf("parse zapret selectors: %v", err)
	}
	if want := []string{"youtube"}; !reflect.DeepEqual(selectors.Services, want) {
		t.Fatalf("unexpected zapret services: %+v", selectors.Services)
	}
}

func TestParseZapretSelectorsRejectsUnknownAliases(t *testing.T) {
	t.Parallel()

	_, err := ParseZapretSelectors([]string{"not-a-service"}, nil)
	if err == nil {
		t.Fatal("expected zapret parser to reject unknown aliases")
	}
	if !strings.Contains(err.Error(), "use a fully qualified domain like youtube.com") {
		t.Fatalf("unexpected error: %v", err)
	}
}

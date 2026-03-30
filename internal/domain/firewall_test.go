package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFirewallTargetsSupportsMixedSelectors(t *testing.T) {
	t.Parallel()

	targets, err := ParseFirewallTargets([]string{
		" 1.1.1.1 ",
		"YouTube",
		"YOUTUBE.COM",
		"192.0.2.0/24",
		"video.googlevideo.com",
		"192.0.2.10-192.0.2.20",
		"youtube.com",
	}, nil)
	if err != nil {
		t.Fatalf("parse firewall targets: %v", err)
	}

	if !reflect.DeepEqual(targets.CIDRs, []string{
		"1.1.1.1",
		"192.0.2.0/24",
		"192.0.2.10-192.0.2.20",
	}) {
		t.Fatalf("unexpected target cidrs: %+v", targets.CIDRs)
	}

	if !reflect.DeepEqual(targets.Services, []string{"youtube"}) {
		t.Fatalf("unexpected target services: %+v", targets.Services)
	}

	if !reflect.DeepEqual(targets.Domains, []string{
		"youtube.com",
		"video.googlevideo.com",
	}) {
		t.Fatalf("unexpected target domains: %+v", targets.Domains)
	}
}

func TestParseFirewallTargetsRejectsURLsAndInvalidDomains(t *testing.T) {
	t.Parallel()

	tests := []string{
		"https://youtube.com",
		"youtube.com:443",
		"*.youtube.com",
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := ParseFirewallTargets([]string{input}, nil)
			if err == nil {
				t.Fatal("expected invalid target to fail")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "target") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExpandFirewallTargetDomainsSupportsServiceFamilies(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetDomains(nil, []string{
		"discord",
		"youtube.com",
	}, []string{
		"instagram.com",
		"example.com",
		"youtube.com",
	})

	want := []string{
		"discord.com",
		"discord.gg",
		"discord.gift",
		"discord.media",
		"discordapp.com",
		"discordapp.net",
		"instagram.com",
		"cdninstagram.com",
		"fbcdn.net",
		"facebook.com",
		"facebook.net",
		"fbsbx.com",
		"example.com",
		"youtube.com",
		"youtu.be",
		"youtube-nocookie.com",
		"youtubei.googleapis.com",
		"youtube.googleapis.com",
		"googlevideo.com",
		"ytimg.com",
		"ggpht.com",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded target domains:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestExpandFirewallTargetCIDRsSupportsTelegramPreset(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetCIDRs(nil, []string{"telegram"}, []string{"1.1.1.1/32"})

	want := []string{
		"91.108.0.0/16",
		"149.154.0.0/16",
		"1.1.1.1/32",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded target cidrs:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestExpandFirewallTargetCIDRsSupportsGoogleAIMobilePresets(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetCIDRs(nil, []string{
		"gemini-mobile",
		"notebooklm-mobile",
	}, nil)

	want := []string{
		"74.125.205.0/24",
		"173.194.73.0/24",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded mobile target cidrs:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestExpandFirewallTargetDomainsSupportsGoogleAIPresets(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetDomains(nil, []string{
		"gemini",
		"notebooklm",
	}, []string{
		"gemini.google.com",
		"notebooklm.google.com",
	})

	want := []string{
		"accounts.google.com",
		"content.googleapis.com",
		"gemini.google.com",
		"geminiweb-pa.clients6.google.com",
		"generativelanguage.googleapis.com",
		"lh3.googleusercontent.com",
		"myaccount.google.com",
		"ogads-pa.clients6.google.com",
		"one.google.com",
		"ssl.gstatic.com",
		"support.google.com",
		"waa-pa.clients6.google.com",
		"www.google.com",
		"www.gstatic.com",
		"notebooklm.google.com",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded target domains:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestExpandFirewallTargetDomainsSupportsGoogleAIMobilePresets(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetDomains(nil, []string{
		"gemini-mobile",
		"notebooklm-mobile",
	}, nil)

	want := []string{
		"accounts.google.com",
		"gemini.google.com",
		"myaccount.google.com",
		"one.google.com",
		"support.google.com",
		"www.google.com",
		"mtalk.google.com",
		"dns.google.com",
		"dns.google",
		"googleapis.com",
		"clients6.google.com",
		"gstatic.com",
		"googleusercontent.com",
		"notebooklm.google.com",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded mobile target domains:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestExpandFirewallTargetDomainsSupportsNetflixAndTwitterFamilies(t *testing.T) {
	t.Parallel()

	expanded := ExpandFirewallTargetDomains(nil, []string{
		"twitter",
	}, []string{
		"netflix.com",
		"x.com",
	})

	want := []string{
		"x.com",
		"twitter.com",
		"t.co",
		"twimg.com",
		"netflix.com",
		"netflix.net",
		"nflxvideo.net",
		"nflximg.net",
		"nflximg.com",
		"nflxso.net",
		"nflxext.com",
		"nflxsearch.net",
		"fast.com",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded target domains:\nwant: %+v\n got: %+v", want, expanded)
	}
}

func TestFirewallTargetServiceNamesIsSorted(t *testing.T) {
	t.Parallel()

	want := []string{
		"discord",
		"facetime",
		"gemini",
		"gemini-mobile",
		"instagram",
		"netflix",
		"notebooklm",
		"notebooklm-mobile",
		"telegram",
		"telegram-web",
		"twitter",
		"whatsapp",
		"youtube",
	}

	if got := FirewallTargetServiceNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected service names:\nwant: %+v\n got: %+v", want, got)
	}
}

func TestParseFirewallTargetsSupportsCustomServices(t *testing.T) {
	t.Parallel()

	catalog := map[string]FirewallTargetDefinition{
		"openai": {
			Domains: []string{"openai.com", "chatgpt.com"},
			CIDRs:   []string{"104.18.0.0/15"},
		},
	}

	targets, err := ParseFirewallTargets([]string{
		"OpenAI",
		"oaistatic.com",
		"1.1.1.1",
	}, catalog)
	if err != nil {
		t.Fatalf("parse firewall targets: %v", err)
	}

	if !reflect.DeepEqual(targets.Services, []string{"openai"}) {
		t.Fatalf("unexpected target services: %+v", targets.Services)
	}
	if !reflect.DeepEqual(targets.Domains, []string{"oaistatic.com"}) {
		t.Fatalf("unexpected target domains: %+v", targets.Domains)
	}
	if !reflect.DeepEqual(targets.CIDRs, []string{"1.1.1.1"}) {
		t.Fatalf("unexpected target cidrs: %+v", targets.CIDRs)
	}
}

func TestParseFirewallTargetsSupportsHyphenatedDomains(t *testing.T) {
	t.Parallel()

	targets, err := ParseFirewallTargets([]string{
		"waa-pa.clients6.google.com",
		"geminiweb-pa.clients6.google.com",
	}, nil)
	if err != nil {
		t.Fatalf("parse firewall targets: %v", err)
	}

	if want := []string{
		"waa-pa.clients6.google.com",
		"geminiweb-pa.clients6.google.com",
	}; !reflect.DeepEqual(targets.Domains, want) {
		t.Fatalf("unexpected target domains:\nwant: %+v\n got: %+v", want, targets.Domains)
	}
}

func TestParseFirewallTargetsRejectsUnknownBareAlias(t *testing.T) {
	t.Parallel()

	_, err := ParseFirewallTargets([]string{"openai"}, nil)
	if err == nil {
		t.Fatal("expected unknown service alias to fail")
	}
	if !strings.Contains(err.Error(), "routeflux services set openai") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFirewallTargetDefinitionRejectsReservedNamesAndAliases(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseFirewallTargetDefinition("youtube", []string{"example.com"}); err == nil {
		t.Fatal("expected builtin name to be reserved")
	}

	_, _, err := ParseFirewallTargetDefinition("openai", []string{"youtube"})
	if err == nil {
		t.Fatal("expected alias selector to fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "domain") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFirewallTargetDefinitionSupportsHyphenatedDomains(t *testing.T) {
	t.Parallel()

	name, definition, err := ParseFirewallTargetDefinition("google-ai-mobile", []string{
		"waa-pa.clients6.google.com",
		"geminiweb-pa.clients6.google.com",
		"www.google.com",
	})
	if err != nil {
		t.Fatalf("parse target definition: %v", err)
	}

	if name != "google-ai-mobile" {
		t.Fatalf("unexpected name: %q", name)
	}

	if want := []string{
		"waa-pa.clients6.google.com",
		"geminiweb-pa.clients6.google.com",
		"www.google.com",
	}; !reflect.DeepEqual(definition.Domains, want) {
		t.Fatalf("unexpected target domains:\nwant: %+v\n got: %+v", want, definition.Domains)
	}
}

func TestExpandFirewallTargetsSupportCustomCatalog(t *testing.T) {
	t.Parallel()

	catalog := map[string]FirewallTargetDefinition{
		"openai": {
			Domains: []string{"openai.com", "chatgpt.com"},
			CIDRs:   []string{"104.18.0.0/15"},
		},
	}

	domains := ExpandFirewallTargetDomains(catalog, []string{"openai"}, []string{"oaistatic.com"})
	if want := []string{"openai.com", "chatgpt.com", "oaistatic.com"}; !reflect.DeepEqual(domains, want) {
		t.Fatalf("unexpected expanded domains:\nwant: %+v\n got: %+v", want, domains)
	}

	cidrs := ExpandFirewallTargetCIDRs(catalog, []string{"openai"}, []string{"1.1.1.1/32"})
	if want := []string{"104.18.0.0/15", "1.1.1.1/32"}; !reflect.DeepEqual(cidrs, want) {
		t.Fatalf("unexpected expanded cidrs:\nwant: %+v\n got: %+v", want, cidrs)
	}
}

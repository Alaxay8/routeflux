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
		"YOUTUBE.COM",
		"192.0.2.0/24",
		"video.googlevideo.com",
		"192.0.2.10-192.0.2.20",
		"youtube.com",
	})
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
		"youtube",
		"youtube.com:443",
		"*.youtube.com",
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := ParseFirewallTargets([]string{input})
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

	expanded := ExpandFirewallTargetDomains([]string{
		"youtube.com",
		"instagram.com",
		"example.com",
		"youtube.com",
	})

	want := []string{
		"youtube.com",
		"youtu.be",
		"youtube-nocookie.com",
		"youtubei.googleapis.com",
		"youtube.googleapis.com",
		"googlevideo.com",
		"ytimg.com",
		"ggpht.com",
		"instagram.com",
		"cdninstagram.com",
		"fbcdn.net",
		"example.com",
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Fatalf("unexpected expanded target domains:\nwant: %+v\n got: %+v", want, expanded)
	}
}

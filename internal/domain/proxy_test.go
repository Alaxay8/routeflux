package domain

import "testing"

func TestProxyPortValidation(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		socks, http, dns, transparent int
		fail                          bool
	}{
		{"defaults", 10808, 10809, 1053, 12345, false},
		{"bounds", 1, 65535, 0, 0, false},
		{"zero", 0, 10809, 0, 0, true},
		{"too large", 10808, 65536, 0, 0, true},
		{"same port", 10808, 10808, 0, 0, true},
		{"DNS collision", 1053, 10809, 1053, 0, true},
		{"transparent collision", 10808, 12345, 0, 12345, true},
		{"disabled listeners", 1053, 12345, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := ProxySettings{SOCKSPort: tc.socks, HTTPPort: tc.http}
			if err := p.Validate(tc.dns, tc.transparent); (err != nil) != tc.fail {
				t.Fatalf("validation: %v", err)
			}
		})
	}
}

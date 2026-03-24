package domain

import "testing"

func TestHumanizeProviderNameFromRootDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "connorion.example", want: "Orion VPN"},
		{input: "key.vpnstarlink.example", want: "Starlink VPN"},
		{input: "https://key.vpnstarlink.example/sub/abc", want: "Starlink VPN"},
		{input: "Liberty VPN", want: "Liberty VPN"},
	}

	for _, tt := range tests {
		if got := HumanizeProviderName(tt.input); got != tt.want {
			t.Fatalf("HumanizeProviderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

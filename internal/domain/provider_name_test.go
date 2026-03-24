package domain

import "testing"

func TestHumanizeProviderNameFromRootDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "connsample.example", want: "Sample VPN"},
		{input: "key.vpndemo.example", want: "Demo VPN"},
		{input: "https://key.vpndemo.example/sub/abc", want: "Demo VPN"},
		{input: "Sample VPN", want: "Sample VPN"},
	}

	for _, tt := range tests {
		if got := HumanizeProviderName(tt.input); got != tt.want {
			t.Fatalf("HumanizeProviderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

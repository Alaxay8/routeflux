package cli

import (
	"strings"
	"testing"
)

func TestPrepareAddRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawURL    string
		rawBody   string
		args      []string
		stdin     string
		wantURL   string
		wantRaw   string
		wantError bool
	}{
		{
			name:    "url argument becomes URL request",
			args:    []string{"https://provider.example/sub"},
			wantURL: "https://provider.example/sub",
		},
		{
			name:    "vless argument becomes raw request",
			args:    []string{"vless://uuid@example.com:443?security=tls#Test"},
			wantRaw: "vless://uuid@example.com:443?security=tls#Test",
		},
		{
			name:    "interactive url becomes URL request",
			stdin:   "https://provider.example/sub\n",
			wantURL: "https://provider.example/sub",
		},
		{
			name:    "interactive xray json becomes raw request",
			stdin:   "{\"outbounds\":[{\"protocol\":\"vless\"}]}",
			wantRaw: "{\"outbounds\":[{\"protocol\":\"vless\"}]}",
		},
		{
			name:      "empty input fails",
			wantError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := prepareAddRequest(tt.rawURL, tt.rawBody, "", tt.args, strings.NewReader(tt.stdin))
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("prepare add request: %v", err)
			}
			if req.URL != tt.wantURL {
				t.Fatalf("unexpected URL: %q", req.URL)
			}
			if req.Raw != tt.wantRaw {
				t.Fatalf("unexpected raw payload: %q", req.Raw)
			}
		})
	}
}

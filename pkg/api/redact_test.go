package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactXrayPreviewRedactsSecretsAndKeepsDoH(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"log": {
			"loglevel": "info"
		},
		"dns": {
			"servers": [
				"https://dns.google/dns-query",
				"https://user:secret@dns.example.com/dns-query?token=abc"
			]
		},
		"outbounds": [{
			"tag": "selected",
			"protocol": "vless",
			"settings": {
				"vnext": [{
					"address": "edge.example.com",
					"port": 443,
					"users": [{
						"id": "11111111-1111-1111-1111-111111111111"
					}]
				}],
				"servers": [{
					"password": "trojan-secret"
				}]
			},
			"streamSettings": {
				"realitySettings": {
					"publicKey": "pub",
					"shortId": "ab12",
					"serverName": "cdn.example.com"
				}
			}
		}],
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [{
				"type": "field",
				"outboundTag": "selected",
				"domain": ["domain:youtube.com"],
				"ip": ["1.1.1.1"]
			}]
		}
	}`)

	redacted, err := RedactXrayPreview(raw, &XrayPreviewMetadata{
		Remark:     "Demo Node",
		ServerName: "cdn.example.com",
	})
	if err != nil {
		t.Fatalf("redact xray preview: %v", err)
	}

	text := string(redacted)
	for _, want := range []string{
		`"selected_node": {`,
		`"remark": "Demo Node"`,
		`"server_name": "cdn.example.com"`,
		`"https://dns.google/dns-query"`,
		`"https://dns.example.com/dns-query"`,
		`"protocol": "vless"`,
		`"tag": "selected"`,
		`"serverName": "cdn.example.com"`,
		`"domainStrategy": "AsIs"`,
		`"outboundTag": "selected"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in redacted preview, got %s", want, text)
		}
	}

	for _, forbidden := range []string{
		`11111111-1111-1111-1111-111111111111`,
		`trojan-secret`,
		`"password"`,
		`"publicKey"`,
		`"shortId"`,
		`user:secret@`,
		`token=abc`,
		`"address": "edge.example.com"`,
		`"port": 443`,
		`domain:youtube.com`,
		`"ip": [`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unexpected secret %q in redacted preview: %s", forbidden, text)
		}
	}
}

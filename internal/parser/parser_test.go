package parser_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alaxay8/routeflux/internal/parser"
)

func TestParseVLESSLink(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "vless", "subscription.txt")
	nodes, err := parser.ParseNodes(input, "Example Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "vless", "normalized.golden.json")
}

func TestParseVMessLink(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "vmess", "subscription.txt")
	nodes, err := parser.ParseNodes(input, "Example Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "vmess", "normalized.golden.json")
}

func TestParseMixedBase64Subscription(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "mixed", "subscription.b64")
	nodes, err := parser.ParseNodes(input, "Mixed Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "mixed", "normalized.golden.json")
}

func TestParseXrayJSONConfig(t *testing.T) {
	t.Parallel()

	input := mustReadFixture(t, "three_x_ui", "config.json")
	nodes, err := parser.ParseNodes(input, "3x-ui Import")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}

	assertGoldenNodes(t, nodes, "three_x_ui", "normalized.golden.json")
}

func TestParseJSONArrayOfXrayConfigs(t *testing.T) {
	t.Parallel()

	input := `[
	  {
	    "remarks": "One",
	    "outbounds": [
	      {
	        "protocol": "vless",
	        "tag": "proxy-one",
	        "settings": {
	          "vnext": [
	            {
	              "address": "one.example.com",
	              "port": 443,
	              "users": [
	                {
	                  "id": "11111111-1111-1111-1111-111111111111",
	                  "encryption": "none",
	                  "flow": "xtls-rprx-vision"
	                }
	              ]
	            }
	          ]
	        },
	        "streamSettings": {
	          "network": "tcp",
	          "security": "reality",
	          "realitySettings": {
	            "serverName": "rbc.ru",
	            "publicKey": "public-key-one",
	            "shortId": "short-one",
	            "fingerprint": "random"
	          }
	        }
	      },
	      {
	        "protocol": "freedom",
	        "tag": "direct"
	      }
	    ]
	  },
	  {
	    "remarks": "Two",
	    "outbounds": [
	      {
	        "protocol": "vless",
	        "tag": "proxy-two",
	        "settings": {
	          "vnext": [
	            {
	              "address": "two.example.com",
	              "port": 8443,
	              "users": [
	                {
	                  "id": "22222222-2222-2222-2222-222222222222",
	                  "encryption": "none",
	                  "flow": "xtls-rprx-vision"
	                }
	              ]
	            }
	          ]
	        },
	        "streamSettings": {
	          "network": "tcp",
	          "security": "reality",
	          "realitySettings": {
	            "serverName": "tradingview.com",
	            "publicKey": "public-key-two",
	            "shortId": "short-two",
	            "fingerprint": "random"
	          }
	        }
	      }
	    ]
	  }
	]`

	nodes, err := parser.ParseNodes(input, "JSON Array Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Protocol != "vless" || nodes[1].Protocol != "vless" {
		t.Fatalf("unexpected protocols: %+v", nodes)
	}
}

func TestParseInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := parser.ParseNodes("not-a-subscription", "Broken"); err == nil {
		t.Fatal("expected invalid input to fail")
	}
}

func mustReadFixture(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", "..", "test", "fixtures"}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	return string(data)
}

func assertGoldenNodes(t *testing.T, nodes any, fixtureDir, golden string) {
	t.Helper()

	rawGot, err := marshalCanonicalJSON(nodes)
	if err != nil {
		t.Fatalf("marshal nodes: %v", err)
	}

	got, err := normalizeJSONString(string(rawGot))
	if err != nil {
		t.Fatalf("normalize generated nodes: %v", err)
	}

	want, err := normalizeJSONString(mustReadFixture(t, fixtureDir, golden))
	if err != nil {
		t.Fatalf("normalize golden: %v", err)
	}

	if got != want {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func normalizeJSONString(input string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return "", err
	}

	data, err := marshalCanonicalJSON(value)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(buffer.Bytes()), nil
}

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

func TestParseShadowsocksLink(t *testing.T) {
	t.Parallel()

	input := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAc3MuZXhhbXBsZS5jb206ODM4OA#SS-Edge"
	nodes, err := parser.ParseNodes(input, "Example Provider")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	got := nodes[0]
	if got.Protocol != "shadowsocks" {
		t.Fatalf("expected shadowsocks protocol, got %+v", got)
	}
	if got.Address != "ss.example.com" || got.Port != 8388 {
		t.Fatalf("unexpected endpoint: %+v", got)
	}
	if got.Encryption != "aes-256-gcm" || got.Password != "password" {
		t.Fatalf("unexpected credentials: %+v", got)
	}
	if got.Name != "SS-Edge" || got.Remark != "SS-Edge" {
		t.Fatalf("unexpected label: %+v", got)
	}
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

func TestParseXrayJSONConfigUsesTopLevelRemarksForNodeLabel(t *testing.T) {
	t.Parallel()

	input := `{
	  "remarks": "🇭🇺Венгрия",
	  "outbounds": [
	    {
	      "protocol": "vless",
	      "tag": "proxy",
	      "settings": {
	        "vnext": [
	          {
	            "address": "hungary-edge.example",
	            "port": 8443,
	            "users": [
	              {
	                "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
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
	          "serverName": "gateway.example",
	          "publicKey": "test-public-key",
	          "shortId": "testshort01",
	          "fingerprint": "random"
	        }
	      }
	    },
	    {
	      "protocol": "freedom",
	      "tag": "direct"
	    }
	  ]
	}`

	nodes, err := parser.ParseNodes(input, "JSON Import")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "🇭🇺Венгрия" || nodes[0].Remark != "🇭🇺Венгрия" {
		t.Fatalf("expected top-level remarks to become node label, got %+v", nodes[0])
	}
}

func TestParseXrayJSONConfigSupportsDirectVLESSSettings(t *testing.T) {
	t.Parallel()

	input := `{
	  "outbounds": [
	    {
	      "settings": {
	        "encryption": "none",
	        "flow": "xtls-rprx-vision",
	        "port": 8443,
	        "address": "hungary-edge.example",
	        "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	      },
	      "protocol": "vless",
	      "tag": "proxy",
	      "streamSettings": {
	        "tcpSettings": {
	          "header": {
	            "type": "none"
	          }
	        },
	        "realitySettings": {
	          "shortId": "testshort01",
	          "publicKey": "test-public-key",
	          "spiderX": "",
	          "serverName": "gateway.example",
	          "fingerprint": "random"
	        },
	        "security": "reality",
	        "network": "tcp"
	      }
	    },
	    {
	      "settings": {
	        "response": {
	          "type": "none"
	        }
	      },
	      "protocol": "blackhole",
	      "tag": "block"
	    },
	    {
	      "settings": {},
	      "protocol": "freedom",
	      "tag": "direct"
	    }
	  ],
	  "policy": {
	    "system": {
	      "statsOutboundUplink": true,
	      "statsInboundUplink": true,
	      "statsInboundDownlink": true,
	      "statsOutboundDownlink": true
	    }
	  },
	  "log": {
	    "loglevel": "info"
	  },
	  "id": "BDB6C282-B014-43D7-A291-7E3DAF88B993",
	  "inbounds": [
	    {
	      "settings": {
	        "udp": true
	      },
	      "listen": "[::1]",
	      "port": 1080,
	      "protocol": "socks",
	      "tag": "socks",
	      "sniffing": {
	        "enabled": true,
	        "destOverride": [
	          "tls",
	          "http",
	          "quic"
	        ],
	        "routeOnly": false
	      }
	    }
	  ],
	  "stats": {},
	  "remarks": "🇭🇺Венгрия"
	}`

	nodes, err := parser.ParseNodes(input, "JSON Import")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	got := nodes[0]
	if got.Name != "🇭🇺Венгрия" || got.Remark != "🇭🇺Венгрия" {
		t.Fatalf("expected top-level remarks to become node label, got %+v", got)
	}
	if got.Address != "hungary-edge.example" || got.Port != 8443 {
		t.Fatalf("unexpected endpoint: %+v", got)
	}
	if got.UUID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("unexpected uuid: %+v", got)
	}
	if got.Security != "reality" || got.ServerName != "gateway.example" {
		t.Fatalf("unexpected stream settings: %+v", got)
	}
}

func TestParseXrayJSONDirectProtocolUsesTopLevelRemarksForNameAndRemark(t *testing.T) {
	t.Parallel()

	input := `{
	  "remarks": "🇳🇱 Нидерланды",
	  "protocol": "vless",
	  "tag": "proxy",
	  "settings": {
	    "encryption": "none",
	    "flow": "xtls-rprx-vision",
	    "port": 8443,
	    "address": "snl4.linkey8.ru",
	    "id": "8b922611-af1c-40c9-9af0-80fd0d782084"
	  },
	  "streamSettings": {
	    "network": "tcp",
	    "security": "reality",
	    "realitySettings": {
	      "serverName": "www.vk.com",
	      "publicKey": "wDQjzXYVtjdLkEyXpReh973y4rDIDH6kkX-g-MR7xAg",
	      "shortId": "",
	      "fingerprint": "qq"
	    }
	  }
	}`

	nodes, err := parser.ParseNodes(input, "Starlink")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "🇳🇱 Нидерланды" || nodes[0].Remark != "🇳🇱 Нидерланды" {
		t.Fatalf("expected name and remark to use top-level remarks, got %+v", nodes[0])
	}
}

func TestParseXrayJSONWrapperPropagatesTopLevelRemarksIntoNestedConfig(t *testing.T) {
	t.Parallel()

	input := `{
	  "remarks": "🇳🇱 Нидерланды",
	  "config": {
	    "outbounds": [
	      {
	        "protocol": "vless",
	        "tag": "proxy",
	        "settings": {
	          "encryption": "none",
	          "flow": "xtls-rprx-vision",
	          "port": 8443,
	          "address": "snl4.linkey8.ru",
	          "id": "8b922611-af1c-40c9-9af0-80fd0d782084"
	        },
	        "streamSettings": {
	          "network": "tcp",
	          "security": "reality",
	          "realitySettings": {
	            "serverName": "www.vk.com",
	            "publicKey": "wDQjzXYVtjdLkEyXpReh973y4rDIDH6kkX-g-MR7xAg",
	            "shortId": "",
	            "fingerprint": "qq"
	          }
	        }
	      },
	      {
	        "protocol": "freedom",
	        "tag": "direct"
	      }
	    ]
	  }
	}`

	nodes, err := parser.ParseNodes(input, "Starlink")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "🇳🇱 Нидерланды" || nodes[0].Remark != "🇳🇱 Нидерланды" {
		t.Fatalf("expected wrapper remarks to populate name and remark, got %+v", nodes[0])
	}
}

func TestParseJSONWrapperLinkPrefersWrapperRemarksOverNestedLinkName(t *testing.T) {
	t.Parallel()

	input := `{
	  "remarks": "Netherlands",
	  "link": "vless://11111111-1111-1111-1111-111111111111@nl.example.com:443?encryption=none&security=tls&sni=nl.example.com&type=ws&path=%2Fa&host=cdn.example.com#Netherlands-bypass"
	}`

	nodes, err := parser.ParseNodes(input, "Liberty VPN")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "Netherlands" || nodes[0].Remark != "Netherlands" {
		t.Fatalf("expected wrapper remarks to override nested link label, got %+v", nodes[0])
	}
}

func TestParseJSONWrapperStringConfigPrefersWrapperRemarksOverNestedLinkName(t *testing.T) {
	t.Parallel()

	input := `{
	  "remarks": "Netherlands",
	  "config": "vless://11111111-1111-1111-1111-111111111111@nl.example.com:443?encryption=none&security=tls&sni=nl.example.com&type=ws&path=%2Fa&host=cdn.example.com#Netherlands-bypass"
	}`

	nodes, err := parser.ParseNodes(input, "Liberty VPN")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "Netherlands" || nodes[0].Remark != "Netherlands" {
		t.Fatalf("expected wrapper remarks to override nested config label, got %+v", nodes[0])
	}
}

func TestParseJSONArrayWrapperRemarksProduceDistinctNodeIDs(t *testing.T) {
	t.Parallel()

	input := `[
	  {
	    "remarks": "Netherlands",
	    "link": "vless://11111111-1111-1111-1111-111111111111@nl.example.com:443?encryption=none&security=tls&sni=nl.example.com&type=ws&path=%2Fa&host=cdn.example.com#Shared-bypass"
	  },
	  {
	    "remarks": "Netherlands-bypass",
	    "link": "vless://11111111-1111-1111-1111-111111111111@nl.example.com:443?encryption=none&security=tls&sni=nl.example.com&type=ws&path=%2Fa&host=cdn.example.com#Shared-bypass"
	  }
	]`

	nodes, err := parser.ParseNodes(input, "Liberty VPN")
	if err != nil {
		t.Fatalf("parse nodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "Netherlands" || nodes[1].Name != "Netherlands-bypass" {
		t.Fatalf("expected wrapper labels to win over nested link labels, got %+v", nodes)
	}
	if nodes[0].ID == nodes[1].ID {
		t.Fatalf("expected distinct node ids after wrapper label override, got %+v", nodes)
	}
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
	            "serverName": "gateway-one.example",
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
	            "serverName": "gateway-two.example",
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
	if nodes[0].Name != "One" || nodes[0].Remark != "One" {
		t.Fatalf("expected first node label from top-level remarks, got %+v", nodes[0])
	}
	if nodes[1].Name != "Two" || nodes[1].Remark != "Two" {
		t.Fatalf("expected second node label from top-level remarks, got %+v", nodes[1])
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

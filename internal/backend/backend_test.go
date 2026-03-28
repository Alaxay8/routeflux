package backend_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/backend/xray"
	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestGenerateManualVLESSConfig(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:          "node-1",
				Name:        "Edge Reality",
				Protocol:    domain.ProtocolVLESS,
				Address:     "node1.example.com",
				Port:        443,
				UUID:        "11111111-1111-1111-1111-111111111111",
				Encryption:  "none",
				Security:    "reality",
				ServerName:  "edge.example.com",
				Fingerprint: "chrome",
				PublicKey:   "public-key-1",
				ShortID:     "ab12cd34",
				Transport:   "ws",
				Path:        "/proxy",
				Host:        "cdn.example.com",
			},
		},
		SelectedNodeID: "node-1",
		LogLevel:       "warning",
		SOCKSPort:      10808,
		HTTPPort:       10809,
	}

	got, err := xray.NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	gotJSON, err := normalizeJSON(got)
	if err != nil {
		t.Fatalf("normalize generated json: %v", err)
	}

	want, err := normalizeJSON([]byte(mustReadGolden(t, "manual_vless.golden.json")))
	if err != nil {
		t.Fatalf("normalize golden json: %v", err)
	}

	if string(gotJSON) != string(want) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), string(gotJSON))
	}
}

func TestGenerateSplitDoHConfig(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:         "node-1",
				Name:       "Edge Reality",
				Protocol:   domain.ProtocolVLESS,
				Address:    "node1.example.com",
				Port:       443,
				UUID:       "11111111-1111-1111-1111-111111111111",
				Encryption: "none",
				Security:   "reality",
				PublicKey:  "public-key-1",
				ShortID:    "ab12cd34",
				Transport:  "tcp",
			},
		},
		SelectedNodeID: "node-1",
		LogLevel:       "warning",
		SOCKSPort:      10808,
		HTTPPort:       10809,
		DNS: domain.DNSSettings{
			Mode:          domain.DNSModeSplit,
			Transport:     domain.DNSTransportDoH,
			Servers:       []string{"dns.google", "1.1.1.1"},
			Bootstrap:     []string{"9.9.9.9"},
			DirectDomains: []string{"domain:lan", "full:router.lan"},
		},
	}

	got, err := xray.NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	dns, ok := cfg["dns"].(map[string]any)
	if !ok {
		t.Fatalf("dns section missing: %+v", cfg)
	}

	servers, ok := dns["servers"].([]any)
	if !ok {
		t.Fatalf("dns servers missing: %+v", dns)
	}
	if len(servers) != 4 {
		t.Fatalf("expected four dns servers, got %d", len(servers))
	}

	local, ok := servers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first dns server object, got %T", servers[0])
	}
	if local["address"] != "localhost" {
		t.Fatalf("unexpected local dns address: %+v", local)
	}
	if !reflect.DeepEqual(asStringSlice(t, local["domains"]), []string{"domain:lan", "full:router.lan"}) {
		t.Fatalf("unexpected local domains: %+v", local["domains"])
	}

	bootstrap, ok := servers[1].(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap dns server object, got %T", servers[1])
	}
	if bootstrap["address"] != "9.9.9.9" {
		t.Fatalf("unexpected bootstrap address: %+v", bootstrap)
	}
	if !reflect.DeepEqual(asStringSlice(t, bootstrap["domains"]), []string{"full:dns.google"}) {
		t.Fatalf("unexpected bootstrap domains: %+v", bootstrap["domains"])
	}

	if servers[2] != "https://dns.google/dns-query" {
		t.Fatalf("unexpected primary doh server: %+v", servers[2])
	}
	if servers[3] != "https://1.1.1.1/dns-query" {
		t.Fatalf("unexpected secondary doh server: %+v", servers[3])
	}
}

func TestGenerateDoTConfigFails(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:       "node-1",
				Protocol: domain.ProtocolVLESS,
				Address:  "node1.example.com",
				Port:     443,
				UUID:     "11111111-1111-1111-1111-111111111111",
			},
		},
		SelectedNodeID: "node-1",
		DNS: domain.DNSSettings{
			Mode:      domain.DNSModeRemote,
			Transport: domain.DNSTransportDoT,
			Servers:   []string{"dns.google"},
		},
	}

	_, err := xray.NewGenerator().Generate(req)
	if err == nil {
		t.Fatal("expected dot transport to fail")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateTransparentVLESSConfig(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:          "node-1",
				Name:        "Edge Reality",
				Protocol:    domain.ProtocolVLESS,
				Address:     "node1.example.com",
				Port:        443,
				UUID:        "11111111-1111-1111-1111-111111111111",
				Encryption:  "none",
				Security:    "reality",
				ServerName:  "edge.example.com",
				Fingerprint: "chrome",
				PublicKey:   "public-key-1",
				ShortID:     "ab12cd34",
				Transport:   "ws",
				Path:        "/proxy",
				Host:        "cdn.example.com",
			},
		},
		SelectedNodeID:   "node-1",
		LogLevel:         "warning",
		SOCKSPort:        10808,
		HTTPPort:         10809,
		TransparentProxy: true,
		TransparentPort:  12345,
	}

	got, err := xray.NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	gotJSON, err := normalizeJSON(got)
	if err != nil {
		t.Fatalf("normalize generated json: %v", err)
	}

	want, err := normalizeJSON([]byte(mustReadGolden(t, "transparent_vless.golden.json")))
	if err != nil {
		t.Fatalf("normalize golden json: %v", err)
	}

	if string(gotJSON) != string(want) {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), string(gotJSON))
	}
}

func TestGenerateTransparentConfigRoutesDNSUpstreamsDirect(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:          "node-1",
				Name:        "Edge Reality",
				Protocol:    domain.ProtocolVLESS,
				Address:     "node1.example.com",
				Port:        443,
				UUID:        "11111111-1111-1111-1111-111111111111",
				Encryption:  "none",
				Security:    "reality",
				ServerName:  "edge.example.com",
				Fingerprint: "chrome",
				PublicKey:   "public-key-1",
				ShortID:     "ab12cd34",
				Transport:   "tcp",
			},
		},
		SelectedNodeID:   "node-1",
		LogLevel:         "warning",
		SOCKSPort:        10808,
		HTTPPort:         10809,
		TransparentProxy: true,
		TransparentPort:  12345,
		DNS: domain.DNSSettings{
			Mode:          domain.DNSModeSplit,
			Transport:     domain.DNSTransportDoH,
			Servers:       []string{"1.1.1.1", "dns.google"},
			Bootstrap:     []string{"9.9.9.9"},
			DirectDomains: []string{"domain:lan"},
		},
	}

	got, err := xray.NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	routing, ok := cfg["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing section missing: %+v", cfg)
	}

	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("routing rules missing: %+v", routing)
	}
	if len(rules) != 3 {
		t.Fatalf("expected dns direct, transparent selected, and local inbound rules, got %d rules", len(rules))
	}

	direct, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first routing rule object, got %T", rules[0])
	}
	if direct["outboundTag"] != "direct" {
		t.Fatalf("expected first routing rule to bypass dns upstreams directly, got %+v", direct)
	}

	gotIPs := asStringSlice(t, direct["ip"])
	if !reflect.DeepEqual(gotIPs, []string{"1.1.1.1", "9.9.9.9"}) {
		t.Fatalf("unexpected direct-route dns IPs: %v", gotIPs)
	}

	gotDomains := asStringSlice(t, direct["domain"])
	if !reflect.DeepEqual(gotDomains, []string{"full:dns.google"}) {
		t.Fatalf("unexpected direct-route dns domains: %v", gotDomains)
	}

	transparent, ok := rules[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second routing rule object, got %T", rules[1])
	}
	if transparent["outboundTag"] != "selected" || transparent["network"] != "tcp" {
		t.Fatalf("unexpected transparent route: %+v", transparent)
	}
	if !reflect.DeepEqual(asStringSlice(t, transparent["inboundTag"]), []string{"transparent-in"}) {
		t.Fatalf("unexpected transparent inbound tags: %+v", transparent["inboundTag"])
	}

	local, ok := rules[2].(map[string]any)
	if !ok {
		t.Fatalf("expected third routing rule object, got %T", rules[2])
	}
	if local["outboundTag"] != "selected" || local["network"] != "tcp,udp" {
		t.Fatalf("unexpected local inbound route: %+v", local)
	}
	if !reflect.DeepEqual(asStringSlice(t, local["inboundTag"]), []string{"socks-in", "http-in"}) {
		t.Fatalf("unexpected local inbound tags: %+v", local["inboundTag"])
	}
}

func TestGenerateTransparentTargetRoutingOnlySelectsMatchedServices(t *testing.T) {
	t.Parallel()

	req := backend.ConfigRequest{
		Mode: domain.SelectionModeManual,
		Nodes: []domain.Node{
			{
				ID:          "node-1",
				Name:        "Edge Reality",
				Protocol:    domain.ProtocolVLESS,
				Address:     "node1.example.com",
				Port:        443,
				UUID:        "11111111-1111-1111-1111-111111111111",
				Encryption:  "none",
				Security:    "reality",
				ServerName:  "edge.example.com",
				Fingerprint: "chrome",
				PublicKey:   "public-key-1",
				ShortID:     "ab12cd34",
				Transport:   "tcp",
			},
		},
		SelectedNodeID:           "node-1",
		LogLevel:                 "warning",
		SOCKSPort:                10808,
		HTTPPort:                 10809,
		TransparentProxy:         true,
		TransparentPort:          12345,
		TransparentTargetDomains: []string{"youtube.com", "googlevideo.com"},
		TransparentTargetCIDRs:   []string{"1.1.1.1/32"},
	}

	got, err := xray.NewGenerator().Generate(req)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil {
		t.Fatalf("unmarshal generated json: %v", err)
	}

	routing, ok := cfg["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing section missing: %+v", cfg)
	}

	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("routing rules missing: %+v", routing)
	}
	if len(rules) != 4 {
		t.Fatalf("expected four routing rules, got %d", len(rules))
	}

	domainRule, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first routing rule object, got %T", rules[0])
	}
	if domainRule["outboundTag"] != "selected" {
		t.Fatalf("unexpected target domain rule: %+v", domainRule)
	}
	if !reflect.DeepEqual(asStringSlice(t, domainRule["inboundTag"]), []string{"transparent-in"}) {
		t.Fatalf("unexpected target domain inbound tag: %+v", domainRule["inboundTag"])
	}
	if !reflect.DeepEqual(asStringSlice(t, domainRule["domain"]), []string{"domain:youtube.com", "domain:googlevideo.com"}) {
		t.Fatalf("unexpected target domains: %+v", domainRule["domain"])
	}

	ipRule, ok := rules[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second routing rule object, got %T", rules[1])
	}
	if ipRule["outboundTag"] != "selected" {
		t.Fatalf("unexpected target ip rule: %+v", ipRule)
	}
	if !reflect.DeepEqual(asStringSlice(t, ipRule["inboundTag"]), []string{"transparent-in"}) {
		t.Fatalf("unexpected target ip inbound tag: %+v", ipRule["inboundTag"])
	}
	if !reflect.DeepEqual(asStringSlice(t, ipRule["ip"]), []string{"1.1.1.1/32"}) {
		t.Fatalf("unexpected target ips: %+v", ipRule["ip"])
	}

	fallbackRule, ok := rules[2].(map[string]any)
	if !ok {
		t.Fatalf("expected third routing rule object, got %T", rules[2])
	}
	if fallbackRule["outboundTag"] != "direct" {
		t.Fatalf("unexpected transparent fallback rule: %+v", fallbackRule)
	}
	if !reflect.DeepEqual(asStringSlice(t, fallbackRule["inboundTag"]), []string{"transparent-in"}) {
		t.Fatalf("unexpected transparent fallback inbound tag: %+v", fallbackRule["inboundTag"])
	}

	localRule, ok := rules[3].(map[string]any)
	if !ok {
		t.Fatalf("expected fourth routing rule object, got %T", rules[3])
	}
	if localRule["outboundTag"] != "selected" {
		t.Fatalf("unexpected local inbound rule: %+v", localRule)
	}
	if !reflect.DeepEqual(asStringSlice(t, localRule["inboundTag"]), []string{"socks-in", "http-in"}) {
		t.Fatalf("unexpected local inbound tags: %+v", localRule["inboundTag"])
	}
}

func mustReadGolden(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "test", "fixtures", "xray", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}

	return string(data)
}

func normalizeJSON(input []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSpace(buffer.Bytes()), nil
}

func asStringSlice(t *testing.T, value any) []string {
	t.Helper()

	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", value)
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		str, ok := item.(string)
		if !ok {
			t.Fatalf("expected string item, got %T", item)
		}
		out = append(out, str)
	}
	return out
}

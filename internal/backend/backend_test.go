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

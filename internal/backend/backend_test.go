package backend_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

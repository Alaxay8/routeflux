package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/speedtest"
)

func TestInspectXrayJSONOutputsRawConfig(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Node 1",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{
		Store:   store,
		Backend: &cliInspectBackend{config: []byte(`{"log":{"loglevel":"warning"}}`)},
	})

	cmd := newInspectCmd(&rootOptions{service: service, jsonOutput: true})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"xray", "--subscription", "sub-1", "--node", "node-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect xray: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if got != `{"log":{"loglevel":"warning"}}` {
		t.Fatalf("unexpected inspect xray output: %s", got)
	}
}

func TestInspectXraySafeOutputsRedactedConfig(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Node 1",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{
		Store: store,
		Backend: &cliInspectBackend{config: []byte(`{
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
		}`)},
	})

	cmd := newInspectCmd(&rootOptions{service: service})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"xray-safe", "--subscription", "sub-1", "--node", "node-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect xray-safe: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		`"selected_node": {`,
		`"remark": "Node 1"`,
		`"server_name": "cdn.example.com"`,
		`"https://dns.google/dns-query"`,
		`"https://dns.example.com/dns-query"`,
		`"serverName": "cdn.example.com"`,
		`"domainStrategy": "AsIs"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in redacted preview, got %s", want, got)
		}
	}

	for _, forbidden := range []string{
		`11111111-1111-1111-1111-111111111111`,
		`"password"`,
		`"publicKey"`,
		`"shortId"`,
		`user:secret@`,
		`token=abc`,
		`"address": "edge.example.com"`,
		`domain:youtube.com`,
		`"ip": [`,
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("unexpected secret %q in redacted preview: %s", forbidden, got)
		}
	}
}

func TestInspectSpeedJSONOutputsMetrics(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Node 1",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{
		Store:       store,
		Backend:     &cliInspectBackend{config: []byte(`{"ok":true}`)},
		SpeedTester: &cliSpeedTester{},
	})

	cmd := newInspectCmd(&rootOptions{service: service, jsonOutput: true})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"speed", "--subscription", "sub-1", "--node", "node-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect speed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"latency_ms": 41.2`) {
		t.Fatalf("expected latency in output, got %s", output)
	}
	if !strings.Contains(output, `"download_mbps": 88.1`) {
		t.Fatalf("expected download metric in output, got %s", output)
	}
}

func TestInspectSpeedDoesNotPrintUsageOnError(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{
						ID:             "node-1",
						SubscriptionID: "sub-1",
						Name:           "Node 1",
						Protocol:       domain.ProtocolVLESS,
						Address:        "edge.example.com",
						Port:           443,
						UUID:           "11111111-1111-1111-1111-111111111111",
					},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{
		Store:       store,
		Backend:     &cliInspectBackend{config: []byte(`{"ok":true}`)},
		SpeedTester: cliSpeedTester{err: speedtest.ErrBusy},
	})

	cmd := newInspectCmd(&rootOptions{service: service, jsonOutput: true})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"speed", "--subscription", "sub-1", "--node", "node-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected inspect speed error")
	}
	if !strings.Contains(err.Error(), speedtest.ErrBusy.Error()) {
		t.Fatalf("expected busy error, got %v", err)
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected no cobra usage output, got %s", stderr.String())
	}
}

func TestInspectPingJSONOutputsNodeResultsEvenWhenOneNodeFails(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	openPort := listener.Addr().(*net.TCPAddr).Port
	closedListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed port: %v", err)
	}
	closedPort := closedListener.Addr().(*net.TCPAddr).Port
	_ = closedListener.Close()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{ID: "node-1", SubscriptionID: "sub-1", Name: "Node 1", Address: "127.0.0.1", Port: openPort},
					{ID: "node-2", SubscriptionID: "sub-1", Name: "Node 2", Address: "127.0.0.1", Port: closedPort},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newInspectCmd(&rootOptions{service: service, jsonOutput: true})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"ping", "--subscription", "sub-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect ping: %v", err)
	}

	var payload app.PingInspectResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal inspect ping output: %v\n%s", err, stdout.String())
	}

	if payload.SubscriptionID != "sub-1" || payload.TimeoutMS != 2000 {
		t.Fatalf("unexpected ping payload: %+v", payload)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("expected two results, got %+v", payload)
	}
	if payload.Results[0].NodeID != "node-1" || !payload.Results[0].Healthy {
		t.Fatalf("expected first node to succeed, got %+v", payload.Results[0])
	}
	if payload.Results[1].NodeID != "node-2" || payload.Results[1].Healthy {
		t.Fatalf("expected second node to fail, got %+v", payload.Results[1])
	}
	if payload.Results[1].Error == "" {
		t.Fatalf("expected failed node error, got %+v", payload.Results[1])
	}
}

func TestInspectPingDoesNotPrintUsageOnLookupError(t *testing.T) {
	t.Parallel()

	store := &cliMemoryStore{
		subs: []domain.Subscription{
			{
				ID:          "sub-1",
				DisplayName: "Demo VPN",
				Nodes: []domain.Node{
					{ID: "node-1", SubscriptionID: "sub-1", Name: "Node 1", Address: "127.0.0.1", Port: 443},
				},
			},
		},
		settings: domain.DefaultSettings(),
		state:    domain.DefaultRuntimeState(),
	}
	service := app.NewService(app.Dependencies{Store: store})

	cmd := newInspectCmd(&rootOptions{service: service, jsonOutput: true})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"ping", "--subscription", "sub-1", "--node", "missing-node"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected inspect ping error")
	}
	if !strings.Contains(err.Error(), "missing-node") {
		t.Fatalf("unexpected inspect ping error: %v", err)
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected no cobra usage output, got %s", stderr.String())
	}
}

type cliInspectBackend struct {
	config []byte
}

func (b *cliInspectBackend) GenerateConfig(backend.ConfigRequest) ([]byte, error) {
	return append([]byte(nil), b.config...), nil
}

func (b *cliInspectBackend) ApplyConfig(context.Context, backend.ConfigRequest) error { return nil }
func (b *cliInspectBackend) CaptureRollback() (backend.RollbackSnapshot, error) {
	return backend.RollbackSnapshot{}, nil
}
func (b *cliInspectBackend) RollbackConfig(context.Context, backend.RollbackSnapshot) error {
	return nil
}
func (b *cliInspectBackend) Start(context.Context) error  { return nil }
func (b *cliInspectBackend) Stop(context.Context) error   { return nil }
func (b *cliInspectBackend) Reload(context.Context) error { return nil }
func (b *cliInspectBackend) Status(context.Context) (backend.RuntimeStatus, error) {
	return backend.RuntimeStatus{}, nil
}

type cliSpeedTester struct {
	err error
}

func (t cliSpeedTester) Test(context.Context, speedtest.Request) (speedtest.Result, error) {
	return t.result()
}

func (t cliSpeedTester) result() (speedtest.Result, error) {
	if t.err != nil {
		return speedtest.Result{}, t.err
	}
	return speedtest.Result{
		SubscriptionID: "sub-1",
		NodeID:         "node-1",
		NodeName:       "Node 1",
		LatencyMS:      41.2,
		DownloadMbps:   88.1,
		UploadMbps:     22.4,
		DownloadBytes:  1234,
		UploadBytes:    5678,
		StartedAt:      time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC),
		FinishedAt:     time.Date(2026, 3, 26, 20, 0, 3, 0, time.UTC),
	}, nil
}

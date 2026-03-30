package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/pkg/api"
)

func TestStatusCommandJSONRedactsSecrets(t *testing.T) {
	t.Parallel()

	cmd := newStatusCmd(&rootOptions{
		service:    newSensitiveCLIService(),
		jsonOutput: true,
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute status json: %v", err)
	}

	assertJSONSecretsRedacted(t, stdout.String())

	var payload api.StatusResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status json: %v\n%s", err, stdout.String())
	}

	if payload.ActiveSubscription == nil {
		t.Fatal("expected active subscription in status response")
	}
	if payload.ActiveSubscription.DisplayName != "Demo VPN" {
		t.Fatalf("unexpected active subscription: %+v", payload.ActiveSubscription)
	}
	if payload.ActiveNode == nil {
		t.Fatal("expected active node in status response")
	}
	if payload.ActiveNode.Address != "203.0.113.10" {
		t.Fatalf("unexpected active node: %+v", payload.ActiveNode)
	}
}

func TestListSubscriptionsJSONRedactsSecrets(t *testing.T) {
	t.Parallel()

	cmd := newListCmd(&rootOptions{
		service:    newSensitiveCLIService(),
		jsonOutput: true,
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"subscriptions"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute list subscriptions json: %v", err)
	}

	assertJSONSecretsRedacted(t, stdout.String())

	var payload []api.SubscriptionSummary
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal subscriptions json: %v\n%s", err, stdout.String())
	}

	if len(payload) != 1 {
		t.Fatalf("expected one subscription, got %d", len(payload))
	}
	if payload[0].SourceType != string(domain.SourceTypeURL) {
		t.Fatalf("unexpected source type: %+v", payload[0])
	}
	if len(payload[0].Nodes) != 1 {
		t.Fatalf("expected one safe node summary, got %+v", payload[0].Nodes)
	}
	if payload[0].Nodes[0].Security != "reality" {
		t.Fatalf("unexpected node summary: %+v", payload[0].Nodes[0])
	}
}

func TestListNodesJSONRedactsSecrets(t *testing.T) {
	t.Parallel()

	cmd := newListCmd(&rootOptions{
		service:    newSensitiveCLIService(),
		jsonOutput: true,
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"nodes", "--subscription", "sub-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute list nodes json: %v", err)
	}

	assertJSONSecretsRedacted(t, stdout.String())

	var payload []api.NodeSummary
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal nodes json: %v\n%s", err, stdout.String())
	}

	if len(payload) != 1 {
		t.Fatalf("expected one node summary, got %d", len(payload))
	}
	if payload[0].Protocol != string(domain.ProtocolVLESS) {
		t.Fatalf("unexpected node protocol: %+v", payload[0])
	}
	if payload[0].Address != "203.0.113.10" {
		t.Fatalf("unexpected node address: %+v", payload[0])
	}
}

func TestDiagnosticsCommandJSONRedactsNestedStatusSecrets(t *testing.T) {
	t.Parallel()

	cmd := newDiagnosticsCmd(&rootOptions{
		rootDir:    t.TempDir(),
		service:    newSensitiveCLIService(),
		jsonOutput: true,
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(new(bytes.Buffer))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute diagnostics json: %v", err)
	}

	assertJSONSecretsRedacted(t, stdout.String())

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal diagnostics json: %v\n%s", err, stdout.String())
	}

	statusValue, ok := payload["status"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested status object, got %#v", payload["status"])
	}
	activeSubscription, ok := statusValue["active_subscription"].(map[string]any)
	if !ok {
		t.Fatalf("expected active_subscription object, got %#v", statusValue["active_subscription"])
	}
	if _, exists := activeSubscription["source"]; exists {
		t.Fatalf("diagnostics leaked subscription source: %v", activeSubscription)
	}
}

func newSensitiveCLIService() *app.Service {
	now := time.Date(2026, 3, 29, 10, 30, 0, 0, time.UTC)

	return app.NewService(app.Dependencies{
		Store: &cliMemoryStore{
			subs: []domain.Subscription{
				{
					ID:                 "sub-1",
					SourceType:         domain.SourceTypeURL,
					Source:             "https://provider.example/subscription/secret-token",
					ProviderName:       "Demo VPN",
					DisplayName:        "Demo VPN",
					ProviderNameSource: domain.ProviderNameSourceManual,
					LastUpdatedAt:      now,
					RefreshInterval:    domain.NewDuration(time.Hour),
					LastError:          "last refresh failed",
					ParserStatus:       "ok",
					Nodes: []domain.Node{
						{
							ID:             "node-1",
							SubscriptionID: "sub-1",
							Name:           "Netherlands",
							Remark:         "Netherlands",
							Protocol:       domain.ProtocolVLESS,
							Address:        "203.0.113.10",
							Port:           443,
							UUID:           "11111111-1111-1111-1111-111111111111",
							Password:       "super-secret-password",
							Security:       "reality",
							PublicKey:      "public-key-secret",
							ShortID:        "abcd1234",
							Transport:      "ws",
							RawQuery:       "security=reality&pbk=public-key-secret&sid=abcd1234",
						},
					},
				},
			},
			settings: domain.DefaultSettings(),
			state: domain.RuntimeState{
				SchemaVersion:        1,
				ActiveSubscriptionID: "sub-1",
				ActiveNodeID:         "node-1",
				Mode:                 domain.SelectionModeManual,
				Connected:            true,
				LastRefreshAt:        map[string]time.Time{"sub-1": now},
				Health:               map[string]domain.NodeHealth{},
				LastSuccessAt:        now,
			},
		},
	})
}

func assertJSONSecretsRedacted(t *testing.T, output string) {
	t.Helper()

	for _, forbiddenKey := range []string{
		`"source"`,
		`"uuid"`,
		`"password"`,
		`"raw_query"`,
		`"public_key"`,
		`"short_id"`,
	} {
		if strings.Contains(output, forbiddenKey) {
			t.Fatalf("expected output to redact %s\n%s", forbiddenKey, output)
		}
	}

	for _, forbiddenValue := range []string{
		"secret-token",
		"11111111-1111-1111-1111-111111111111",
		"super-secret-password",
		"public-key-secret",
		"abcd1234",
	} {
		if strings.Contains(output, forbiddenValue) {
			t.Fatalf("expected output to hide %q\n%s", forbiddenValue, output)
		}
	}
}

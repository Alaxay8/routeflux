package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func TestDurationJSONRoundTrip(t *testing.T) {
	t.Parallel()

	type payload struct {
		Interval domain.Duration `json:"interval"`
	}

	in := payload{Interval: domain.NewDuration(90 * time.Minute)}

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out payload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.Interval.Duration() != 90*time.Minute {
		t.Fatalf("unexpected duration: %s", out.Interval.Duration())
	}
}

func TestSubscriptionNodeLookup(t *testing.T) {
	t.Parallel()

	sub := domain.Subscription{
		ID: "sub-1",
		Nodes: []domain.Node{
			{ID: "node-a", Name: "A"},
			{ID: "node-b", Name: "B"},
		},
	}

	node, ok := sub.NodeByID("node-b")
	if !ok {
		t.Fatal("expected node lookup to succeed")
	}

	if node.Name != "B" {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestDefaultSettingsAreSane(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings()

	if settings.RefreshInterval.Duration() <= 0 {
		t.Fatal("refresh interval must be positive")
	}

	if settings.SwitchCooldown.Duration() <= 0 {
		t.Fatal("switch cooldown must be positive")
	}

	if settings.Mode != domain.SelectionModeManual {
		t.Fatalf("unexpected default mode: %s", settings.Mode)
	}
}

func TestEffectiveTransparentBlockQUICHonorsExplicitSetting(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings().Firewall
	settings.BlockQUIC = true

	if !domain.EffectiveTransparentBlockQUIC(settings, nil) {
		t.Fatal("expected explicit block-quic setting to win")
	}
}

func TestEffectiveTransparentBlockQUICAutoBlocksIncompatibleNode(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings().Firewall
	node := domain.Node{
		Protocol:  domain.ProtocolVLESS,
		Transport: "tcp",
		Security:  "reality",
		Flow:      "xtls-rprx-vision",
	}

	if !domain.EffectiveTransparentBlockQUIC(settings, &node) {
		t.Fatal("expected incompatible node to force block-quic")
	}
}

func TestEffectiveTransparentBlockQUICKeepsCompatibleNodeProxied(t *testing.T) {
	t.Parallel()

	settings := domain.DefaultSettings().Firewall
	node := domain.Node{
		Protocol:  domain.ProtocolVLESS,
		Transport: "ws",
		Security:  "tls",
	}

	if domain.EffectiveTransparentBlockQUIC(settings, &node) {
		t.Fatal("expected compatible node to keep proxied quic")
	}
}

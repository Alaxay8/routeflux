package xray

import (
	"encoding/json"
	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"testing"
)

func TestProxyListenAndPorts(t *testing.T) {
	for _, allow := range []bool{false, true} {
		req := backend.ConfigRequest{Nodes: []domain.Node{{ID: "n", Protocol: domain.ProtocolSocks, Address: "127.0.0.1", Port: 9999}}, SelectedNodeID: "n", AllowLAN: allow, SOCKSPort: 20808, HTTPPort: 20809}
		data, err := NewGenerator().Generate(req)
		if err != nil {
			t.Fatal(err)
		}
		var cfg xrayConfig
		if err = json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		want := "127.0.0.1"
		if allow {
			want = "0.0.0.0"
		}
		if len(cfg.Inbounds) != 2 {
			t.Fatalf("unexpected inbounds: %+v", cfg.Inbounds)
		}
		for i, in := range cfg.Inbounds {
			if in.Listen != want || in.Port != 20808+i {
				t.Fatalf("inbound: %+v", in)
			}
		}
		rule := cfg.Routing.Rules[len(cfg.Routing.Rules)-1]
		if rule.OutboundTag != "selected" {
			t.Fatalf("route: %+v", rule)
		}
	}
}

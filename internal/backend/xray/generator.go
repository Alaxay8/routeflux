package xray

import (
	"encoding/json"
	"fmt"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

// Generator builds Xray configuration files from RouteFlux nodes.
type Generator struct{}

// NewGenerator creates a config generator instance.
func NewGenerator() Generator {
	return Generator{}
}

// Generate renders an Xray JSON config.
func (Generator) Generate(req backend.ConfigRequest) ([]byte, error) {
	selected, err := selectedNode(req.Nodes, req.SelectedNodeID)
	if err != nil {
		return nil, err
	}

	outbound, err := outboundForNode(selected)
	if err != nil {
		return nil, err
	}
	outbound.Tag = "selected"

	cfg := xrayConfig{
		Log: xrayLog{LogLevel: firstNonEmpty(req.LogLevel, "warning")},
		Inbounds: []xrayInbound{
			{
				Tag:      "socks-in",
				Listen:   "127.0.0.1",
				Port:     fallbackPort(req.SOCKSPort, 10808),
				Protocol: "socks",
				Settings: struct {
					UDP bool `json:"udp"`
				}{UDP: true},
			},
			{
				Tag:      "http-in",
				Listen:   "127.0.0.1",
				Port:     fallbackPort(req.HTTPPort, 10809),
				Protocol: "http",
				Settings: struct{}{},
			},
		},
		Outbounds: []any{
			outbound,
			xrayCommonOutbound{Tag: "direct", Protocol: "freedom"},
			xrayCommonOutbound{Tag: "block", Protocol: "blackhole"},
		},
		Routing: xrayRouting{
			DomainStrategy: "AsIs",
			Rules: []xrayRouteRule{
				{Type: "field", OutboundTag: "selected", Network: "tcp,udp"},
			},
		},
	}

	rendered, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray config: %w", err)
	}

	return rendered, nil
}

func selectedNode(nodes []domain.Node, selectedID string) (domain.Node, error) {
	for _, node := range nodes {
		if node.ID == selectedID {
			return node, nil
		}
	}

	return domain.Node{}, fmt.Errorf("selected node %q not found", selectedID)
}

func fallbackPort(got, fallback int) int {
	if got > 0 {
		return got
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func outboundForNode(node domain.Node) (xrayCommonOutbound, error) {
	stream := make(map[string]any)
	if node.Transport != "" {
		stream["network"] = node.Transport
	}
	if node.Security != "" {
		stream["security"] = node.Security
	}

	switch node.Security {
	case "tls":
		tls := map[string]any{}
		if node.ServerName != "" {
			tls["serverName"] = node.ServerName
		}
		if len(node.ALPN) > 0 {
			tls["alpn"] = node.ALPN
		}
		if node.Fingerprint != "" {
			tls["fingerprint"] = node.Fingerprint
		}
		if len(tls) > 0 {
			stream["tlsSettings"] = tls
		}
	case "reality":
		stream["realitySettings"] = map[string]any{
			"fingerprint": node.Fingerprint,
			"publicKey":   node.PublicKey,
			"serverName":  node.ServerName,
			"shortId":     node.ShortID,
		}
	}

	switch node.Transport {
	case "ws":
		stream["wsSettings"] = map[string]any{
			"headers": map[string]string{"Host": node.Host},
			"path":    node.Path,
		}
	case "grpc":
		stream["grpcSettings"] = map[string]any{
			"serviceName": node.Path,
		}
	}

	switch node.Protocol {
	case domain.ProtocolVLESS:
		return xrayCommonOutbound{
			Protocol: "vless",
			Settings: map[string]any{
				"vnext": []map[string]any{
					{
						"address": node.Address,
						"port":    node.Port,
						"users": []map[string]any{
							{
								"id":         node.UUID,
								"encryption": firstNonEmpty(node.Encryption, "none"),
								"flow":       node.Flow,
							},
						},
					},
				},
			},
			StreamSettings: stream,
		}, nil
	case domain.ProtocolVMess:
		return xrayCommonOutbound{
			Protocol: "vmess",
			Settings: map[string]any{
				"vnext": []map[string]any{
					{
						"address": node.Address,
						"port":    node.Port,
						"users": []map[string]any{
							{
								"id":       node.UUID,
								"security": firstNonEmpty(node.Encryption, "auto"),
								"alterId":  0,
							},
						},
					},
				},
			},
			StreamSettings: stream,
		}, nil
	case domain.ProtocolTrojan:
		return xrayCommonOutbound{
			Protocol: "trojan",
			Settings: map[string]any{
				"servers": []map[string]any{
					{
						"address":  node.Address,
						"port":     node.Port,
						"password": node.Password,
					},
				},
			},
			StreamSettings: stream,
		}, nil
	default:
		return xrayCommonOutbound{}, fmt.Errorf("unsupported protocol %s", node.Protocol)
	}
}

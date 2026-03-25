package parser

import (
	"fmt"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func normalizeNode(node domain.Node, provider string) (domain.Node, error) {
	node.Address = strings.TrimSpace(node.Address)
	node.Name = strings.TrimSpace(node.Name)
	node.Remark = strings.TrimSpace(node.Remark)
	node.ProviderName = provider
	if node.Name == "" {
		node.Name = node.Remark
	}
	if node.Remark == "" {
		node.Remark = node.Name
	}
	node.Name = node.Remark
	if node.Transport == "" {
		node.Transport = "tcp"
	}
	if node.Extras == nil {
		node.Extras = map[string]string{}
	}

	switch node.Protocol {
	case domain.ProtocolVLESS, domain.ProtocolVMess:
		if node.UUID == "" {
			return domain.Node{}, fmt.Errorf("%s node missing uuid", node.Protocol)
		}
	case domain.ProtocolTrojan, domain.ProtocolShadowsocks:
		if node.Password == "" {
			return domain.Node{}, fmt.Errorf("%s node missing password", node.Protocol)
		}
	default:
		return domain.Node{}, fmt.Errorf("unsupported protocol %q", node.Protocol)
	}

	if node.Address == "" {
		return domain.Node{}, fmt.Errorf("%s node missing address", node.Protocol)
	}
	if node.Port <= 0 {
		return domain.Node{}, fmt.Errorf("%s node missing port", node.Protocol)
	}

	node.ID = domain.StableNodeID(node)
	return node, nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

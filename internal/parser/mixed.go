package parser

import (
	"fmt"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// ParseNodes parses mixed protocol subscription content into normalized nodes.
func ParseNodes(input, provider string) ([]domain.Node, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("empty subscription input")
	}

	if nodes, ok, err := tryParseJSONNodes(trimmed, provider); ok {
		return nodes, err
	}

	if looksLikeBase64Payload(trimmed) {
		decoded, err := decodeBase64Payload(trimmed)
		if err == nil && strings.Contains(decoded, "://") {
			trimmed = decoded
		}
	}

	if isNodeLink(trimmed) && !strings.Contains(trimmed, "\n") {
		node, err := parseSingleNode(trimmed, provider)
		if err != nil {
			return nil, err
		}
		return []domain.Node{node}, nil
	}

	lines := strings.Split(trimmed, "\n")
	nodes := make([]domain.Node, 0, len(lines))
	var lastErr error
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		node, err := parseSingleNode(line, provider)
		if err != nil {
			lastErr = fmt.Errorf("parse subscription line %q: %w", line, err)
			continue
		}

		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no supported nodes found")
	}

	return nodes, nil
}

func parseSingleNode(line, provider string) (domain.Node, error) {
	switch {
	case strings.HasPrefix(strings.ToLower(line), "vless://"):
		return ParseVLESS(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "vmess://"):
		return ParseVMess(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "trojan://"):
		return ParseTrojan(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "ss://"):
		return ParseShadowsocks(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "socks://"):
		return ParseSocks(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "hysteria://"):
		return ParseHysteria(line, provider)
	case strings.HasPrefix(strings.ToLower(line), "hysteria2://"), strings.HasPrefix(strings.ToLower(line), "hy2://"):
		return ParseHysteria2(line, provider)
	default:
		return domain.Node{}, fmt.Errorf("unsupported subscription entry")
	}
}

package parser

import (
	"strings"
)

func isNodeLink(input string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	return strings.HasPrefix(trimmed, "vless://") ||
		strings.HasPrefix(trimmed, "vmess://") ||
		strings.HasPrefix(trimmed, "trojan://") ||
		strings.HasPrefix(trimmed, "ss://")
}

func looksLikeBase64Payload(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return false
	}

	for _, ch := range trimmed {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '+', ch == '/', ch == '=', ch == '\n', ch == '\r':
		default:
			return false
		}
	}

	return true
}

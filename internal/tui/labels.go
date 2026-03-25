package tui

import (
	"fmt"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func nodeLabel(node domain.Node) string {
	for _, candidate := range []string{
		node.Name,
		node.Remark,
		fmt.Sprintf("%s:%d", node.Address, node.Port),
	} {
		if label := tuiText(candidate); label != "" {
			return label
		}
	}

	return node.DisplayName()
}

func tuiText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range value {
		if isHiddenTUIRune(r) {
			continue
		}
		builder.WriteRune(r)
	}

	return strings.Join(strings.Fields(builder.String()), " ")
}

func isHiddenTUIRune(r rune) bool {
	switch {
	case r == '\u200d':
		return true
	case r >= '\ufe00' && r <= '\ufe0f':
		return true
	case r >= '\U0001f1e6' && r <= '\U0001f1ff':
		return true
	case r >= '\U0001f300' && r <= '\U0001f5ff':
		return true
	case r >= '\U0001f600' && r <= '\U0001f64f':
		return true
	case r >= '\U0001f680' && r <= '\U0001f6ff':
		return true
	case r >= '\U0001f700' && r <= '\U0001f77f':
		return true
	case r >= '\U0001f780' && r <= '\U0001f7ff':
		return true
	case r >= '\U0001f800' && r <= '\U0001f8ff':
		return true
	case r >= '\U0001f900' && r <= '\U0001f9ff':
		return true
	case r >= '\U0001fa70' && r <= '\U0001faff':
		return true
	case r >= '\u2600' && r <= '\u27bf':
		return true
	default:
		return false
	}
}

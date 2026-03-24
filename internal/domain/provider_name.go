package domain

import (
	"net"
	"net/url"
	"strings"
	"unicode"
)

// HumanizeProviderName converts provider metadata or source hosts into a stable
// display title such as "Sample VPN" or "Demo VPN".
func HumanizeProviderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Imported VPN"
	}

	if !isDomainLike(value) {
		return value
	}

	label := providerDomainStem(value)
	if label == "" {
		return "Imported VPN"
	}

	label = strings.NewReplacer("-", " ", "_", " ").Replace(label)
	label = titleWords(label)
	if !strings.Contains(strings.ToLower(label), "vpn") {
		label += " VPN"
	}

	return strings.TrimSpace(label)
}

// ProviderNameFromURL derives a display provider title from a subscription URL.
func ProviderNameFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "Imported Subscription"
	}

	return HumanizeProviderName(rawURL)
}

func providerDomainStem(value string) string {
	host := normalizeProviderHost(value)
	if host == "" {
		return ""
	}

	parts := strings.Split(strings.ToLower(host), ".")
	if len(parts) == 0 {
		return ""
	}

	index := len(parts) - 1
	if len(parts) >= 2 {
		index = len(parts) - 2
		if len(parts) >= 3 && len(parts[len(parts)-1]) == 2 && isCommonSecondLevelLabel(parts[len(parts)-2]) {
			index = len(parts) - 3
		}
	}

	label := parts[index]
	for _, prefix := range []string{"conn", "vpn", "sub", "api", "www"} {
		if strings.HasPrefix(label, prefix) && len(label) > len(prefix)+2 {
			label = strings.TrimPrefix(label, prefix)
			break
		}
	}

	return strings.TrimSpace(label)
}

func normalizeProviderHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		return strings.TrimSpace(parsed.Hostname())
	}

	if slash := strings.Index(value, "/"); slash >= 0 {
		value = value[:slash]
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.TrimSpace(host)
	}

	return strings.TrimSpace(value)
}

func isCommonSecondLevelLabel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "co", "com", "net", "org", "gov", "ac", "edu":
		return true
	default:
		return false
	}
}

func isDomainLike(value string) bool {
	host := normalizeProviderHost(value)
	if host == "" || strings.Contains(host, " ") {
		return false
	}

	return strings.Contains(host, ".") && net.ParseIP(host) == nil
}

func titleWords(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	for idx, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[idx] = string(runes)
	}
	return strings.Join(parts, " ")
}

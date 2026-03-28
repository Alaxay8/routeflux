package domain

import (
	"fmt"
	"net"
	"strings"
)

var firewallTargetDomainFamilies = map[string][]string{
	"youtube.com": {
		"youtube.com",
		"youtu.be",
		"youtube-nocookie.com",
		"youtubei.googleapis.com",
		"youtube.googleapis.com",
		"googlevideo.com",
		"ytimg.com",
		"ggpht.com",
	},
	"instagram.com": {
		"instagram.com",
		"cdninstagram.com",
		"fbcdn.net",
	},
}

// FirewallTargets stores validated transparent proxy selectors.
type FirewallTargets struct {
	CIDRs   []string
	Domains []string
}

// ParseFirewallTargets validates and splits mixed IPv4 and domain selectors.
func ParseFirewallTargets(selectors []string) (FirewallTargets, error) {
	result := FirewallTargets{
		CIDRs:   make([]string, 0, len(selectors)),
		Domains: make([]string, 0, len(selectors)),
	}

	seenCIDRs := make(map[string]struct{}, len(selectors))
	seenDomains := make(map[string]struct{}, len(selectors))

	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}

		if normalized, ok, err := normalizeFirewallIPv4Selector(selector); err != nil {
			return FirewallTargets{}, err
		} else if ok {
			if _, seen := seenCIDRs[normalized]; seen {
				continue
			}
			seenCIDRs[normalized] = struct{}{}
			result.CIDRs = append(result.CIDRs, normalized)
			continue
		}

		normalized, err := normalizeFirewallDomain(selector)
		if err != nil {
			return FirewallTargets{}, err
		}
		if _, seen := seenDomains[normalized]; seen {
			continue
		}
		seenDomains[normalized] = struct{}{}
		result.Domains = append(result.Domains, normalized)
	}

	return result, nil
}

// ExpandFirewallTargetDomains expands well-known service roots to the domains
// they need while preserving user-entered domains verbatim for storage/UI.
func ExpandFirewallTargetDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))

	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain == "" {
			continue
		}

		family := firewallTargetDomainFamilies[domain]
		if len(family) == 0 {
			family = []string{domain}
		}

		for _, expanded := range family {
			if _, ok := seen[expanded]; ok {
				continue
			}
			seen[expanded] = struct{}{}
			out = append(out, expanded)
		}
	}

	return out
}

func normalizeFirewallIPv4Selector(value string) (string, bool, error) {
	value = strings.TrimSpace(value)

	switch {
	case parseIPv4(value), parseIPv4CIDR(value):
		return value, true, nil
	case strings.Contains(value, "-"):
		if err := validateIPv4Range(value); err != nil {
			return "", false, err
		}
		return value, true, nil
	default:
		return "", false, nil
	}
}

func normalizeFirewallDomain(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")

	switch {
	case value == "":
		return "", fmt.Errorf("unsupported target %q: empty values are not supported", value)
	case strings.Contains(value, "://"):
		return "", fmt.Errorf("unsupported target %q: URLs are not supported", value)
	case strings.ContainsAny(value, "/:@"):
		return "", fmt.Errorf("unsupported target %q: only IPv4, IPv4 CIDR, IPv4 ranges, and domain names are supported", value)
	case strings.Contains(value, "*"):
		return "", fmt.Errorf("unsupported target %q: wildcard domains are not supported", value)
	case strings.Count(value, ".") == 0:
		return "", fmt.Errorf("unsupported target %q: use a fully qualified domain like youtube.com", value)
	}

	labels := strings.Split(value, ".")
	for _, label := range labels {
		switch {
		case label == "":
			return "", fmt.Errorf("unsupported target %q: invalid domain name", value)
		case strings.HasPrefix(label, "-"), strings.HasSuffix(label, "-"):
			return "", fmt.Errorf("unsupported target %q: invalid domain label %q", value, label)
		}

		for _, r := range label {
			if r >= 'a' && r <= 'z' {
				continue
			}
			if r >= '0' && r <= '9' {
				continue
			}
			if r == '-' {
				continue
			}
			return "", fmt.Errorf("unsupported target %q: invalid domain label %q", value, label)
		}
	}

	return value, nil
}

func parseIPv4(value string) bool {
	ip := net.ParseIP(value)
	return ip != nil && ip.To4() != nil
}

func parseIPv4CIDR(value string) bool {
	ip, _, err := net.ParseCIDR(value)
	return err == nil && ip.To4() != nil
}

func validateIPv4Range(value string) error {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid IPv4 range %q", value)
	}

	startIP := net.ParseIP(strings.TrimSpace(parts[0]))
	endIP := net.ParseIP(strings.TrimSpace(parts[1]))
	if startIP == nil || startIP.To4() == nil || endIP == nil || endIP.To4() == nil {
		return fmt.Errorf("unsupported target %q: only IPv4, IPv4 CIDR, IPv4 ranges, and domain names are supported", value)
	}

	startBytes := startIP.To4()
	endBytes := endIP.To4()
	for idx := 0; idx < len(startBytes); idx++ {
		start := startBytes[idx]
		end := endBytes[idx]
		if start < end {
			return nil
		}
		if start > end {
			return fmt.Errorf("invalid IPv4 range %q: start must be <= end", value)
		}
	}

	return nil
}

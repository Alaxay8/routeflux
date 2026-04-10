package domain

import (
	"fmt"
	"slices"
	"strings"
)

// TransportMode identifies the currently active routing transport.
type TransportMode string

const (
	// TransportModeProxy means RouteFlux uses the proxy backend.
	TransportModeProxy TransportMode = "proxy"
	// TransportModeZapret means RouteFlux uses Zapret fallback.
	TransportModeZapret TransportMode = "zapret"
	// TransportModeDirect means RouteFlux is currently fail-open direct.
	TransportModeDirect TransportMode = "direct"
)

// ZapretSettings stores RouteFlux-managed Zapret fallback preferences.
type ZapretSettings struct {
	Enabled                  bool                `json:"enabled"`
	Selectors                FirewallSelectorSet `json:"selectors"`
	FailbackSuccessThreshold int                 `json:"failback_success_threshold"`
}

// ZapretStatus describes the observed Zapret runtime state.
type ZapretStatus struct {
	Installed     bool   `json:"installed"`
	Managed       bool   `json:"managed"`
	Active        bool   `json:"active"`
	ServiceActive bool   `json:"service_active"`
	TestActive    bool   `json:"test_active,omitempty"`
	ServiceState  string `json:"service_state"`
	LastReason    string `json:"last_reason,omitempty"`
}

// DefaultZapretSettings returns the baseline Zapret fallback configuration.
func DefaultZapretSettings() ZapretSettings {
	return ZapretSettings{
		Enabled:                  false,
		Selectors:                FirewallSelectorSet{},
		FailbackSuccessThreshold: 3,
	}
}

// NormalizeTransportMode coerces unknown values to direct.
func NormalizeTransportMode(mode TransportMode) TransportMode {
	switch mode {
	case TransportModeProxy, TransportModeZapret:
		return mode
	default:
		return TransportModeDirect
	}
}

// NormalizeZapretSettings deep-copies selectors and restores safe defaults.
func NormalizeZapretSettings(settings ZapretSettings) ZapretSettings {
	settings.Selectors = CloneFirewallSelectorSet(settings.Selectors)
	if settings.FailbackSuccessThreshold < 1 {
		settings.FailbackSuccessThreshold = DefaultZapretSettings().FailbackSuccessThreshold
	}
	return settings
}

// CanonicalZapretSettingsWithCatalog expands any legacy service aliases into
// plain domains and IPv4 selectors so runtime settings stay explicit.
func CanonicalZapretSettingsWithCatalog(settings ZapretSettings, customCatalog map[string]FirewallTargetDefinition) ZapretSettings {
	settings = NormalizeZapretSettings(settings)
	settings.Selectors.Domains = NormalizeZapretDomainList(ExpandZapretSelectorDomains(customCatalog, settings.Selectors))
	settings.Selectors.CIDRs = NormalizeZapretCIDRList(ExpandZapretSelectorCIDRs(customCatalog, settings.Selectors))
	settings.Selectors.Services = nil
	return settings
}

// ParseZapretSelectors validates selectors supported by Zapret fallback.
func ParseZapretSelectors(selectors []string, customCatalog map[string]FirewallTargetDefinition) (FirewallSelectorSet, error) {
	_ = customCatalog
	result := FirewallSelectorSet{
		Domains: make([]string, 0, len(selectors)),
		CIDRs:   make([]string, 0, len(selectors)),
	}

	seenDomains := make(map[string]struct{}, len(selectors))
	seenCIDRs := make(map[string]struct{}, len(selectors))

	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}

		if normalized, ok, err := normalizeFirewallIPv4Selector(selector); err != nil {
			return FirewallSelectorSet{}, err
		} else if ok {
			if _, seen := seenCIDRs[normalized]; seen {
				continue
			}
			seenCIDRs[normalized] = struct{}{}
			result.CIDRs = append(result.CIDRs, normalized)
			continue
		}

		if !strings.Contains(selector, ".") {
			return FirewallSelectorSet{}, fmt.Errorf("unsupported zapret selector %q: use a fully qualified domain like youtube.com", strings.TrimSpace(strings.ToLower(selector)))
		}

		normalized, err := normalizeZapretDomain(selector)
		if err != nil {
			return FirewallSelectorSet{}, err
		}
		if _, seen := seenDomains[normalized]; seen {
			continue
		}
		seenDomains[normalized] = struct{}{}
		result.Domains = append(result.Domains, normalized)
	}

	return result, nil
}

// ExpandZapretSelectorDomains expands any legacy service aliases into domains.
func ExpandZapretSelectorDomains(customCatalog map[string]FirewallTargetDefinition, selectors FirewallSelectorSet) []string {
	return ExpandFirewallTargetDomains(customCatalog, selectors.Services, selectors.Domains)
}

// ExpandZapretSelectorCIDRs expands legacy service aliases to static IPv4 selectors.
func ExpandZapretSelectorCIDRs(customCatalog map[string]FirewallTargetDefinition, selectors FirewallSelectorSet) []string {
	return ExpandFirewallTargetCIDRs(customCatalog, selectors.Services, selectors.CIDRs)
}

// ZapretSelectorSetHasEntries reports whether the selector set contains usable Zapret selectors.
func ZapretSelectorSetHasEntries(value FirewallSelectorSet) bool {
	return len(value.Services) > 0 || len(value.Domains) > 0 || len(value.CIDRs) > 0
}

// NormalizeZapretDomainList sorts and deduplicates expanded domains.
func NormalizeZapretDomainList(domains []string) []string {
	return normalizeZapretList(domains)
}

// NormalizeZapretCIDRList sorts and deduplicates expanded IPv4 selectors.
func NormalizeZapretCIDRList(cidrs []string) []string {
	return normalizeZapretList(cidrs)
}

func normalizeZapretList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	if len(cleaned) == 0 {
		return nil
	}
	slices.Sort(cleaned)
	return cleaned
}

func normalizeZapretDomain(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")

	switch {
	case value == "":
		return "", fmt.Errorf("unsupported zapret selector %q: empty values are not supported", value)
	case strings.Contains(value, "://"):
		return "", fmt.Errorf("unsupported zapret selector %q: URLs are not supported", value)
	case strings.ContainsAny(value, "/:@"):
		return "", fmt.Errorf("unsupported zapret selector %q: only fully qualified domains are supported", value)
	case strings.Contains(value, "*"):
		return "", fmt.Errorf("unsupported zapret selector %q: wildcard domains are not supported", value)
	case strings.Count(value, ".") == 0:
		return "", fmt.Errorf("unsupported zapret selector %q: use a fully qualified domain like youtube.com", value)
	}

	labels := strings.Split(value, ".")
	for _, label := range labels {
		switch {
		case label == "":
			return "", fmt.Errorf("unsupported zapret selector %q: invalid domain name", value)
		case strings.HasPrefix(label, "-"), strings.HasSuffix(label, "-"):
			return "", fmt.Errorf("unsupported zapret selector %q: invalid domain label %q", value, label)
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
			return "", fmt.Errorf("unsupported zapret selector %q: invalid domain label %q", value, label)
		}
	}

	return value, nil
}

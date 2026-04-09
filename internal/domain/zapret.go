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
	settings.Selectors.CIDRs = nil
	if settings.FailbackSuccessThreshold < 1 {
		settings.FailbackSuccessThreshold = DefaultZapretSettings().FailbackSuccessThreshold
	}
	return settings
}

// ParseZapretSelectors validates selectors supported by Zapret fallback.
func ParseZapretSelectors(selectors []string, customCatalog map[string]FirewallTargetDefinition) (FirewallSelectorSet, error) {
	registry := mergeFirewallTargetRegistry(customCatalog)
	result := FirewallSelectorSet{
		Services: make([]string, 0, len(selectors)),
		Domains:  make([]string, 0, len(selectors)),
	}

	seenServices := make(map[string]struct{}, len(selectors))
	seenDomains := make(map[string]struct{}, len(selectors))

	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}

		if normalized, ok, err := normalizeFirewallIPv4Selector(selector); err != nil {
			return FirewallSelectorSet{}, err
		} else if ok {
			return FirewallSelectorSet{}, fmt.Errorf("unsupported zapret selector %q: only service aliases and fully qualified domains are supported", normalized)
		}

		if normalized, ok := normalizeFirewallTargetService(selector, registry); ok {
			definition := registry[normalized]
			if len(definition.Domains) == 0 && len(definition.CIDRs) == 0 {
				return FirewallSelectorSet{}, fmt.Errorf("zapret service %q does not resolve to any domains or CIDRs", normalized)
			}
			if _, seen := seenServices[normalized]; seen {
				continue
			}
			seenServices[normalized] = struct{}{}
			result.Services = append(result.Services, normalized)
			continue
		}

		if normalized, err := normalizeFirewallTargetAlias(selector); err == nil && !strings.Contains(selector, ".") {
			return FirewallSelectorSet{}, fmt.Errorf("unknown zapret service %q: create it with routeflux services set %s <domain...> or use a fully qualified domain", selector, normalized)
		}

		normalized, err := normalizeFirewallDomain(selector)
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

// ExpandZapretSelectorDomains expands Zapret selectors to routeable domains.
func ExpandZapretSelectorDomains(customCatalog map[string]FirewallTargetDefinition, selectors FirewallSelectorSet) []string {
	return ExpandFirewallTargetDomains(customCatalog, selectors.Services, selectors.Domains)
}

// ExpandZapretSelectorCIDRs expands Zapret selectors to static IPv4 selectors.
func ExpandZapretSelectorCIDRs(customCatalog map[string]FirewallTargetDefinition, selectors FirewallSelectorSet) []string {
	return ExpandFirewallTargetCIDRs(customCatalog, selectors.Services, selectors.CIDRs)
}

// ZapretSelectorSetHasEntries reports whether the selector set contains usable Zapret selectors.
func ZapretSelectorSetHasEntries(value FirewallSelectorSet) bool {
	return len(value.Services) > 0 || len(value.Domains) > 0
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

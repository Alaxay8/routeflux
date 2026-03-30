package domain

import (
	"fmt"
	"net"
	"slices"
	"strings"
)

// FirewallTargetDefinition stores domains and IPv4 selectors for a target service.
type FirewallTargetDefinition struct {
	Domains []string `json:"domains"`
	CIDRs   []string `json:"cidrs"`
}

// FirewallTargetServiceSource identifies whether a target service is built in or user-defined.
type FirewallTargetServiceSource string

const (
	FirewallTargetServiceSourceBuiltin FirewallTargetServiceSource = "builtin"
	FirewallTargetServiceSourceCustom  FirewallTargetServiceSource = "custom"
)

// FirewallTargetService describes one resolved service entry from the merged registry.
type FirewallTargetService struct {
	Name     string                      `json:"name"`
	Source   FirewallTargetServiceSource `json:"source"`
	ReadOnly bool                        `json:"readonly"`
	Domains  []string                    `json:"domains"`
	CIDRs    []string                    `json:"cidrs"`
}

var googleAIMobileSharedDomains = []string{
	"myaccount.google.com",
	"one.google.com",
	"support.google.com",
	"www.google.com",
	"mtalk.google.com",
	"dns.google.com",
	"dns.google",
	"googleapis.com",
	"clients6.google.com",
	"gstatic.com",
	"googleusercontent.com",
}

var googleAIMobileSharedCIDRs = []string{
	"74.125.205.0/24",
	"173.194.73.0/24",
}

var firewallTargetServicePresets = map[string]FirewallTargetDefinition{
	"youtube": {
		Domains: []string{
			"youtube.com",
			"youtu.be",
			"youtube-nocookie.com",
			"youtubei.googleapis.com",
			"youtube.googleapis.com",
			"googlevideo.com",
			"ytimg.com",
			"ggpht.com",
		},
	},
	"instagram": {
		Domains: []string{
			"instagram.com",
			"cdninstagram.com",
			"fbcdn.net",
			"facebook.com",
			"facebook.net",
			"fbsbx.com",
		},
	},
	"netflix": {
		Domains: []string{
			"netflix.com",
			"netflix.net",
			"nflxvideo.net",
			"nflximg.net",
			"nflximg.com",
			"nflxso.net",
			"nflxext.com",
			"nflxsearch.net",
			"fast.com",
		},
	},
	"discord": {
		Domains: []string{
			"discord.com",
			"discord.gg",
			"discord.gift",
			"discord.media",
			"discordapp.com",
			"discordapp.net",
		},
	},
	"whatsapp": {
		Domains: []string{
			"whatsapp.com",
			"web.whatsapp.com",
			"whatsapp.net",
			"static.whatsapp.net",
			"mmg.whatsapp.net",
			"graph.whatsapp.net",
			"pps.whatsapp.net",
			"g.whatsapp.net",
		},
	},
	"telegram-web": {
		Domains: []string{
			"telegram.org",
			"t.me",
			"telegram.me",
			"web.telegram.org",
			"desktop.telegram.org",
			"core.telegram.org",
		},
	},
	"telegram": {
		Domains: []string{
			"telegram.org",
			"t.me",
			"telegram.me",
			"web.telegram.org",
			"desktop.telegram.org",
			"core.telegram.org",
		},
		CIDRs: []string{
			"91.108.0.0/16",
			"149.154.0.0/16",
		},
	},
	"twitter": {
		Domains: []string{
			"x.com",
			"twitter.com",
			"t.co",
			"twimg.com",
		},
	},
	"facetime": {
		Domains: []string{
			"facetime.apple.com",
			"facetime.icloud.com",
			"gateway.icloud.com",
			"relay.icloud.com",
			"push.apple.com",
			"courier.push.apple.com",
		},
	},
	"gemini": {
		Domains: []string{
			"accounts.google.com",
			"content.googleapis.com",
			"gemini.google.com",
			"geminiweb-pa.clients6.google.com",
			"generativelanguage.googleapis.com",
			"lh3.googleusercontent.com",
			"myaccount.google.com",
			"ogads-pa.clients6.google.com",
			"one.google.com",
			"ssl.gstatic.com",
			"support.google.com",
			"waa-pa.clients6.google.com",
			"www.google.com",
			"www.gstatic.com",
		},
	},
	"gemini-mobile": {
		Domains: googleAIMobileDomains("gemini.google.com"),
		CIDRs:   slices.Clone(googleAIMobileSharedCIDRs),
	},
	"notebooklm": {
		Domains: []string{
			"accounts.google.com",
			"content.googleapis.com",
			"generativelanguage.googleapis.com",
			"lh3.googleusercontent.com",
			"notebooklm.google.com",
		},
	},
	"notebooklm-mobile": {
		Domains: googleAIMobileDomains("notebooklm.google.com"),
		CIDRs:   slices.Clone(googleAIMobileSharedCIDRs),
	},
}

var firewallTargetDomainFamilies = map[string]string{
	"youtube.com":           "youtube",
	"instagram.com":         "instagram",
	"netflix.com":           "netflix",
	"discord.com":           "discord",
	"whatsapp.com":          "whatsapp",
	"x.com":                 "twitter",
	"twitter.com":           "twitter",
	"gemini.google.com":     "gemini",
	"notebooklm.google.com": "notebooklm",
	"telegram.org":          "telegram-web",
	"web.telegram.org":      "telegram-web",
	"facetime.apple.com":    "facetime",
}

// FirewallTargets stores validated transparent proxy selectors.
type FirewallTargets struct {
	Services []string
	CIDRs    []string
	Domains  []string
}

// CloneFirewallTargetCatalog deep-copies the provided service catalog.
func CloneFirewallTargetCatalog(catalog map[string]FirewallTargetDefinition) map[string]FirewallTargetDefinition {
	if len(catalog) == 0 {
		return nil
	}

	cloned := make(map[string]FirewallTargetDefinition, len(catalog))
	for name, definition := range catalog {
		cloned[name] = cloneFirewallTargetDefinition(definition)
	}
	return cloned
}

// ParseFirewallTargets validates and splits mixed IPv4, service, and domain selectors.
func ParseFirewallTargets(selectors []string, customCatalog map[string]FirewallTargetDefinition) (FirewallTargets, error) {
	registry := mergeFirewallTargetRegistry(customCatalog)
	result := FirewallTargets{
		Services: make([]string, 0, len(selectors)),
		CIDRs:    make([]string, 0, len(selectors)),
		Domains:  make([]string, 0, len(selectors)),
	}

	seenServices := make(map[string]struct{}, len(selectors))
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

		if normalized, ok := normalizeFirewallTargetService(selector, registry); ok {
			if _, seen := seenServices[normalized]; seen {
				continue
			}
			seenServices[normalized] = struct{}{}
			result.Services = append(result.Services, normalized)
			continue
		}

		if normalized, err := normalizeFirewallTargetAlias(selector); err == nil && !strings.Contains(selector, ".") {
			return FirewallTargets{}, fmt.Errorf("unknown target service %q: create it with routeflux services set %s <domain-or-ip...> or use a fully qualified domain like youtube.com", selector, normalized)
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

// ParseFirewallTargetDefinition validates a user-defined target service entry.
func ParseFirewallTargetDefinition(name string, selectors []string) (string, FirewallTargetDefinition, error) {
	normalizedName, err := normalizeFirewallTargetAlias(name)
	if err != nil {
		return "", FirewallTargetDefinition{}, err
	}
	if isBuiltinFirewallTargetService(normalizedName) {
		return "", FirewallTargetDefinition{}, fmt.Errorf("target service %q is readonly and reserved by a built-in preset", normalizedName)
	}

	definition := FirewallTargetDefinition{
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
			return "", FirewallTargetDefinition{}, fmt.Errorf("parse target service %q selector %q: %w", normalizedName, selector, err)
		} else if ok {
			if _, seen := seenCIDRs[normalized]; seen {
				continue
			}
			seenCIDRs[normalized] = struct{}{}
			definition.CIDRs = append(definition.CIDRs, normalized)
			continue
		}

		normalized, err := normalizeFirewallDomain(selector)
		if err != nil {
			return "", FirewallTargetDefinition{}, fmt.Errorf("parse target service %q selector %q: %w", normalizedName, selector, err)
		}
		if _, seen := seenDomains[normalized]; seen {
			continue
		}
		seenDomains[normalized] = struct{}{}
		definition.Domains = append(definition.Domains, normalized)
	}

	if len(definition.Domains) == 0 && len(definition.CIDRs) == 0 {
		return "", FirewallTargetDefinition{}, fmt.Errorf("target service %q requires at least one domain or IPv4 selector", normalizedName)
	}

	return normalizedName, definition, nil
}

// FirewallTargetServiceNames returns the built-in preset names accepted by firewall targets.
func FirewallTargetServiceNames() []string {
	names := make([]string, 0, len(firewallTargetServicePresets))
	for name := range firewallTargetServicePresets {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// FirewallTargetServices returns the merged built-in and custom registry as sorted entries.
func FirewallTargetServices(customCatalog map[string]FirewallTargetDefinition) []FirewallTargetService {
	services := make([]FirewallTargetService, 0, len(firewallTargetServicePresets)+len(customCatalog))

	for _, name := range FirewallTargetServiceNames() {
		definition := firewallTargetServicePresets[name]
		services = append(services, FirewallTargetService{
			Name:     name,
			Source:   FirewallTargetServiceSourceBuiltin,
			ReadOnly: true,
			Domains:  append([]string(nil), definition.Domains...),
			CIDRs:    append([]string(nil), definition.CIDRs...),
		})
	}

	customNames := make([]string, 0, len(customCatalog))
	for name := range customCatalog {
		if isBuiltinFirewallTargetService(name) {
			continue
		}
		customNames = append(customNames, name)
	}
	slices.Sort(customNames)
	for _, name := range customNames {
		definition := customCatalog[name]
		services = append(services, FirewallTargetService{
			Name:     name,
			Source:   FirewallTargetServiceSourceCustom,
			ReadOnly: false,
			Domains:  append([]string(nil), definition.Domains...),
			CIDRs:    append([]string(nil), definition.CIDRs...),
		})
	}

	return services
}

// LookupFirewallTargetService returns one service entry from the merged registry.
func LookupFirewallTargetService(customCatalog map[string]FirewallTargetDefinition, name string) (FirewallTargetService, bool) {
	normalized, err := normalizeFirewallTargetAlias(name)
	if err != nil {
		return FirewallTargetService{}, false
	}

	if definition, ok := firewallTargetServicePresets[normalized]; ok {
		return FirewallTargetService{
			Name:     normalized,
			Source:   FirewallTargetServiceSourceBuiltin,
			ReadOnly: true,
			Domains:  append([]string(nil), definition.Domains...),
			CIDRs:    append([]string(nil), definition.CIDRs...),
		}, true
	}

	definition, ok := customCatalog[normalized]
	if !ok {
		return FirewallTargetService{}, false
	}

	return FirewallTargetService{
		Name:     normalized,
		Source:   FirewallTargetServiceSourceCustom,
		ReadOnly: false,
		Domains:  append([]string(nil), definition.Domains...),
		CIDRs:    append([]string(nil), definition.CIDRs...),
	}, true
}

// ExpandFirewallTargetDomains expands service aliases and well-known built-in root domains.
func ExpandFirewallTargetDomains(customCatalog map[string]FirewallTargetDefinition, services, domains []string) []string {
	registry := mergeFirewallTargetRegistry(customCatalog)
	out := make([]string, 0, len(services)+len(domains))
	seen := make(map[string]struct{}, len(services)+len(domains))

	for _, service := range services {
		appendFirewallServiceDomains(registry, &out, seen, service)
	}

	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain == "" {
			continue
		}

		if service, ok := firewallTargetDomainFamilies[domain]; ok {
			appendFirewallServiceDomains(registry, &out, seen, service)
			continue
		}

		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}

	return out
}

// ExpandFirewallTargetCIDRs expands service aliases to static CIDR targets and appends user-entered IPv4 selectors.
func ExpandFirewallTargetCIDRs(customCatalog map[string]FirewallTargetDefinition, services, cidrs []string) []string {
	registry := mergeFirewallTargetRegistry(customCatalog)
	out := make([]string, 0, len(services)+len(cidrs))
	seen := make(map[string]struct{}, len(services)+len(cidrs))

	for _, service := range services {
		appendFirewallServiceCIDRs(registry, &out, seen, service)
	}

	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		out = append(out, cidr)
	}

	return out
}

func mergeFirewallTargetRegistry(customCatalog map[string]FirewallTargetDefinition) map[string]FirewallTargetDefinition {
	registry := make(map[string]FirewallTargetDefinition, len(firewallTargetServicePresets)+len(customCatalog))
	for name, definition := range customCatalog {
		registry[name] = cloneFirewallTargetDefinition(definition)
	}
	for name, definition := range firewallTargetServicePresets {
		registry[name] = cloneFirewallTargetDefinition(definition)
	}
	return registry
}

func cloneFirewallTargetDefinition(definition FirewallTargetDefinition) FirewallTargetDefinition {
	return FirewallTargetDefinition{
		Domains: append([]string(nil), definition.Domains...),
		CIDRs:   append([]string(nil), definition.CIDRs...),
	}
}

func appendFirewallServiceDomains(registry map[string]FirewallTargetDefinition, out *[]string, seen map[string]struct{}, service string) {
	service = strings.TrimSpace(strings.ToLower(service))
	definition, ok := registry[service]
	if !ok {
		return
	}

	for _, domain := range definition.Domains {
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		*out = append(*out, domain)
	}
}

func appendFirewallServiceCIDRs(registry map[string]FirewallTargetDefinition, out *[]string, seen map[string]struct{}, service string) {
	service = strings.TrimSpace(strings.ToLower(service))
	definition, ok := registry[service]
	if !ok {
		return
	}

	for _, cidr := range definition.CIDRs {
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		*out = append(*out, cidr)
	}
}

func normalizeFirewallTargetService(value string, registry map[string]FirewallTargetDefinition) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", false
	}
	if _, ok := registry[value]; !ok {
		return "", false
	}
	return value, true
}

func isBuiltinFirewallTargetService(name string) bool {
	_, ok := firewallTargetServicePresets[name]
	return ok
}

func normalizeFirewallTargetAlias(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", fmt.Errorf("target service name cannot be empty")
	}

	for idx, r := range value {
		switch {
		case idx == 0 && r >= 'a' && r <= 'z':
			continue
		case idx > 0 && r >= 'a' && r <= 'z':
			continue
		case idx > 0 && r >= '0' && r <= '9':
			continue
		case idx > 0 && r == '-':
			continue
		default:
			return "", fmt.Errorf("invalid target service name %q: use lowercase letters, digits, and hyphens, starting with a letter", value)
		}
	}

	return value, nil
}

func normalizeFirewallIPv4Selector(value string) (string, bool, error) {
	value = strings.TrimSpace(value)

	switch {
	case parseIPv4(value), parseIPv4CIDR(value):
		return value, true, nil
	case looksLikeIPv4Range(value):
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
		return "", fmt.Errorf("unsupported target %q: only IPv4, IPv4 CIDR, IPv4 ranges, service presets, and domain names are supported", value)
	case strings.Contains(value, "*"):
		return "", fmt.Errorf("unsupported target %q: wildcard domains are not supported", value)
	case strings.Count(value, ".") == 0:
		return "", fmt.Errorf("unsupported target %q: use a supported service preset or a fully qualified domain like youtube.com", value)
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
		return fmt.Errorf("unsupported target %q: only IPv4, IPv4 CIDR, IPv4 ranges, service presets, and domain names are supported", value)
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

func looksLikeIPv4Range(value string) bool {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return false
	}

	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	if start == "" || end == "" {
		return false
	}

	return parseIPv4(start) && parseIPv4(end)
}

func googleAIMobileDomains(primaryHost string) []string {
	domains := make([]string, 0, len(googleAIMobileSharedDomains)+2)
	domains = append(domains, "accounts.google.com", primaryHost)
	domains = append(domains, googleAIMobileSharedDomains...)
	return domains
}

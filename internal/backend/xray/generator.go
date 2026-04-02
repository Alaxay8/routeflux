package xray

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"strings"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
)

// Generator builds Xray configuration files from RouteFlux nodes.
type Generator struct{}

const (
	transparentTCPInboundTag = "transparent-in"
	transparentUDPInboundTag = "transparent-udp-in"
)

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

	dnsConfig, err := buildDNSConfig(req.DNS)
	if err != nil {
		return nil, err
	}

	cfg := xrayConfig{
		Log: xrayLog{LogLevel: firstNonEmpty(req.LogLevel, "warning")},
		DNS: dnsConfig,
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
			Rules:          []xrayRouteRule{},
		},
	}

	if rule, err := directDNSRouteRule(req.DNS); err != nil {
		return nil, err
	} else if rule != nil {
		cfg.Routing.Rules = append(cfg.Routing.Rules, *rule)
	}

	if req.TransparentProxy {
		port := fallbackPort(req.TransparentPort, 12345)
		cfg.Inbounds = append(cfg.Inbounds,
			transparentInbound(transparentTCPInboundTag, port, "tcp", "redirect"),
			transparentInbound(transparentUDPInboundTag, port, "udp", "tproxy"),
		)
	}

	cfg.Routing.Rules = append(cfg.Routing.Rules, transparentRoutingRules(req)...)
	cfg.Routing.Rules = append(cfg.Routing.Rules, xrayRouteRule{
		Type:        "field",
		OutboundTag: "selected",
		InboundTag:  []string{"socks-in", "http-in"},
		Network:     "tcp,udp",
	})

	rendered, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal xray config: %w", err)
	}

	return rendered, nil
}

func transparentRoutingRules(req backend.ConfigRequest) []xrayRouteRule {
	if !req.TransparentProxy {
		return nil
	}

	if req.TransparentSelectiveCapture {
		return transparentSelectiveRoutingRules(req)
	}

	rules := make([]xrayRouteRule, 0, 12)
	blockQUIC := req.TransparentBlockQUIC
	defaultAction := req.TransparentDefaultAction
	if defaultAction == "" {
		defaultAction = domain.FirewallDefaultActionProxy
	} else {
		defaultAction = domain.NormalizeFirewallDefaultAction(defaultAction)
	}
	hasSelectors := len(req.TransparentProxyDomains) > 0 ||
		len(req.TransparentProxyCIDRs) > 0 ||
		len(req.TransparentBypassDomains) > 0 ||
		len(req.TransparentBypassCIDRs) > 0

	if len(req.TransparentBypassDomains) > 0 {
		rules = append(rules, transparentDomainRules("direct", req.TransparentBypassDomains, blockQUIC)...)
	}
	if len(req.TransparentBypassCIDRs) > 0 {
		rules = append(rules, transparentIPRules("direct", req.TransparentBypassCIDRs, blockQUIC)...)
	}

	if len(req.TransparentProxyDomains) > 0 {
		rules = append(rules, transparentDomainRules("selected", req.TransparentProxyDomains, blockQUIC)...)
	}
	if len(req.TransparentProxyCIDRs) > 0 {
		rules = append(rules, transparentIPRules("selected", req.TransparentProxyCIDRs, blockQUIC)...)
	}

	if hasSelectors {
		rules = append(rules, transparentFallbackRules(defaultAction, blockQUIC)...)
		return rules
	}

	if defaultAction == domain.FirewallDefaultActionDirect {
		rules = append(rules,
			transparentRouteRule("tcp", "direct", nil, nil),
			transparentRouteRule("udp", "direct", nil, nil),
		)
		return rules
	}

	rules = append(rules,
		transparentRouteRule("tcp", "selected", nil, nil),
		transparentRouteRule("udp", transparentProxiedUDPOutbound(blockQUIC), nil, nil),
	)

	return rules
}

func transparentSelectiveRoutingRules(req backend.ConfigRequest) []xrayRouteRule {
	rules := make([]xrayRouteRule, 0, 6)
	blockQUIC := req.TransparentBlockQUIC

	if len(req.TransparentBypassDomains) > 0 {
		rules = append(rules, transparentDomainRules("direct", req.TransparentBypassDomains, blockQUIC)...)
	}
	if len(req.TransparentBypassCIDRs) > 0 {
		rules = append(rules, transparentIPRules("direct", req.TransparentBypassCIDRs, blockQUIC)...)
	}

	rules = append(rules,
		transparentRouteRule("tcp", "selected", nil, nil),
		transparentRouteRule("udp", transparentProxiedUDPOutbound(blockQUIC), nil, nil),
	)

	return rules
}

func transparentDomainRules(tcpOutbound string, domains []string, blockQUIC bool) []xrayRouteRule {
	matchers := routeDomainMatchers(domains)
	if len(matchers) == 0 {
		return nil
	}

	udpOutbound := transparentUDPOutboundForTCPOutbound(tcpOutbound, blockQUIC)

	return []xrayRouteRule{
		transparentRouteRule("tcp", tcpOutbound, matchers, nil),
		transparentRouteRule("udp", udpOutbound, matchers, nil),
	}
}

func transparentIPRules(tcpOutbound string, cidrs []string, blockQUIC bool) []xrayRouteRule {
	cleaned := cleanStringList(cidrs)
	if len(cleaned) == 0 {
		return nil
	}

	udpOutbound := transparentUDPOutboundForTCPOutbound(tcpOutbound, blockQUIC)

	return []xrayRouteRule{
		transparentRouteRule("tcp", tcpOutbound, nil, cleaned),
		transparentRouteRule("udp", udpOutbound, nil, cleaned),
	}
}

func transparentFallbackRules(defaultAction domain.FirewallDefaultAction, blockQUIC bool) []xrayRouteRule {
	tcpOutbound := "direct"
	udpOutbound := "direct"
	if defaultAction == domain.FirewallDefaultActionProxy {
		tcpOutbound = "selected"
		udpOutbound = transparentProxiedUDPOutbound(blockQUIC)
	}

	return []xrayRouteRule{
		transparentRouteRule("tcp", tcpOutbound, nil, nil),
		transparentRouteRule("udp", udpOutbound, nil, nil),
	}
}

func transparentProxiedUDPOutbound(blockQUIC bool) string {
	if blockQUIC {
		return "block"
	}
	return "selected"
}

func transparentUDPOutboundForTCPOutbound(tcpOutbound string, blockQUIC bool) string {
	if tcpOutbound == "direct" {
		return "direct"
	}
	return transparentProxiedUDPOutbound(blockQUIC)
}

func transparentRouteRule(network string, outboundTag string, domains []string, cidrs []string) xrayRouteRule {
	return xrayRouteRule{
		Type:        "field",
		OutboundTag: outboundTag,
		InboundTag:  transparentInboundTagsForNetwork(network),
		Network:     network,
		Domain:      domains,
		IP:          cidrs,
	}
}

func transparentInbound(tag string, port int, network string, tproxyMode string) xrayInbound {
	return xrayInbound{
		Tag:      tag,
		Listen:   "0.0.0.0",
		Port:     port,
		Protocol: "dokodemo-door",
		Settings: map[string]any{
			"followRedirect": true,
			"network":        network,
		},
		Sniffing: map[string]any{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic"},
		},
		StreamSettings: map[string]any{
			"sockopt": map[string]any{
				"tproxy": tproxyMode,
			},
		},
	}
}

func transparentInboundTags() []string {
	return []string{transparentTCPInboundTag, transparentUDPInboundTag}
}

func transparentInboundTagsForNetwork(network string) []string {
	switch network {
	case "udp":
		return []string{transparentUDPInboundTag}
	default:
		return []string{transparentTCPInboundTag}
	}
}

func routeDomainMatchers(domains []string) []string {
	cleaned := cleanStringList(domains)
	if len(cleaned) == 0 {
		return nil
	}

	out := make([]string, 0, len(cleaned))
	for _, domain := range cleaned {
		out = append(out, "domain:"+domain)
	}
	return out
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

func buildDNSConfig(settings domain.DNSSettings) (*xrayDNS, error) {
	mode, err := domain.ParseDNSMode(string(settings.Mode))
	if err != nil {
		return nil, err
	}

	switch mode {
	case domain.DNSModeSystem, domain.DNSModeDisabled:
		return nil, nil
	}

	transport, err := domain.ParseDNSTransport(string(settings.Transport))
	if err != nil {
		return nil, err
	}

	servers, err := formatDNSServers(settings.Servers, transport)
	if err != nil {
		return nil, err
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("dns servers are required when dns mode is %q", mode)
	}

	result := make([]any, 0, len(servers)+len(settings.Bootstrap)+1)
	if mode == domain.DNSModeSplit {
		directDomains := cleanStringList(settings.DirectDomains)
		if len(directDomains) > 0 {
			result = append(result, xrayDNSServer{
				Address:      "localhost",
				Domains:      directDomains,
				SkipFallback: true,
			})
		}
	}

	bootstrapDomains := dnsBootstrapDomains(servers)
	if len(bootstrapDomains) > 0 {
		bootstrapServers, err := formatDNSServers(settings.Bootstrap, domain.DNSTransportPlain)
		if err != nil {
			return nil, err
		}
		for _, server := range bootstrapServers {
			result = append(result, xrayDNSServer{
				Address:      server,
				Domains:      bootstrapDomains,
				SkipFallback: true,
			})
		}
	}

	for _, server := range servers {
		result = append(result, server)
	}

	return &xrayDNS{Servers: result}, nil
}

func directDNSRouteRule(settings domain.DNSSettings) (*xrayRouteRule, error) {
	mode, err := domain.ParseDNSMode(string(settings.Mode))
	if err != nil {
		return nil, err
	}
	if mode == domain.DNSModeSystem || mode == domain.DNSModeDisabled {
		return nil, nil
	}

	transport, err := domain.ParseDNSTransport(string(settings.Transport))
	if err != nil {
		return nil, err
	}

	servers, err := formatDNSServers(settings.Servers, transport)
	if err != nil {
		return nil, err
	}
	bootstrapServers, err := formatDNSServers(settings.Bootstrap, domain.DNSTransportPlain)
	if err != nil {
		return nil, err
	}

	ips, domains := directDNSDestinations(append(servers, bootstrapServers...))
	if len(ips) == 0 && len(domains) == 0 {
		return nil, nil
	}

	return &xrayRouteRule{
		Type:        "field",
		OutboundTag: "direct",
		Domain:      domains,
		IP:          ips,
	}, nil
}

func formatDNSServers(servers []string, transport domain.DNSTransport) ([]string, error) {
	cleaned := cleanStringList(servers)
	if len(cleaned) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(cleaned))
	for _, server := range cleaned {
		if hasScheme(server) {
			out = append(out, server)
			continue
		}

		switch transport {
		case domain.DNSTransportPlain:
			out = append(out, server)
		case domain.DNSTransportDoH:
			out = append(out, formatDoHServer(server))
		case domain.DNSTransportDoT:
			return nil, fmt.Errorf("dns transport %q is not supported by the current xray backend", transport)
		default:
			return nil, fmt.Errorf("unsupported dns transport %q", transport)
		}
	}

	return out, nil
}

func formatDoHServer(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return ""
	}
	if strings.Contains(server, "/") {
		return "https://" + strings.TrimPrefix(server, "https://")
	}
	return "https://" + server + "/dns-query"
}

func dnsBootstrapDomains(servers []string) []string {
	seen := make(map[string]struct{}, len(servers))
	out := make([]string, 0, len(servers))
	for _, server := range servers {
		host := dnsServerHostname(server)
		if host == "" {
			continue
		}
		if _, err := netip.ParseAddr(host); err == nil {
			continue
		}

		domain := "full:" + host
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	return out
}

func dnsServerHostname(server string) string {
	if hasScheme(server) {
		if parsed, err := url.Parse(server); err == nil {
			return parsed.Hostname()
		}
	}
	if parsed, err := url.Parse("//" + server); err == nil {
		return parsed.Hostname()
	}
	return ""
}

func directDNSDestinations(servers []string) ([]string, []string) {
	seenIPs := make(map[string]struct{}, len(servers))
	seenDomains := make(map[string]struct{}, len(servers))
	ips := make([]string, 0, len(servers))
	domains := make([]string, 0, len(servers))

	for _, server := range servers {
		host := dnsServerHostname(server)
		if host == "" {
			continue
		}

		if _, err := netip.ParseAddr(host); err == nil {
			if _, ok := seenIPs[host]; ok {
				continue
			}
			seenIPs[host] = struct{}{}
			ips = append(ips, host)
			continue
		}

		domain := "full:" + host
		if _, ok := seenDomains[domain]; ok {
			continue
		}
		seenDomains[domain] = struct{}{}
		domains = append(domains, domain)
	}

	return ips, domains
}

func hasScheme(value string) bool {
	return strings.Contains(value, "://")
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
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

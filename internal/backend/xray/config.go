package xray

type xrayConfig struct {
	Log       xrayLog       `json:"log"`
	DNS       *xrayDNS      `json:"dns,omitempty"`
	Inbounds  []xrayInbound `json:"inbounds"`
	Outbounds []any         `json:"outbounds"`
	Routing   xrayRouting   `json:"routing"`
}

type xrayLog struct {
	LogLevel string `json:"loglevel"`
}

type xrayDNS struct {
	Servers []any `json:"servers,omitempty"`
}

type xrayDNSServer struct {
	Address      string   `json:"address"`
	Domains      []string `json:"domains,omitempty"`
	SkipFallback bool     `json:"skipFallback,omitempty"`
}

type xrayInbound struct {
	Tag            string `json:"tag"`
	Listen         string `json:"listen"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`
	Settings       any    `json:"settings"`
	Sniffing       any    `json:"sniffing,omitempty"`
	StreamSettings any    `json:"streamSettings,omitempty"`
}

type xrayRouting struct {
	DomainStrategy string          `json:"domainStrategy"`
	Rules          []xrayRouteRule `json:"rules"`
}

type xrayRouteRule struct {
	Type        string   `json:"type"`
	OutboundTag string   `json:"outboundTag"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	Network     string   `json:"network,omitempty"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
}

type xrayCommonOutbound struct {
	Tag            string `json:"tag"`
	Protocol       string `json:"protocol"`
	Settings       any    `json:"settings,omitempty"`
	StreamSettings any    `json:"streamSettings,omitempty"`
}

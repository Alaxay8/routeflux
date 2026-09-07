package domain

import "fmt"

// ProxySettings configures explicit SOCKS5 and HTTP proxy listeners.
type ProxySettings struct {
	AllowLAN  bool `json:"allow_lan"`
	SOCKSPort int  `json:"socks_port"`
	HTTPPort  int  `json:"http_port"`
}

// DefaultProxySettings keeps explicit proxies private until LAN access is enabled.
func DefaultProxySettings() ProxySettings {
	return ProxySettings{SOCKSPort: 10808, HTTPPort: 10809}
}

// Validate checks listener ports against each other and active runtime listeners.
// A zero DNS or transparent port means that listener is disabled.
func (p ProxySettings) Validate(dnsPort, transparentPort int) error {
	for _, port := range []int{p.SOCKSPort, p.HTTPPort} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("proxy port must be between 1 and 65535: %d", port)
		}
		if port == dnsPort {
			return fmt.Errorf("proxy port %d conflicts with local DNS", port)
		}
		if port == transparentPort {
			return fmt.Errorf("proxy port %d conflicts with transparent routing", port)
		}
	}
	if p.SOCKSPort == p.HTTPPort {
		return fmt.Errorf("SOCKS5 and HTTP ports must be different")
	}
	return nil
}

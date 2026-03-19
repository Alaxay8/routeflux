package parser

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// ParseVLESS parses a VLESS link into a normalized node.
func ParseVLESS(raw, provider string) (domain.Node, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse vless link: %w", err)
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse vless port: %w", err)
	}

	query := parsed.Query()
	node := domain.Node{
		Name:        parsed.Fragment,
		Remark:      parsed.Fragment,
		Protocol:    domain.ProtocolVLESS,
		Address:     parsed.Hostname(),
		Port:        port,
		UUID:        parsed.User.Username(),
		Encryption:  query.Get("encryption"),
		Security:    query.Get("security"),
		ServerName:  firstNonEmpty(query.Get("sni"), query.Get("serverName")),
		ALPN:        splitCSV(query.Get("alpn")),
		Fingerprint: query.Get("fp"),
		PublicKey:   firstNonEmpty(query.Get("pbk"), query.Get("publicKey")),
		ShortID:     firstNonEmpty(query.Get("sid"), query.Get("shortId")),
		Flow:        query.Get("flow"),
		Transport:   firstNonEmpty(query.Get("type"), query.Get("network")),
		Path:        query.Get("path"),
		Host:        query.Get("host"),
		RawQuery:    parsed.RawQuery,
		Extras: map[string]string{
			"type": firstNonEmpty(query.Get("type"), query.Get("network")),
		},
	}

	return normalizeNode(node, provider)
}

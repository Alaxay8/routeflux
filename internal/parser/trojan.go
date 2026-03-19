package parser

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

// ParseTrojan parses a Trojan share link into a normalized node.
func ParseTrojan(raw, provider string) (domain.Node, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse trojan link: %w", err)
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse trojan port: %w", err)
	}

	query := parsed.Query()
	node := domain.Node{
		Name:       parsed.Fragment,
		Remark:     parsed.Fragment,
		Protocol:   domain.ProtocolTrojan,
		Address:    parsed.Hostname(),
		Port:       port,
		Password:   passwordFromURL(parsed),
		Security:   query.Get("security"),
		ServerName: firstNonEmpty(query.Get("sni"), query.Get("serverName")),
		ALPN:       splitCSV(query.Get("alpn")),
		Transport:  firstNonEmpty(query.Get("type"), query.Get("network")),
		Path:       query.Get("path"),
		Host:       query.Get("host"),
		RawQuery:   parsed.RawQuery,
		Extras: map[string]string{
			"type": firstNonEmpty(query.Get("type"), query.Get("network")),
		},
	}

	return normalizeNode(node, provider)
}

// ParseShadowsocks parses common SS share links.
func ParseShadowsocks(raw, provider string) (domain.Node, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "ss://"))
	fragment := ""
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		fragment = trimmed[idx+1:]
		trimmed = trimmed[:idx]
	}

	decoded, err := decodeShadowsocksUserInfo(trimmed)
	if err != nil {
		return domain.Node{}, err
	}

	parts := strings.Split(decoded, "@")
	if len(parts) != 2 {
		return domain.Node{}, fmt.Errorf("invalid shadowsocks payload")
	}

	credParts := strings.SplitN(parts[0], ":", 2)
	if len(credParts) != 2 {
		return domain.Node{}, fmt.Errorf("invalid shadowsocks credentials")
	}

	host, portText, ok := strings.Cut(parts[1], ":")
	if !ok {
		return domain.Node{}, fmt.Errorf("invalid shadowsocks host")
	}

	port, err := strconv.Atoi(portText)
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse shadowsocks port: %w", err)
	}

	node := domain.Node{
		Name:       fragment,
		Remark:     fragment,
		Protocol:   domain.ProtocolShadowsocks,
		Address:    host,
		Port:       port,
		Password:   credParts[1],
		Encryption: credParts[0],
	}

	return normalizeNode(node, provider)
}

func decodeShadowsocksUserInfo(input string) (string, error) {
	if strings.Contains(input, "@") && strings.Contains(input, ":") {
		return input, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(input)
	if err == nil {
		return string(payload), nil
	}

	payload, err = base64.StdEncoding.DecodeString(input)
	if err == nil {
		return string(payload), nil
	}

	return "", fmt.Errorf("decode shadowsocks payload: %w", err)
}

func passwordFromURL(parsed *url.URL) string {
	if parsed.User == nil {
		return ""
	}

	if password, ok := parsed.User.Password(); ok {
		return password
	}

	return parsed.User.Username()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

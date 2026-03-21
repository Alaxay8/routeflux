package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

var errUnsupportedJSONOutbound = errors.New("unsupported json outbound")

type xrayOutboundJSON struct {
	Protocol       string          `json:"protocol"`
	Tag            string          `json:"tag"`
	Remark         string          `json:"remark"`
	Name           string          `json:"ps"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings"`
}

type xrayVNextSettings struct {
	VNext []struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
		Users   []struct {
			ID         string `json:"id"`
			Encryption string `json:"encryption"`
			Flow       string `json:"flow"`
			Security   string `json:"security"`
		} `json:"users"`
	} `json:"vnext"`
}

type xrayServerSettings struct {
	Servers []struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Password string `json:"password"`
		Method   string `json:"method"`
	} `json:"servers"`
}

type xrayStreamSettings struct {
	Network     string `json:"network"`
	Security    string `json:"security"`
	TLSSettings struct {
		ServerName  string   `json:"serverName"`
		ALPN        []string `json:"alpn"`
		Fingerprint string   `json:"fingerprint"`
	} `json:"tlsSettings"`
	RealitySettings struct {
		ServerName  string `json:"serverName"`
		PublicKey   string `json:"publicKey"`
		ShortID     string `json:"shortId"`
		Fingerprint string `json:"fingerprint"`
	} `json:"realitySettings"`
	WSSettings struct {
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
	} `json:"wsSettings"`
	GRPCSettings struct {
		ServiceName string `json:"serviceName"`
	} `json:"grpcSettings"`
}

func tryParseJSONNodes(input, provider string) ([]domain.Node, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, false, nil
	}
	if !(strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		return nil, false, nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, false, nil
	}

	nodes, err := parseJSONImport(json.RawMessage(trimmed), provider)
	if err != nil {
		return nil, true, err
	}

	return nodes, true, nil
}

func parseJSONImport(raw json.RawMessage, provider string) ([]domain.Node, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty json import")
	}

	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
			return nil, fmt.Errorf("decode json object: %w", err)
		}

		if outbounds, ok := object["outbounds"]; ok {
			return parseOutboundList(outbounds, provider)
		}
		if protocol, ok := object["protocol"]; ok && len(protocol) > 0 {
			node, err := parseJSONOutbound([]byte(trimmed), provider)
			if err != nil {
				return nil, err
			}
			return []domain.Node{node}, nil
		}
		if rawConfig, ok := object["config"]; ok {
			var text string
			if err := json.Unmarshal(rawConfig, &text); err == nil {
				return ParseNodes(text, provider)
			}
			return parseJSONImport(rawConfig, provider)
		}
		if rawLink, ok := object["link"]; ok {
			var link string
			if err := json.Unmarshal(rawLink, &link); err == nil {
				return ParseNodes(link, provider)
			}
		}

		return nil, fmt.Errorf("unsupported json import format")
	case '[':
		return parseOutboundList([]byte(trimmed), provider)
	default:
		return nil, fmt.Errorf("unsupported json import format")
	}
}

func parseOutboundList(raw json.RawMessage, provider string) ([]domain.Node, error) {
	var outbounds []json.RawMessage
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return nil, fmt.Errorf("decode json outbounds: %w", err)
	}

	nodes := make([]domain.Node, 0, len(outbounds))
	for _, outbound := range outbounds {
		node, err := parseJSONOutbound(outbound, provider)
		if err != nil {
			if errors.Is(err, errUnsupportedJSONOutbound) {
				continue
			}
			return nil, err
		}
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no supported nodes found")
	}

	return nodes, nil
}

func parseJSONOutbound(raw json.RawMessage, provider string) (domain.Node, error) {
	var outbound xrayOutboundJSON
	if err := json.Unmarshal(raw, &outbound); err != nil {
		return domain.Node{}, fmt.Errorf("decode outbound: %w", err)
	}

	var stream xrayStreamSettings
	if len(outbound.StreamSettings) > 0 {
		if err := json.Unmarshal(outbound.StreamSettings, &stream); err != nil {
			return domain.Node{}, fmt.Errorf("decode stream settings: %w", err)
		}
	}

	name := firstNonEmpty(outbound.Tag, outbound.Remark, outbound.Name)
	extras := map[string]string{}
	if outbound.Tag != "" {
		extras["tag"] = outbound.Tag
	}
	if stream.Network != "" {
		extras["type"] = stream.Network
	}

	switch strings.ToLower(outbound.Protocol) {
	case "vless":
		var settings xrayVNextSettings
		if err := json.Unmarshal(outbound.Settings, &settings); err != nil {
			return domain.Node{}, fmt.Errorf("decode vless settings: %w", err)
		}
		if len(settings.VNext) == 0 || len(settings.VNext[0].Users) == 0 {
			return domain.Node{}, fmt.Errorf("invalid vless settings")
		}
		user := settings.VNext[0].Users[0]
		node := domain.Node{
			Name:        name,
			Remark:      name,
			Protocol:    domain.ProtocolVLESS,
			Address:     settings.VNext[0].Address,
			Port:        settings.VNext[0].Port,
			UUID:        user.ID,
			Encryption:  firstNonEmpty(user.Encryption, "none"),
			Security:    stream.Security,
			ServerName:  firstNonEmpty(stream.RealitySettings.ServerName, stream.TLSSettings.ServerName),
			ALPN:        stream.TLSSettings.ALPN,
			Fingerprint: firstNonEmpty(stream.RealitySettings.Fingerprint, stream.TLSSettings.Fingerprint),
			PublicKey:   stream.RealitySettings.PublicKey,
			ShortID:     stream.RealitySettings.ShortID,
			Flow:        user.Flow,
			Transport:   firstNonEmpty(stream.Network, "tcp"),
			Path:        firstNonEmpty(stream.WSSettings.Path, stream.GRPCSettings.ServiceName),
			Host:        stream.WSSettings.Headers["Host"],
			Extras:      extras,
		}
		return normalizeNode(node, provider)
	case "vmess":
		var settings xrayVNextSettings
		if err := json.Unmarshal(outbound.Settings, &settings); err != nil {
			return domain.Node{}, fmt.Errorf("decode vmess settings: %w", err)
		}
		if len(settings.VNext) == 0 || len(settings.VNext[0].Users) == 0 {
			return domain.Node{}, fmt.Errorf("invalid vmess settings")
		}
		user := settings.VNext[0].Users[0]
		node := domain.Node{
			Name:        name,
			Remark:      name,
			Protocol:    domain.ProtocolVMess,
			Address:     settings.VNext[0].Address,
			Port:        settings.VNext[0].Port,
			UUID:        user.ID,
			Encryption:  firstNonEmpty(user.Security, "auto"),
			Security:    stream.Security,
			ServerName:  firstNonEmpty(stream.RealitySettings.ServerName, stream.TLSSettings.ServerName),
			ALPN:        stream.TLSSettings.ALPN,
			Fingerprint: firstNonEmpty(stream.RealitySettings.Fingerprint, stream.TLSSettings.Fingerprint),
			PublicKey:   stream.RealitySettings.PublicKey,
			ShortID:     stream.RealitySettings.ShortID,
			Transport:   firstNonEmpty(stream.Network, "tcp"),
			Path:        firstNonEmpty(stream.WSSettings.Path, stream.GRPCSettings.ServiceName),
			Host:        stream.WSSettings.Headers["Host"],
			Extras:      extras,
		}
		return normalizeNode(node, provider)
	case "trojan":
		var settings xrayServerSettings
		if err := json.Unmarshal(outbound.Settings, &settings); err != nil {
			return domain.Node{}, fmt.Errorf("decode trojan settings: %w", err)
		}
		if len(settings.Servers) == 0 {
			return domain.Node{}, fmt.Errorf("invalid trojan settings")
		}
		server := settings.Servers[0]
		node := domain.Node{
			Name:        name,
			Remark:      name,
			Protocol:    domain.ProtocolTrojan,
			Address:     server.Address,
			Port:        server.Port,
			Password:    server.Password,
			Encryption:  server.Method,
			Security:    stream.Security,
			ServerName:  firstNonEmpty(stream.RealitySettings.ServerName, stream.TLSSettings.ServerName),
			ALPN:        stream.TLSSettings.ALPN,
			Fingerprint: firstNonEmpty(stream.RealitySettings.Fingerprint, stream.TLSSettings.Fingerprint),
			PublicKey:   stream.RealitySettings.PublicKey,
			ShortID:     stream.RealitySettings.ShortID,
			Transport:   firstNonEmpty(stream.Network, "tcp"),
			Path:        firstNonEmpty(stream.WSSettings.Path, stream.GRPCSettings.ServiceName),
			Host:        stream.WSSettings.Headers["Host"],
			Extras:      extras,
		}
		return normalizeNode(node, provider)
	default:
		return domain.Node{}, errUnsupportedJSONOutbound
	}
}

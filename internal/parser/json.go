package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

var errUnsupportedJSONOutbound = errors.New("unsupported json outbound")
var errUnsupportedJSONImport = errors.New("unsupported json import")

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

type xrayDirectEndpointSettings struct {
	Address    string `json:"address"`
	Port       int    `json:"port"`
	ID         string `json:"id"`
	UUID       string `json:"uuid"`
	Password   string `json:"password"`
	Method     string `json:"method"`
	Encryption string `json:"encryption"`
	Flow       string `json:"flow"`
	Security   string `json:"security"`
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

type xrayJSONImportMetadata struct {
	Remarks string `json:"remarks"`
	Remark  string `json:"remark"`
	Name    string `json:"name"`
	PS      string `json:"ps"`
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
	return parseJSONImportWithLabel(raw, provider, "")
}

func parseJSONImportWithLabel(raw json.RawMessage, provider, defaultLabel string) ([]domain.Node, error) {
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
			label, err := parseJSONImportLabel([]byte(trimmed))
			if err != nil {
				return nil, err
			}
			return parseOutboundList(outbounds, provider, firstNonEmpty(label, defaultLabel))
		}
		if protocol, ok := object["protocol"]; ok && len(protocol) > 0 {
			label, err := parseJSONImportLabel([]byte(trimmed))
			if err != nil {
				return nil, err
			}
			node, err := parseJSONOutbound([]byte(trimmed), provider, firstNonEmpty(label, defaultLabel))
			if err != nil {
				return nil, err
			}
			return []domain.Node{node}, nil
		}
		if rawConfig, ok := object["config"]; ok {
			label, err := parseJSONImportLabel([]byte(trimmed))
			if err != nil {
				return nil, err
			}
			fallbackLabel := firstNonEmpty(label, defaultLabel)

			var text string
			if err := json.Unmarshal(rawConfig, &text); err == nil {
				nodes, err := ParseNodes(text, provider)
				if err != nil {
					return nil, err
				}
				return applyFallbackNodeLabel(nodes, fallbackLabel), nil
			}
			return parseJSONImportWithLabel(rawConfig, provider, fallbackLabel)
		}
		if rawLink, ok := object["link"]; ok {
			label, err := parseJSONImportLabel([]byte(trimmed))
			if err != nil {
				return nil, err
			}

			var link string
			if err := json.Unmarshal(rawLink, &link); err == nil {
				nodes, err := ParseNodes(link, provider)
				if err != nil {
					return nil, err
				}
				return applyFallbackNodeLabel(nodes, firstNonEmpty(label, defaultLabel)), nil
			}
		}

		return nil, errUnsupportedJSONImport
	case '[':
		return parseJSONList([]byte(trimmed), provider)
	default:
		return nil, errUnsupportedJSONImport
	}
}

func parseJSONList(raw json.RawMessage, provider string) ([]domain.Node, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode json list: %w", err)
	}

	nodes := make([]domain.Node, 0, len(items))
	for _, item := range items {
		parsed, err := parseJSONImport(item, provider)
		if err != nil {
			if errors.Is(err, errUnsupportedJSONImport) || errors.Is(err, errUnsupportedJSONOutbound) {
				continue
			}
			if strings.Contains(err.Error(), "no supported nodes found") {
				continue
			}
			return nil, err
		}
		nodes = append(nodes, parsed...)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no supported nodes found")
	}

	return nodes, nil
}

func parseOutboundList(raw json.RawMessage, provider, defaultLabel string) ([]domain.Node, error) {
	var outbounds []json.RawMessage
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return nil, fmt.Errorf("decode json outbounds: %w", err)
	}

	nodes := make([]domain.Node, 0, len(outbounds))
	for _, outbound := range outbounds {
		node, err := parseJSONOutbound(outbound, provider, defaultLabel)
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

func parseJSONImportLabel(raw json.RawMessage) (string, error) {
	var metadata xrayJSONImportMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "", fmt.Errorf("decode json metadata: %w", err)
	}

	return firstNonEmpty(metadata.Remarks, metadata.Remark, metadata.Name, metadata.PS), nil
}

func applyFallbackNodeLabel(nodes []domain.Node, label string) []domain.Node {
	label = strings.TrimSpace(label)
	if label == "" {
		return nodes
	}

	for idx := range nodes {
		nodes[idx].Remark = label
		nodes[idx].Name = label
		nodes[idx].ID = domain.StableNodeID(nodes[idx])
	}

	return nodes
}

func parseJSONOutbound(raw json.RawMessage, provider, defaultLabel string) (domain.Node, error) {
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

	name := firstNonEmpty(defaultLabel, outbound.Remark, outbound.Name, outbound.Tag)
	extras := map[string]string{}
	if outbound.Tag != "" {
		extras["tag"] = outbound.Tag
	}
	if stream.Network != "" {
		extras["type"] = stream.Network
	}

	switch strings.ToLower(outbound.Protocol) {
	case "vless":
		address, port, user, err := parseJSONVNextEndpoint(outbound.Settings)
		if err != nil {
			return domain.Node{}, fmt.Errorf("decode vless settings: %w", err)
		}
		if address == "" || port <= 0 || firstNonEmpty(user.ID, user.UUID) == "" {
			return domain.Node{}, fmt.Errorf("invalid vless settings")
		}
		node := domain.Node{
			Name:        name,
			Remark:      name,
			Protocol:    domain.ProtocolVLESS,
			Address:     address,
			Port:        port,
			UUID:        firstNonEmpty(user.ID, user.UUID),
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
		address, port, user, err := parseJSONVNextEndpoint(outbound.Settings)
		if err != nil {
			return domain.Node{}, fmt.Errorf("decode vmess settings: %w", err)
		}
		if address == "" || port <= 0 || firstNonEmpty(user.ID, user.UUID) == "" {
			return domain.Node{}, fmt.Errorf("invalid vmess settings")
		}
		node := domain.Node{
			Name:        name,
			Remark:      name,
			Protocol:    domain.ProtocolVMess,
			Address:     address,
			Port:        port,
			UUID:        firstNonEmpty(user.ID, user.UUID),
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
		server, err := parseJSONTrojanServer(outbound.Settings)
		if err != nil {
			return domain.Node{}, fmt.Errorf("decode trojan settings: %w", err)
		}
		if server.Address == "" || server.Port <= 0 || server.Password == "" {
			return domain.Node{}, fmt.Errorf("invalid trojan settings")
		}
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

func parseJSONVNextEndpoint(raw json.RawMessage) (string, int, xrayDirectEndpointSettings, error) {
	var settings xrayVNextSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return "", 0, xrayDirectEndpointSettings{}, err
	}
	if len(settings.VNext) > 0 && len(settings.VNext[0].Users) > 0 {
		return settings.VNext[0].Address, settings.VNext[0].Port, xrayDirectEndpointSettings{
			ID:         settings.VNext[0].Users[0].ID,
			Encryption: settings.VNext[0].Users[0].Encryption,
			Flow:       settings.VNext[0].Users[0].Flow,
			Security:   settings.VNext[0].Users[0].Security,
		}, nil
	}

	var direct xrayDirectEndpointSettings
	if err := json.Unmarshal(raw, &direct); err != nil {
		return "", 0, xrayDirectEndpointSettings{}, err
	}

	return direct.Address, direct.Port, direct, nil
}

func parseJSONTrojanServer(raw json.RawMessage) (xrayDirectEndpointSettings, error) {
	var settings xrayServerSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return xrayDirectEndpointSettings{}, err
	}
	if len(settings.Servers) > 0 {
		return xrayDirectEndpointSettings{
			Address:  settings.Servers[0].Address,
			Port:     settings.Servers[0].Port,
			Password: settings.Servers[0].Password,
			Method:   settings.Servers[0].Method,
		}, nil
	}

	var direct xrayDirectEndpointSettings
	if err := json.Unmarshal(raw, &direct); err != nil {
		return xrayDirectEndpointSettings{}, err
	}

	return direct, nil
}

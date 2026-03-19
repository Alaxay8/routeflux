package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

type vmessPayload struct {
	Version     string `json:"v"`
	Name        string `json:"ps"`
	Address     string `json:"add"`
	Port        string `json:"port"`
	ID          string `json:"id"`
	AlterID     string `json:"aid"`
	Security    string `json:"scy"`
	Network     string `json:"net"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	TLS         string `json:"tls"`
	ServerName  string `json:"sni"`
	ALPN        string `json:"alpn"`
	Fingerprint string `json:"fp"`
	Flow        string `json:"flow"`
}

// ParseVMess parses a VMess base64 JSON link.
func ParseVMess(raw, provider string) (domain.Node, error) {
	encoded := strings.TrimPrefix(strings.TrimSpace(raw), "vmess://")
	payload, err := decodeVMessPayload(encoded)
	if err != nil {
		return domain.Node{}, err
	}

	port, err := strconv.Atoi(payload.Port)
	if err != nil {
		return domain.Node{}, fmt.Errorf("parse vmess port: %w", err)
	}

	node := domain.Node{
		Name:        payload.Name,
		Remark:      payload.Name,
		Protocol:    domain.ProtocolVMess,
		Address:     payload.Address,
		Port:        port,
		UUID:        payload.ID,
		Encryption:  payload.Security,
		Security:    payload.TLS,
		ServerName:  payload.ServerName,
		ALPN:        splitCSV(payload.ALPN),
		Fingerprint: payload.Fingerprint,
		Flow:        payload.Flow,
		Transport:   payload.Network,
		Path:        payload.Path,
		Host:        payload.Host,
		Extras: map[string]string{
			"aid":  payload.AlterID,
			"type": payload.Type,
			"v":    payload.Version,
		},
	}

	return normalizeNode(node, provider)
}

func decodeVMessPayload(encoded string) (vmessPayload, error) {
	encoded = strings.TrimSpace(encoded)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return vmessPayload{}, fmt.Errorf("decode vmess payload: %w", err)
		}
	}

	var payload vmessPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return vmessPayload{}, fmt.Errorf("decode vmess json: %w", err)
	}

	return payload, nil
}

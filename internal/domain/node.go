package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Protocol identifies a supported outbound protocol.
type Protocol string

const (
	// ProtocolVLESS represents a VLESS node.
	ProtocolVLESS Protocol = "vless"
	// ProtocolVMess represents a VMess node.
	ProtocolVMess Protocol = "vmess"
	// ProtocolTrojan represents a Trojan node.
	ProtocolTrojan Protocol = "trojan"
	// ProtocolShadowsocks represents a Shadowsocks node.
	ProtocolShadowsocks Protocol = "shadowsocks"
)

// Node is the normalized representation of a provider endpoint.
type Node struct {
	ID             string            `json:"id"`
	SubscriptionID string            `json:"subscription_id"`
	Name           string            `json:"name"`
	ProviderName   string            `json:"provider_name"`
	Protocol       Protocol          `json:"protocol"`
	Address        string            `json:"address"`
	Port           int               `json:"port"`
	UUID           string            `json:"uuid"`
	Password       string            `json:"password"`
	Encryption     string            `json:"encryption"`
	Security       string            `json:"security"`
	ServerName     string            `json:"server_name"`
	ALPN           []string          `json:"alpn"`
	Fingerprint    string            `json:"fingerprint"`
	PublicKey      string            `json:"public_key"`
	ShortID        string            `json:"short_id"`
	Flow           string            `json:"flow"`
	Transport      string            `json:"transport"`
	Path           string            `json:"path"`
	Host           string            `json:"host"`
	Remark         string            `json:"remark"`
	RawQuery       string            `json:"raw_query"`
	Extras         map[string]string `json:"extras"`
}

// StableNodeID derives a deterministic ID from the node identity.
func StableNodeID(node Node) string {
	key := strings.Join([]string{
		string(node.Protocol),
		node.Address,
		strconv.Itoa(node.Port),
		node.UUID,
		node.Password,
		node.Name,
		node.ServerName,
		node.Transport,
	}, "|")

	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])[:12]
}

// DisplayName returns a user-facing node label.
func (n Node) DisplayName() string {
	if n.Name != "" {
		return n.Name
	}

	if n.Remark != "" {
		return n.Remark
	}

	return fmt.Sprintf("%s:%d", n.Address, n.Port)
}

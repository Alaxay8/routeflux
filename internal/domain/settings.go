package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration wraps time.Duration with string JSON encoding.
type Duration time.Duration

// NewDuration converts a time.Duration into a serializable Duration.
func NewDuration(value time.Duration) Duration {
	return Duration(value)
}

// Duration returns the stdlib time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String formats the duration using Go duration syntax.
func (d Duration) String() string {
	return d.Duration().String()
}

// MarshalJSON encodes the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON decodes either a duration string or a raw integer.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		parsed, err := time.ParseDuration(asString)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", asString, err)
		}
		*d = NewDuration(parsed)
		return nil
	}

	var asNumber int64
	if err := json.Unmarshal(data, &asNumber); err != nil {
		return fmt.Errorf("parse duration value: %w", err)
	}

	*d = Duration(asNumber)
	return nil
}

// SelectionMode defines how RouteFlux selects the active node.
type SelectionMode string

const (
	// SelectionModeManual pins a user-selected node.
	SelectionModeManual SelectionMode = "manual"
	// SelectionModeAuto automatically selects the best node.
	SelectionModeAuto SelectionMode = "auto"
	// SelectionModeDisconnected means no node is active.
	SelectionModeDisconnected SelectionMode = "disconnected"
)

// Settings stores user-configurable application behavior.
type Settings struct {
	SchemaVersion       int              `json:"schema_version"`
	RefreshInterval     Duration         `json:"refresh_interval"`
	HealthCheckInterval Duration         `json:"health_check_interval"`
	SwitchCooldown      Duration         `json:"switch_cooldown"`
	LatencyThreshold    Duration         `json:"latency_threshold"`
	DNS                 DNSSettings      `json:"dns"`
	Firewall            FirewallSettings `json:"firewall"`
	AutoMode            bool             `json:"auto_mode"`
	Mode                SelectionMode    `json:"mode"`
	LogLevel            string           `json:"log_level"`
}

// DNSMode controls how RouteFlux manages runtime DNS behavior.
type DNSMode string

const (
	// DNSModeSystem leaves DNS handling to the router or host system.
	DNSModeSystem DNSMode = "system"
	// DNSModeRemote forces DNS queries to configured upstream servers.
	DNSModeRemote DNSMode = "remote"
	// DNSModeSplit keeps selected domains on system DNS and sends the rest upstream.
	DNSModeSplit DNSMode = "split"
	// DNSModeDisabled omits RouteFlux-managed DNS config.
	DNSModeDisabled DNSMode = "disabled"
)

// DNSTransport controls how RouteFlux talks to upstream DNS servers.
type DNSTransport string

const (
	// DNSTransportPlain uses plain DNS as written by the server address.
	DNSTransportPlain DNSTransport = "plain"
	// DNSTransportDoH uses DNS over HTTPS.
	DNSTransportDoH DNSTransport = "doh"
	// DNSTransportDoT is reserved for future backends.
	DNSTransportDoT DNSTransport = "dot"
)

// DNSSettings stores RouteFlux-managed DNS preferences.
type DNSSettings struct {
	Mode          DNSMode      `json:"mode"`
	Transport     DNSTransport `json:"transport"`
	Servers       []string     `json:"servers"`
	Bootstrap     []string     `json:"bootstrap"`
	DirectDomains []string     `json:"direct_domains"`
}

// FirewallTargetMode controls what happens when a transparent selector matches.
type FirewallTargetMode string

const (
	// FirewallTargetModeProxy sends matched targets through the selected proxy node.
	FirewallTargetModeProxy FirewallTargetMode = "proxy"
	// FirewallTargetModeBypass keeps matched targets direct while the rest uses the proxy.
	FirewallTargetModeBypass FirewallTargetMode = "bypass"
)

// FirewallModeDraft stores saved selectors for one LuCI firewall mode.
type FirewallModeDraft struct {
	TargetServices []string `json:"target_services"`
	TargetCIDRs    []string `json:"target_cidrs"`
	TargetDomains  []string `json:"target_domains"`
	SourceCIDRs    []string `json:"source_cidrs"`
}

// FirewallModeDrafts stores saved selectors for each supported LuCI firewall mode.
type FirewallModeDrafts struct {
	Hosts      FirewallModeDraft `json:"hosts"`
	Targets    FirewallModeDraft `json:"targets"`
	AntiTarget FirewallModeDraft `json:"anti_target"`
}

// FirewallSettings stores transparent proxy routing preferences.
type FirewallSettings struct {
	Enabled              bool                                `json:"enabled"`
	TransparentPort      int                                 `json:"transparent_port"`
	TargetMode           FirewallTargetMode                  `json:"target_mode"`
	TargetServices       []string                            `json:"target_services"`
	TargetServiceCatalog map[string]FirewallTargetDefinition `json:"target_service_catalog"`
	TargetCIDRs          []string                            `json:"target_cidrs"`
	TargetDomains        []string                            `json:"target_domains"`
	SourceCIDRs          []string                            `json:"source_cidrs"`
	ModeDrafts           FirewallModeDrafts                  `json:"mode_drafts"`
	BlockQUIC            bool                                `json:"block_quic"`
}

// DefaultSettings returns the baseline configuration used on first start.
func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:       7,
		RefreshInterval:     NewDuration(time.Hour),
		HealthCheckInterval: NewDuration(30 * time.Second),
		SwitchCooldown:      NewDuration(5 * time.Minute),
		LatencyThreshold:    NewDuration(50 * time.Millisecond),
		DNS:                 DefaultDNSSettings(),
		Firewall: FirewallSettings{
			Enabled:              false,
			TransparentPort:      12345,
			TargetMode:           FirewallTargetModeProxy,
			TargetServices:       nil,
			TargetServiceCatalog: nil,
			TargetCIDRs:          nil,
			TargetDomains:        nil,
			SourceCIDRs:          nil,
			ModeDrafts:           FirewallModeDrafts{},
			BlockQUIC:            false,
		},
		AutoMode: false,
		Mode:     SelectionModeManual,
		LogLevel: "info",
	}
}

// DefaultDNSSettings returns the recommended DNS profile for RouteFlux.
func DefaultDNSSettings() DNSSettings {
	return DNSSettings{
		Mode:          DNSModeSplit,
		Transport:     DNSTransportDoH,
		Servers:       []string{"1.1.1.1", "1.0.0.1"},
		Bootstrap:     nil,
		DirectDomains: []string{"domain:lan", "full:router.lan"},
	}
}

// NormalizeFirewallTargetMode coerces unknown values to the default proxy behavior.
func NormalizeFirewallTargetMode(mode FirewallTargetMode) FirewallTargetMode {
	switch mode {
	case FirewallTargetModeBypass:
		return FirewallTargetModeBypass
	default:
		return FirewallTargetModeProxy
	}
}

// EffectiveTransparentBlockQUIC returns the runtime QUIC policy for transparent mode.
//
// Some outbound types, notably VLESS over TCP Reality/XTLS Vision, reject UDP/443 in
// Xray. For those nodes we block QUIC and rely on TCP fallback even when the user has
// not explicitly enabled block_quic.
func EffectiveTransparentBlockQUIC(settings FirewallSettings, node *Node) bool {
	if settings.BlockQUIC {
		return true
	}

	return nodeRequiresTransparentQUICBlock(node)
}

func nodeRequiresTransparentQUICBlock(node *Node) bool {
	if node == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(node.Transport), "tcp") {
		return false
	}
	if node.Protocol != ProtocolVLESS {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(node.Flow), "xtls-rprx-vision") {
		return true
	}

	return strings.EqualFold(strings.TrimSpace(node.Security), "reality")
}

// CloneFirewallModeDraft deep-copies one firewall mode draft.
func CloneFirewallModeDraft(draft FirewallModeDraft) FirewallModeDraft {
	return FirewallModeDraft{
		TargetServices: append([]string(nil), draft.TargetServices...),
		TargetCIDRs:    append([]string(nil), draft.TargetCIDRs...),
		TargetDomains:  append([]string(nil), draft.TargetDomains...),
		SourceCIDRs:    append([]string(nil), draft.SourceCIDRs...),
	}
}

// CloneFirewallModeDrafts deep-copies all firewall mode drafts.
func CloneFirewallModeDrafts(drafts FirewallModeDrafts) FirewallModeDrafts {
	return FirewallModeDrafts{
		Hosts:      CloneFirewallModeDraft(drafts.Hosts),
		Targets:    CloneFirewallModeDraft(drafts.Targets),
		AntiTarget: CloneFirewallModeDraft(drafts.AntiTarget),
	}
}

// ParseDurationValue accepts either a Go duration string or an integer nanosecond value.
func ParseDurationValue(raw string) (Duration, error) {
	if parsed, err := time.ParseDuration(raw); err == nil {
		return NewDuration(parsed), nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", raw, err)
	}

	return Duration(value), nil
}

// ParseDNSMode validates and normalizes a DNS mode value.
func ParseDNSMode(raw string) (DNSMode, error) {
	switch DNSMode(strings.ToLower(strings.TrimSpace(raw))) {
	case "":
		return DNSModeSystem, nil
	case DNSModeSystem, DNSModeRemote, DNSModeSplit, DNSModeDisabled:
		return DNSMode(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", fmt.Errorf("unsupported dns mode %q", raw)
	}
}

// ParseDNSTransport validates and normalizes a DNS transport value.
func ParseDNSTransport(raw string) (DNSTransport, error) {
	switch DNSTransport(strings.ToLower(strings.TrimSpace(raw))) {
	case "":
		return DNSTransportPlain, nil
	case DNSTransportPlain, DNSTransportDoH, DNSTransportDoT:
		return DNSTransport(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", fmt.Errorf("unsupported dns transport %q", raw)
	}
}

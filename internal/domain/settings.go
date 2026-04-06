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

// FirewallMode controls the active transparent-routing strategy.
type FirewallMode string

const (
	// FirewallModeDisabled turns off transparent routing.
	FirewallModeDisabled FirewallMode = "disabled"
	// FirewallModeHosts proxies all traffic from selected LAN clients.
	FirewallModeHosts FirewallMode = "hosts"
	// FirewallModeTargets proxies only selected destinations.
	FirewallModeTargets FirewallMode = "targets"
	// FirewallModeSplit applies an explicit proxy/bypass policy.
	FirewallModeSplit FirewallMode = "split"
)

// FirewallDefaultAction controls what happens when no split selector matches.
type FirewallDefaultAction string

const (
	// FirewallDefaultActionDirect keeps unmatched split traffic direct.
	FirewallDefaultActionDirect FirewallDefaultAction = "direct"
	// FirewallDefaultActionProxy sends unmatched split traffic through the proxy.
	FirewallDefaultActionProxy FirewallDefaultAction = "proxy"
)

// FirewallTargetMode is kept for legacy anti-target migration and CLI compatibility.
type FirewallTargetMode string

const (
	// FirewallTargetModeProxy maps legacy target mode to explicit proxy selectors.
	FirewallTargetModeProxy FirewallTargetMode = "proxy"
	// FirewallTargetModeBypass maps legacy target mode to explicit bypass selectors.
	FirewallTargetModeBypass FirewallTargetMode = "bypass"
)

// FirewallSelectorSet stores service aliases, domains, and IPv4 selectors.
type FirewallSelectorSet struct {
	Services []string `json:"services"`
	Domains  []string `json:"domains"`
	CIDRs    []string `json:"cidrs"`
}

// FirewallSplitSettings stores the full split tunnelling policy.
type FirewallSplitSettings struct {
	Proxy           FirewallSelectorSet   `json:"proxy"`
	Bypass          FirewallSelectorSet   `json:"bypass"`
	ExcludedSources []string              `json:"excluded_sources"`
	DefaultAction   FirewallDefaultAction `json:"default_action"`
}

// FirewallModeDraft stores saved selectors for one LuCI firewall mode.
type FirewallModeDraft struct {
	TargetServices []string `json:"target_services"`
	TargetCIDRs    []string `json:"target_cidrs"`
	TargetDomains  []string `json:"target_domains"`
	SourceCIDRs    []string `json:"source_cidrs"`
}

// FirewallSplitDraft stores saved selectors for the split firewall mode.
type FirewallSplitDraft struct {
	Proxy           FirewallSelectorSet `json:"proxy"`
	Bypass          FirewallSelectorSet `json:"bypass"`
	ExcludedSources []string            `json:"excluded_sources"`
}

// FirewallModeDrafts stores saved selectors for each supported LuCI firewall mode.
type FirewallModeDrafts struct {
	Hosts   FirewallModeDraft  `json:"hosts"`
	Targets FirewallModeDraft  `json:"targets"`
	Split   FirewallSplitDraft `json:"split"`
}

// FirewallSettings stores transparent proxy routing preferences.
type FirewallSettings struct {
	Enabled              bool                                `json:"enabled"`
	TransparentPort      int                                 `json:"transparent_port"`
	Mode                 FirewallMode                        `json:"mode"`
	DisableIPv6          bool                                `json:"disable_ipv6"`
	Hosts                []string                            `json:"hosts"`
	Targets              FirewallSelectorSet                 `json:"targets"`
	Split                FirewallSplitSettings               `json:"split"`
	TargetServiceCatalog map[string]FirewallTargetDefinition `json:"target_service_catalog"`
	ModeDrafts           FirewallModeDrafts                  `json:"mode_drafts"`
	BlockQUIC            bool                                `json:"block_quic"`
}

// DefaultSettings returns the baseline configuration used on first start.
func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:       8,
		RefreshInterval:     NewDuration(time.Hour),
		HealthCheckInterval: NewDuration(30 * time.Second),
		SwitchCooldown:      NewDuration(5 * time.Minute),
		LatencyThreshold:    NewDuration(50 * time.Millisecond),
		DNS:                 DefaultDNSSettings(),
		Firewall: FirewallSettings{
			Enabled:              false,
			TransparentPort:      12345,
			Mode:                 FirewallModeDisabled,
			DisableIPv6:          false,
			Hosts:                nil,
			Targets:              FirewallSelectorSet{},
			Split:                DefaultFirewallSplitSettings(),
			TargetServiceCatalog: nil,
			ModeDrafts:           FirewallModeDrafts{},
			BlockQUIC:            false,
		},
		AutoMode: false,
		Mode:     SelectionModeManual,
		LogLevel: "info",
	}
}

// DefaultFirewallSplitSettings returns the default split tunnelling policy.
func DefaultFirewallSplitSettings() FirewallSplitSettings {
	return FirewallSplitSettings{
		Proxy:           FirewallSelectorSet{},
		Bypass:          FirewallSelectorSet{},
		ExcludedSources: nil,
		DefaultAction:   FirewallDefaultActionDirect,
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

// NormalizeFirewallMode coerces unknown values to disabled.
func NormalizeFirewallMode(mode FirewallMode) FirewallMode {
	switch mode {
	case FirewallModeHosts, FirewallModeTargets, FirewallModeSplit:
		return mode
	default:
		return FirewallModeDisabled
	}
}

// NormalizeFirewallDefaultAction coerces unknown values to direct.
func NormalizeFirewallDefaultAction(action FirewallDefaultAction) FirewallDefaultAction {
	switch action {
	case FirewallDefaultActionProxy:
		return FirewallDefaultActionProxy
	default:
		return FirewallDefaultActionDirect
	}
}

// NormalizeFirewallTargetMode coerces unknown legacy values to proxy.
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

// CloneFirewallSelectorSet deep-copies one firewall selector set.
func CloneFirewallSelectorSet(value FirewallSelectorSet) FirewallSelectorSet {
	return FirewallSelectorSet{
		Services: append([]string(nil), value.Services...),
		Domains:  append([]string(nil), value.Domains...),
		CIDRs:    append([]string(nil), value.CIDRs...),
	}
}

// CloneFirewallSplitSettings deep-copies one split tunnelling policy.
func CloneFirewallSplitSettings(value FirewallSplitSettings) FirewallSplitSettings {
	return FirewallSplitSettings{
		Proxy:           CloneFirewallSelectorSet(value.Proxy),
		Bypass:          CloneFirewallSelectorSet(value.Bypass),
		ExcludedSources: append([]string(nil), value.ExcludedSources...),
		DefaultAction:   NormalizeFirewallDefaultAction(value.DefaultAction),
	}
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

// CloneFirewallSplitDraft deep-copies one split firewall draft.
func CloneFirewallSplitDraft(draft FirewallSplitDraft) FirewallSplitDraft {
	return FirewallSplitDraft{
		Proxy:           CloneFirewallSelectorSet(draft.Proxy),
		Bypass:          CloneFirewallSelectorSet(draft.Bypass),
		ExcludedSources: append([]string(nil), draft.ExcludedSources...),
	}
}

// CloneFirewallModeDrafts deep-copies all firewall mode drafts.
func CloneFirewallModeDrafts(drafts FirewallModeDrafts) FirewallModeDrafts {
	return FirewallModeDrafts{
		Hosts:   CloneFirewallModeDraft(drafts.Hosts),
		Targets: CloneFirewallModeDraft(drafts.Targets),
		Split:   CloneFirewallSplitDraft(drafts.Split),
	}
}

// FirewallSelectorSetFromTargets converts parsed mixed selectors into a selector set.
func FirewallSelectorSetFromTargets(targets FirewallTargets) FirewallSelectorSet {
	return FirewallSelectorSet{
		Services: append([]string(nil), targets.Services...),
		Domains:  append([]string(nil), targets.Domains...),
		CIDRs:    append([]string(nil), targets.CIDRs...),
	}
}

// FirewallSelectorSetHasEntries reports whether the selector set contains any selectors.
func FirewallSelectorSetHasEntries(value FirewallSelectorSet) bool {
	return len(value.Services) > 0 || len(value.Domains) > 0 || len(value.CIDRs) > 0
}

// FirewallRoutingEnabled reports whether the firewall has active routing selectors.
func FirewallRoutingEnabled(settings FirewallSettings) bool {
	if !settings.Enabled {
		return false
	}

	switch NormalizeFirewallMode(settings.Mode) {
	case FirewallModeHosts:
		return len(settings.Hosts) > 0
	case FirewallModeTargets:
		return FirewallSelectorSetHasEntries(settings.Targets)
	case FirewallModeSplit:
		return FirewallSelectorSetHasEntries(settings.Split.Proxy) ||
			FirewallSelectorSetHasEntries(settings.Split.Bypass)
	default:
		return false
	}
}

// CanonicalFirewallMode infers the effective firewall mode for legacy or partial settings.
func CanonicalFirewallMode(settings FirewallSettings) FirewallMode {
	if !settings.Enabled {
		return FirewallModeDisabled
	}

	switch NormalizeFirewallMode(settings.Mode) {
	case FirewallModeHosts:
		if len(settings.Hosts) > 0 {
			return FirewallModeHosts
		}
	case FirewallModeTargets:
		if FirewallSelectorSetHasEntries(settings.Targets) {
			return FirewallModeTargets
		}
	case FirewallModeSplit:
		if FirewallSelectorSetHasEntries(settings.Split.Proxy) || FirewallSelectorSetHasEntries(settings.Split.Bypass) {
			return FirewallModeSplit
		}
	}

	if len(settings.Hosts) > 0 {
		return FirewallModeHosts
	}
	if FirewallSelectorSetHasEntries(settings.Split.Proxy) || FirewallSelectorSetHasEntries(settings.Split.Bypass) {
		return FirewallModeSplit
	}
	if FirewallSelectorSetHasEntries(settings.Targets) {
		return FirewallModeTargets
	}

	return FirewallModeDisabled
}

// CanonicalFirewallSettings normalizes firewall settings for compatibility with older persisted states.
func CanonicalFirewallSettings(settings FirewallSettings) FirewallSettings {
	settings.Mode = CanonicalFirewallMode(settings)
	settings.Targets = CloneFirewallSelectorSet(settings.Targets)
	settings.Split = CloneFirewallSplitSettings(settings.Split)
	settings.ModeDrafts = CloneFirewallModeDrafts(settings.ModeDrafts)
	settings.Hosts = append([]string(nil), settings.Hosts...)
	settings.TargetServiceCatalog = CloneFirewallTargetCatalog(settings.TargetServiceCatalog)
	return settings
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

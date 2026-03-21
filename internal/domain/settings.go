package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	Firewall            FirewallSettings `json:"firewall"`
	AutoMode            bool             `json:"auto_mode"`
	Mode                SelectionMode    `json:"mode"`
	LogLevel            string           `json:"log_level"`
}

// FirewallSettings stores transparent proxy routing preferences.
type FirewallSettings struct {
	Enabled         bool     `json:"enabled"`
	TransparentPort int      `json:"transparent_port"`
	TargetCIDRs     []string `json:"target_cidrs"`
	SourceCIDRs     []string `json:"source_cidrs"`
	BlockQUIC       bool     `json:"block_quic"`
}

// DefaultSettings returns the baseline configuration used on first start.
func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:       1,
		RefreshInterval:     NewDuration(time.Hour),
		HealthCheckInterval: NewDuration(30 * time.Second),
		SwitchCooldown:      NewDuration(5 * time.Minute),
		LatencyThreshold:    NewDuration(50 * time.Millisecond),
		Firewall: FirewallSettings{
			Enabled:         false,
			TransparentPort: 12345,
			TargetCIDRs:     nil,
			SourceCIDRs:     nil,
			BlockQUIC:       true,
		},
		AutoMode: false,
		Mode:     SelectionModeManual,
		LogLevel: "info",
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

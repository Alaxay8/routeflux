package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
)

var (
	ErrUnsupportedSettingsSchema = errors.New("unsupported settings schema version")
	ErrUnsupportedStateSchema    = errors.New("unsupported state schema version")
)

func decodeSettings(data []byte, path string) (domain.Settings, error) {
	type rawDNSSettings struct {
		Mode          *domain.DNSMode      `json:"mode"`
		Transport     *domain.DNSTransport `json:"transport"`
		Servers       *[]string            `json:"servers"`
		Bootstrap     *[]string            `json:"bootstrap"`
		DirectDomains *[]string            `json:"direct_domains"`
	}

	type rawFirewallModeDraft struct {
		TargetServices *[]string `json:"target_services"`
		TargetCIDRs    *[]string `json:"target_cidrs"`
		TargetDomains  *[]string `json:"target_domains"`
		SourceCIDRs    *[]string `json:"source_cidrs"`
	}

	type rawFirewallModeDrafts struct {
		Hosts      *rawFirewallModeDraft `json:"hosts"`
		Targets    *rawFirewallModeDraft `json:"targets"`
		AntiTarget *rawFirewallModeDraft `json:"anti_target"`
	}

	type rawFirewallSettings struct {
		Enabled              *bool                                       `json:"enabled"`
		TransparentPort      *int                                        `json:"transparent_port"`
		TargetMode           *domain.FirewallTargetMode                  `json:"target_mode"`
		TargetServices       *[]string                                   `json:"target_services"`
		TargetServiceCatalog *map[string]domain.FirewallTargetDefinition `json:"target_service_catalog"`
		TargetCIDRs          *[]string                                   `json:"target_cidrs"`
		TargetDomains        *[]string                                   `json:"target_domains"`
		SourceCIDRs          *[]string                                   `json:"source_cidrs"`
		ModeDrafts           *rawFirewallModeDrafts                      `json:"mode_drafts"`
		BlockQUIC            *bool                                       `json:"block_quic"`
	}

	type rawSettings struct {
		SchemaVersion       *int                  `json:"schema_version"`
		RefreshInterval     *domain.Duration      `json:"refresh_interval"`
		HealthCheckInterval *domain.Duration      `json:"health_check_interval"`
		SwitchCooldown      *domain.Duration      `json:"switch_cooldown"`
		LatencyThreshold    *domain.Duration      `json:"latency_threshold"`
		DNS                 *rawDNSSettings       `json:"dns"`
		Firewall            *rawFirewallSettings  `json:"firewall"`
		AutoMode            *bool                 `json:"auto_mode"`
		Mode                *domain.SelectionMode `json:"mode"`
		LogLevel            *string               `json:"log_level"`
	}

	var raw rawSettings
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Settings{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}

	settings := domain.DefaultSettings()
	schemaVersion := 0
	if raw.SchemaVersion != nil {
		schemaVersion = *raw.SchemaVersion
	}
	if schemaVersion > settings.SchemaVersion {
		return domain.Settings{}, fmt.Errorf("%w %d", ErrUnsupportedSettingsSchema, schemaVersion)
	}

	if raw.RefreshInterval != nil {
		settings.RefreshInterval = *raw.RefreshInterval
	}
	if raw.HealthCheckInterval != nil {
		settings.HealthCheckInterval = *raw.HealthCheckInterval
	}
	if raw.SwitchCooldown != nil {
		settings.SwitchCooldown = *raw.SwitchCooldown
	}
	if raw.LatencyThreshold != nil {
		settings.LatencyThreshold = *raw.LatencyThreshold
	}
	if raw.AutoMode != nil {
		settings.AutoMode = *raw.AutoMode
	}
	if raw.Mode != nil {
		settings.Mode = *raw.Mode
	}
	if raw.LogLevel != nil {
		settings.LogLevel = *raw.LogLevel
	}

	if raw.DNS != nil {
		if raw.DNS.Mode != nil {
			settings.DNS.Mode = *raw.DNS.Mode
		}
		if raw.DNS.Transport != nil {
			settings.DNS.Transport = *raw.DNS.Transport
		}
		if raw.DNS.Servers != nil {
			settings.DNS.Servers = append([]string(nil), (*raw.DNS.Servers)...)
		}
		if raw.DNS.Bootstrap != nil {
			settings.DNS.Bootstrap = append([]string(nil), (*raw.DNS.Bootstrap)...)
		}
		if raw.DNS.DirectDomains != nil {
			settings.DNS.DirectDomains = append([]string(nil), (*raw.DNS.DirectDomains)...)
		}
	}

	if raw.Firewall != nil {
		if raw.Firewall.Enabled != nil {
			settings.Firewall.Enabled = *raw.Firewall.Enabled
		}
		if raw.Firewall.TransparentPort != nil {
			settings.Firewall.TransparentPort = *raw.Firewall.TransparentPort
		}
		if raw.Firewall.TargetMode != nil {
			settings.Firewall.TargetMode = domain.NormalizeFirewallTargetMode(*raw.Firewall.TargetMode)
		}
		if raw.Firewall.TargetServices != nil {
			settings.Firewall.TargetServices = append([]string(nil), (*raw.Firewall.TargetServices)...)
		}
		if raw.Firewall.TargetServiceCatalog != nil {
			settings.Firewall.TargetServiceCatalog = domain.CloneFirewallTargetCatalog(*raw.Firewall.TargetServiceCatalog)
		}
		if raw.Firewall.TargetCIDRs != nil {
			settings.Firewall.TargetCIDRs = append([]string(nil), (*raw.Firewall.TargetCIDRs)...)
		}
		if raw.Firewall.TargetDomains != nil {
			settings.Firewall.TargetDomains = append([]string(nil), (*raw.Firewall.TargetDomains)...)
		}
		if raw.Firewall.SourceCIDRs != nil {
			settings.Firewall.SourceCIDRs = append([]string(nil), (*raw.Firewall.SourceCIDRs)...)
		}
		if raw.Firewall.ModeDrafts != nil {
			if raw.Firewall.ModeDrafts.Hosts != nil {
				if raw.Firewall.ModeDrafts.Hosts.TargetServices != nil {
					settings.Firewall.ModeDrafts.Hosts.TargetServices = append([]string(nil), (*raw.Firewall.ModeDrafts.Hosts.TargetServices)...)
				}
				if raw.Firewall.ModeDrafts.Hosts.TargetCIDRs != nil {
					settings.Firewall.ModeDrafts.Hosts.TargetCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.Hosts.TargetCIDRs)...)
				}
				if raw.Firewall.ModeDrafts.Hosts.TargetDomains != nil {
					settings.Firewall.ModeDrafts.Hosts.TargetDomains = append([]string(nil), (*raw.Firewall.ModeDrafts.Hosts.TargetDomains)...)
				}
				if raw.Firewall.ModeDrafts.Hosts.SourceCIDRs != nil {
					settings.Firewall.ModeDrafts.Hosts.SourceCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.Hosts.SourceCIDRs)...)
				}
			}
			if raw.Firewall.ModeDrafts.Targets != nil {
				if raw.Firewall.ModeDrafts.Targets.TargetServices != nil {
					settings.Firewall.ModeDrafts.Targets.TargetServices = append([]string(nil), (*raw.Firewall.ModeDrafts.Targets.TargetServices)...)
				}
				if raw.Firewall.ModeDrafts.Targets.TargetCIDRs != nil {
					settings.Firewall.ModeDrafts.Targets.TargetCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.Targets.TargetCIDRs)...)
				}
				if raw.Firewall.ModeDrafts.Targets.TargetDomains != nil {
					settings.Firewall.ModeDrafts.Targets.TargetDomains = append([]string(nil), (*raw.Firewall.ModeDrafts.Targets.TargetDomains)...)
				}
				if raw.Firewall.ModeDrafts.Targets.SourceCIDRs != nil {
					settings.Firewall.ModeDrafts.Targets.SourceCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.Targets.SourceCIDRs)...)
				}
			}
			if raw.Firewall.ModeDrafts.AntiTarget != nil {
				if raw.Firewall.ModeDrafts.AntiTarget.TargetServices != nil {
					settings.Firewall.ModeDrafts.AntiTarget.TargetServices = append([]string(nil), (*raw.Firewall.ModeDrafts.AntiTarget.TargetServices)...)
				}
				if raw.Firewall.ModeDrafts.AntiTarget.TargetCIDRs != nil {
					settings.Firewall.ModeDrafts.AntiTarget.TargetCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.AntiTarget.TargetCIDRs)...)
				}
				if raw.Firewall.ModeDrafts.AntiTarget.TargetDomains != nil {
					settings.Firewall.ModeDrafts.AntiTarget.TargetDomains = append([]string(nil), (*raw.Firewall.ModeDrafts.AntiTarget.TargetDomains)...)
				}
				if raw.Firewall.ModeDrafts.AntiTarget.SourceCIDRs != nil {
					settings.Firewall.ModeDrafts.AntiTarget.SourceCIDRs = append([]string(nil), (*raw.Firewall.ModeDrafts.AntiTarget.SourceCIDRs)...)
				}
			}
		}
		if raw.Firewall.BlockQUIC != nil {
			settings.Firewall.BlockQUIC = *raw.Firewall.BlockQUIC
		}
	}

	if raw.AutoMode != nil && raw.Mode == nil && *raw.AutoMode {
		settings.Mode = domain.SelectionModeAuto
	}

	settings.SchemaVersion = domain.DefaultSettings().SchemaVersion
	return settings, nil
}

func decodeState(data []byte, path string) (domain.RuntimeState, error) {
	type rawState struct {
		SchemaVersion        *int                          `json:"schema_version"`
		ActiveSubscriptionID *string                       `json:"active_subscription_id"`
		ActiveNodeID         *string                       `json:"active_node_id"`
		Mode                 *domain.SelectionMode         `json:"mode"`
		Connected            *bool                         `json:"connected"`
		LastRefreshAt        *map[string]time.Time         `json:"last_refresh_at"`
		Health               *map[string]domain.NodeHealth `json:"health"`
		LastSwitchAt         *time.Time                    `json:"last_switch_at"`
		LastSuccessAt        *time.Time                    `json:"last_success_at"`
		LastFailureReason    *string                       `json:"last_failure_reason"`
	}

	var raw rawState
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.RuntimeState{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}

	state := domain.DefaultRuntimeState()
	schemaVersion := 0
	if raw.SchemaVersion != nil {
		schemaVersion = *raw.SchemaVersion
	}
	if schemaVersion > state.SchemaVersion {
		return domain.RuntimeState{}, fmt.Errorf("%w %d", ErrUnsupportedStateSchema, schemaVersion)
	}

	if raw.ActiveSubscriptionID != nil {
		state.ActiveSubscriptionID = *raw.ActiveSubscriptionID
	}
	if raw.ActiveNodeID != nil {
		state.ActiveNodeID = *raw.ActiveNodeID
	}
	if raw.Mode != nil {
		state.Mode = *raw.Mode
	}
	if raw.Connected != nil {
		state.Connected = *raw.Connected
	}
	if raw.LastRefreshAt != nil {
		state.LastRefreshAt = *raw.LastRefreshAt
	}
	if raw.Health != nil {
		state.Health = *raw.Health
	}
	if raw.LastSwitchAt != nil {
		state.LastSwitchAt = *raw.LastSwitchAt
	}
	if raw.LastSuccessAt != nil {
		state.LastSuccessAt = *raw.LastSuccessAt
	}
	if raw.LastFailureReason != nil {
		state.LastFailureReason = *raw.LastFailureReason
	}

	if state.LastRefreshAt == nil {
		state.LastRefreshAt = make(map[string]time.Time)
	}
	if state.Health == nil {
		state.Health = make(map[string]domain.NodeHealth)
	}

	state.SchemaVersion = domain.DefaultRuntimeState().SchemaVersion
	return state, nil
}

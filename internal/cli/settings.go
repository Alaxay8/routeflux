package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func newSettingsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Get or update RouteFlux settings",
		Long:  "General RouteFlux settings. For DNS, prefer `routeflux dns ...` because it explains each option in plain language.",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "get",
			Short: "Print current settings",
			RunE: func(cmd *cobra.Command, args []string) error {
				settings, err := opts.service.GetSettings()
				if err != nil {
					return err
				}

				if opts.jsonOutput {
					return printOutput(cmd, true, settings, "")
				}

				text := fmt.Sprintf(
					"refresh-interval=%s\nhealth-check-interval=%s\nswitch-cooldown=%s\nlatency-threshold=%s\nauto-mode=%t\nmode=%s\nlog-level=%s\nfirewall-enabled=%t\nfirewall-mode=%s\nfirewall-port=%d\nfirewall-default-action=%s\nfirewall-targets=%s\nfirewall-target-services=%s\nfirewall-target-domains=%s\nfirewall-target-cidrs=%s\nfirewall-split-proxy=%s\nfirewall-split-bypass=%s\nfirewall-split-excluded-sources=%s\nfirewall-hosts=%s\nfirewall-block-quic=%t\nfirewall-disable-ipv6=%t\nzapret-enabled=%t\nzapret-selectors=%s\nzapret-services=%s\nzapret-domains=%s\nzapret-failback-success-threshold=%d",
					settings.RefreshInterval,
					settings.HealthCheckInterval,
					settings.SwitchCooldown,
					settings.LatencyThreshold,
					settings.AutoMode,
					settings.Mode,
					settings.LogLevel,
					settings.Firewall.Enabled,
					domain.NormalizeFirewallMode(settings.Firewall.Mode),
					settings.Firewall.TransparentPort,
					domain.NormalizeFirewallDefaultAction(settings.Firewall.Split.DefaultAction),
					firewallSelectorSummary(settings.Firewall.Targets),
					strings.Join(settings.Firewall.Targets.Services, ", "),
					strings.Join(settings.Firewall.Targets.Domains, ", "),
					strings.Join(settings.Firewall.Targets.CIDRs, ", "),
					firewallSelectorSummary(settings.Firewall.Split.Proxy),
					firewallSelectorSummary(settings.Firewall.Split.Bypass),
					strings.Join(settings.Firewall.Split.ExcludedSources, ", "),
					strings.Join(settings.Firewall.Hosts, ", "),
					settings.Firewall.BlockQUIC,
					settings.Firewall.DisableIPv6,
					settings.Zapret.Enabled,
					zapretSelectorSummary(settings.Zapret.Selectors),
					strings.Join(settings.Zapret.Selectors.Services, ", "),
					strings.Join(settings.Zapret.Selectors.Domains, ", "),
					settings.Zapret.FailbackSuccessThreshold,
				)
				return printOutput(cmd, false, nil, text)
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Update a setting",
			Long:  "Update one low-level setting key. For DNS settings, prefer `routeflux dns set ...` because it uses simpler names and clearer help.",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				settings, err := opts.service.SetSetting(args[0], args[1])
				if err != nil {
					return err
				}

				return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Updated %s=%s", args[0], args[1]))
			},
		},
	)

	return cmd
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
					"refresh-interval=%s\nhealth-check-interval=%s\nswitch-cooldown=%s\nlatency-threshold=%s\nauto-mode=%t\nmode=%s\nlog-level=%s\nfirewall-enabled=%t\nfirewall-port=%d\nfirewall-targets=%s\nfirewall-target-services=%s\nfirewall-target-domains=%s\nfirewall-target-cidrs=%s\nfirewall-hosts=%s\nfirewall-block-quic=%t",
					settings.RefreshInterval,
					settings.HealthCheckInterval,
					settings.SwitchCooldown,
					settings.LatencyThreshold,
					settings.AutoMode,
					settings.Mode,
					settings.LogLevel,
					settings.Firewall.Enabled,
					settings.Firewall.TransparentPort,
					firewallTargetsSummary(settings.Firewall),
					strings.Join(settings.Firewall.TargetServices, ", "),
					strings.Join(settings.Firewall.TargetDomains, ", "),
					strings.Join(settings.Firewall.TargetCIDRs, ", "),
					strings.Join(settings.Firewall.SourceCIDRs, ", "),
					settings.Firewall.BlockQUIC,
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

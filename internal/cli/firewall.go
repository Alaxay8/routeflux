package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newFirewallCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Configure simple OpenWrt routing through RouteFlux",
	}

	var port int
	setCmd := &cobra.Command{
		Use:   "set <ip-or-cidr-or-range> [more ...]",
		Short: "Enable routing for destination IPv4 addresses, CIDRs, or ranges",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.ConfigureFirewall(context.Background(), args, true, port)
			if err != nil {
				return err
			}

			text := fmt.Sprintf("Firewall enabled for %s", strings.Join(settings.TargetCIDRs, ", "))
			return printOutput(cmd, opts.jsonOutput, settings, text)
		},
	}
	setCmd.Flags().IntVar(&port, "port", 12345, "Transparent redirect port")

	hostCmd := &cobra.Command{
		Use:   "host <ipv4-or-cidr-or-range> [more ...]",
		Short: "Route all TCP traffic from selected LAN hosts or ranges through RouteFlux",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.ConfigureFirewallHosts(context.Background(), args, true, port)
			if err != nil {
				return err
			}

			text := fmt.Sprintf("Host routing enabled for %s", strings.Join(settings.SourceCIDRs, ", "))
			return printOutput(cmd, opts.jsonOutput, settings, text)
		},
	}
	hostCmd.Flags().IntVar(&port, "port", 12345, "Transparent redirect port")

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable simple destination routing",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.DisableFirewall(context.Background())
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, "Firewall disabled")
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show firewall routing status",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.GetFirewallSettings()
			if err != nil {
				return err
			}

			text := fmt.Sprintf(
				"enabled=%t\ntransparent-port=%d\ntargets=%s\nhosts=%s\nblock-quic=%t",
				settings.Enabled,
				settings.TransparentPort,
				strings.Join(settings.TargetCIDRs, ", "),
				strings.Join(settings.SourceCIDRs, ", "),
				settings.BlockQUIC,
			)
			return printOutput(cmd, opts.jsonOutput, settings, text)
		},
	}

	cmd.AddCommand(setCmd, hostCmd, disableCmd, statusCmd)
	return cmd
}

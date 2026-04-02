package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func newFirewallCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Easy firewall routing settings for RouteFlux",
		Long: strings.TrimSpace(`
Firewall controls which traffic RouteFlux redirects into the transparent proxy.

Think of it like this:
- mode answers "what do you want to match?"
- targets means selected services, domains, or destination IPv4 targets go through RouteFlux
- anti-target means selected services, domains, or destination IPv4 targets stay direct while everything else from LAN clients goes through RouteFlux
- hosts means all traffic from selected LAN clients goes through RouteFlux
`),
		Example: strings.TrimSpace(`
routeflux firewall get
routeflux firewall explain
routeflux firewall set hosts 192.168.1.150
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts all
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux firewall set anti-target gosuslugi.ru sberbank.ru
routeflux firewall set port 12345
routeflux firewall set block-quic true
routeflux firewall draft targets youtube instagram
routeflux firewall draft hosts all
routeflux firewall disable
`),
	}

	cmd.AddCommand(
		newFirewallGetCmd(opts),
		newFirewallExplainCmd(opts),
		newFirewallSetCmd(opts),
		newFirewallDraftCmd(opts),
		newFirewallHostCmd(opts),
		newFirewallDisableCmd(opts),
	)

	return cmd
}

func newFirewallGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Aliases: []string{"status"},
		Short:   "Show current firewall routing settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.GetFirewallSettings()
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, settings, "")
			}

			return printOutput(cmd, false, nil, renderFirewallSettingsText(settings))
		},
	}
}

func newFirewallExplainCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "explain",
		Short: "Explain firewall routing settings in plain language",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printOutput(cmd, false, nil, strings.TrimSpace(fmt.Sprintf(`
Firewall modes:
- disabled: Do not redirect router traffic through RouteFlux.
  Example: the proxy is installed, but no device or destination is forced through it.
  Command: routeflux firewall disable
- targets: Send traffic through RouteFlux only when the destination matches selected services, domains, or IPv4 targets.
  Example: routeflux firewall set targets youtube telegram discord means "those services only".
  Service presets: %s.
  Create your own aliases with routeflux services set openai openai.com chatgpt.com.
  Popular root domains like youtube.com, instagram.com, netflix.com, x.com, gemini.google.com, and notebooklm.google.com still auto-expand to the domain families they need.
  Use gemini-mobile or notebooklm-mobile for the Android or iOS apps when the web preset is too narrow.
  Gemini and NotebookLM mobile presets are broader and still best-effort because Google apps can use extra shared infrastructure and direct IPv4 endpoints.
  Command: routeflux firewall set targets youtube telegram discord 1.1.1.1
- anti-target: Send all other LAN traffic through RouteFlux, but keep selected services, domains, or destination IPv4 targets direct.
  Example: routeflux firewall set anti-target gosuslugi.ru sberbank.ru means "everything except those resources".
  This mode is best for LAN clients and does not redirect router-originated traffic.
  Command: routeflux firewall set anti-target gosuslugi.ru sberbank.ru
- hosts: Send all traffic from selected LAN devices through RouteFlux.
  Example: route one TV, phone, or laptop through the proxy.
  Command: routeflux firewall set hosts 192.168.1.150

Hosts selectors:
- one device: 192.168.1.150
- subnet: 192.168.1.0/24
- range: 192.168.1.150-192.168.1.159
- all or *: all common private LAN ranges

Other options:
- port: port used for transparent redirect
- block-quic: when true, RouteFlux blocks proxied QUIC/UDP traffic so clients fall back to TCP; when false, QUIC is proxied normally
`, firewallPresetSummary())))
		},
	}
}

func firewallPresetSummary() string {
	return strings.Join(domain.FirewallTargetServiceNames(), ", ")
}

func newFirewallSetCmd(opts *rootOptions) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "set <option> <value...>",
		Short: "Change firewall routing settings",
		Long: strings.TrimSpace(`
Firewall options:
- targets: selected service presets, domains, IPv4 addresses, CIDRs, or ranges
- anti-target: selected service presets, domains, IPv4 addresses, CIDRs, or ranges that should bypass the proxy
- hosts: LAN clients whose traffic should go through RouteFlux
- port: transparent redirect port
- block-quic: true or false
`),
		Example: strings.TrimSpace(`
routeflux firewall set hosts 192.168.1.150
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set hosts 192.168.1.150-192.168.1.159
routeflux firewall set hosts all
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux firewall set anti-target gosuslugi.ru sberbank.ru
routeflux firewall set port 12345
routeflux firewall set block-quic true
routeflux firewall set youtube.com 1.1.1.1
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			option, values, err := parseFirewallSetArgs(args)
			if err != nil {
				return err
			}

			switch option {
			case "targets":
				settings, err := opts.service.GetFirewallSettings()
				if err != nil {
					return err
				}
				targetPort := settings.TransparentPort
				if cmd.Flags().Changed("port") {
					targetPort = port
				}

				updated, err := opts.service.ConfigureFirewall(context.Background(), values, true, targetPort)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall targets set to %s", firewallTargetsSummary(updated)))
			case "anti-target":
				settings, err := opts.service.GetFirewallSettings()
				if err != nil {
					return err
				}
				targetPort := settings.TransparentPort
				if cmd.Flags().Changed("port") {
					targetPort = port
				}

				updated, err := opts.service.ConfigureFirewallAntiTargets(context.Background(), values, true, targetPort)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall anti-targets set to %s", firewallTargetsSummary(updated)))
			case "hosts":
				settings, err := opts.service.GetFirewallSettings()
				if err != nil {
					return err
				}
				targetPort := settings.TransparentPort
				if cmd.Flags().Changed("port") {
					targetPort = port
				}

				updated, err := opts.service.ConfigureFirewallHosts(context.Background(), values, true, targetPort)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall hosts set to %s", strings.Join(updated.SourceCIDRs, ", ")))
			case "port":
				if len(values) != 1 {
					return fmt.Errorf("firewall port expects exactly one value")
				}
				value, err := strconv.Atoi(values[0])
				if err != nil {
					return fmt.Errorf("parse firewall port %q: %w", values[0], err)
				}
				updated, err := opts.service.UpdateFirewallPort(context.Background(), value)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall port set to %d", updated.TransparentPort))
			case "block-quic":
				if len(values) != 1 {
					return fmt.Errorf("firewall block-quic expects exactly one value")
				}
				value, err := strconv.ParseBool(values[0])
				if err != nil {
					return fmt.Errorf("parse firewall block-quic %q: %w", values[0], err)
				}
				updated, err := opts.service.UpdateFirewallBlockQUIC(context.Background(), value)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall block-quic set to %t", updated.BlockQUIC))
			default:
				return fmt.Errorf("unsupported firewall option %q", option)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Override transparent redirect port for hosts or targets")
	return cmd
}

func newFirewallDraftCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "draft <hosts|targets|anti-target> [selector...]",
		Short: "Store or clear saved LuCI selectors for one firewall mode",
		Long: strings.TrimSpace(`
Draft slots are saved selector sets for the LuCI Firewall page.

- routeflux firewall draft targets youtube instagram stores the targets draft
- routeflux firewall draft hosts all stores the hosts draft
- routeflux firewall draft anti-target gosuslugi.ru stores the anti-target draft
- routeflux firewall draft targets clears the targets draft
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := strings.TrimSpace(strings.ToLower(args[0]))
			if mode == "anti-targets" {
				mode = "anti-target"
			}

			if len(args) == 1 {
				settings, err := opts.service.ClearFirewallModeDraft(context.Background(), mode)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Firewall draft %s cleared", mode))
			}

			settings, err := opts.service.UpdateFirewallModeDraft(context.Background(), mode, args[1:])
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Firewall draft %s updated", mode))
		},
	}
}

func newFirewallHostCmd(opts *rootOptions) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "host <ipv4-or-cidr-or-range|all|*> [more ...]",
		Short: "Legacy alias for routeflux firewall set hosts ...",
		Long: strings.TrimSpace(`
Choose which LAN clients should send all traffic through RouteFlux.

Supported selectors:
- single IPv4 address: 192.168.1.150
- IPv4 CIDR pool: 192.168.1.0/24
- IPv4 range: 192.168.1.150-192.168.1.159
- all or *: all common private LAN ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
`),
		Example: strings.TrimSpace(`
routeflux firewall host 192.168.1.150
routeflux firewall host 192.168.1.0/24
routeflux firewall host 192.168.1.150-192.168.1.159
routeflux firewall host 192.168.1.10 192.168.1.32/27
routeflux firewall host all
routeflux firewall host *
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.GetFirewallSettings()
			if err != nil {
				return err
			}
			targetPort := settings.TransparentPort
			if cmd.Flags().Changed("port") {
				targetPort = port
			}

			updated, err := opts.service.ConfigureFirewallHosts(context.Background(), args, true, targetPort)
			if err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Host routing enabled for %s", strings.Join(updated.SourceCIDRs, ", ")))
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Override transparent redirect port")
	return cmd
}

func newFirewallDisableCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "disable",
		Aliases: []string{"off"},
		Short:   "Disable firewall routing",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.DisableFirewall(context.Background())
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, "Firewall disabled")
		},
	}
}

func parseFirewallSetArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("firewall set expects an option or target values")
	}

	switch strings.TrimSpace(strings.ToLower(args[0])) {
	case "targets", "anti-target", "anti-targets", "hosts", "port", "block-quic":
		if len(args) < 2 {
			return "", nil, fmt.Errorf("firewall %s expects at least one value", args[0])
		}
		option := strings.ToLower(strings.TrimSpace(args[0]))
		if option == "anti-targets" {
			option = "anti-target"
		}
		return option, args[1:], nil
	default:
		return "targets", args, nil
	}
}

func renderFirewallSettingsText(settings domain.FirewallSettings) string {
	return strings.Join([]string{
		fmt.Sprintf("enabled=%t", settings.Enabled),
		fmt.Sprintf("mode=%s", firewallMode(settings)),
		fmt.Sprintf("mode-help=%s", firewallModeHelp(settings)),
		fmt.Sprintf("transparent-port=%d", settings.TransparentPort),
		fmt.Sprintf("target-mode=%s", domain.NormalizeFirewallTargetMode(settings.TargetMode)),
		fmt.Sprintf("targets=%s", firewallTargetsSummary(settings)),
		fmt.Sprintf("target-services=%s", strings.Join(settings.TargetServices, ", ")),
		fmt.Sprintf("target-domains=%s", strings.Join(settings.TargetDomains, ", ")),
		fmt.Sprintf("target-ips=%s", strings.Join(settings.TargetCIDRs, ", ")),
		fmt.Sprintf("hosts=%s", strings.Join(settings.SourceCIDRs, ", ")),
		fmt.Sprintf("block-quic=%t", settings.BlockQUIC),
	}, "\n")
}

func firewallMode(settings domain.FirewallSettings) string {
	targetMode := domain.NormalizeFirewallTargetMode(settings.TargetMode)
	switch {
	case !settings.Enabled || (len(settings.TargetServices) == 0 && len(settings.TargetCIDRs) == 0 && len(settings.TargetDomains) == 0 && len(settings.SourceCIDRs) == 0):
		return "disabled"
	case (len(settings.TargetServices) > 0 || len(settings.TargetCIDRs) > 0 || len(settings.TargetDomains) > 0) && len(settings.SourceCIDRs) == 0:
		if targetMode == domain.FirewallTargetModeBypass {
			return "anti-target"
		}
		return "targets"
	case len(settings.SourceCIDRs) > 0 && len(settings.TargetServices) == 0 && len(settings.TargetCIDRs) == 0 && len(settings.TargetDomains) == 0:
		return "hosts"
	default:
		return "mixed"
	}
}

func firewallModeHelp(settings domain.FirewallSettings) string {
	switch firewallMode(settings) {
	case "disabled":
		return "No traffic is being redirected through RouteFlux."
	case "targets":
		return "Only traffic to selected services, domains, or destination IPv4 targets goes through RouteFlux."
	case "anti-target":
		return "Selected services, domains, or destination IPv4 targets stay direct while all other LAN traffic goes through RouteFlux."
	case "hosts":
		return "All traffic from selected LAN devices goes through RouteFlux."
	default:
		return "Both destination targets and source hosts are active."
	}
}

func firewallTargetsSummary(settings domain.FirewallSettings) string {
	values := make([]string, 0, len(settings.TargetServices)+len(settings.TargetDomains)+len(settings.TargetCIDRs))
	values = append(values, settings.TargetServices...)
	values = append(values, settings.TargetDomains...)
	values = append(values, settings.TargetCIDRs...)
	return strings.Join(values, ", ")
}

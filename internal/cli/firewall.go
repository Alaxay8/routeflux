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
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Long: strings.TrimSpace(`
Firewall controls which traffic RouteFlux redirects into the transparent proxy.
Common choices: hosts, targets, or bypass.
For mode-by-mode guidance, use routeflux firewall explain.
`),
		Example: strings.TrimSpace(`
routeflux firewall get
routeflux firewall set hosts 192.168.1.150
routeflux firewall set targets youtube instagram
routeflux firewall set bypass gosuslugi.ru --exclude-host 192.168.1.50
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
			return printOutput(cmd, false, nil, strings.TrimSpace(`
Firewall modes:
- disabled: Do not redirect router traffic through RouteFlux.
  Example: the proxy is installed, but no device or destination is forced through it.
  Command: routeflux firewall disable
- targets: Send traffic through RouteFlux only when the destination matches selected services, domains, or IPv4 targets.
  Example: only traffic to selected services or domains should go through the proxy.
  Command: routeflux firewall set targets youtube instagram
- bypass: Send all other traffic through RouteFlux while keeping selected resources direct and optionally excluding whole LAN devices.
  Example: proxy most traffic, but keep selected resources or devices direct.
  Command: routeflux firewall set bypass gosuslugi.ru --exclude-host 192.168.1.50
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
- ipv6: when disabled in RouteFlux, the router turns off IPv6 because transparent routing is IPv4-only and otherwise IPv6 can bypass the proxy

Advanced presets, split mode, and legacy compatibility are documented in README.
`))
		},
	}
}

func firewallPresetSummary() string {
	return strings.Join(domain.FirewallTargetServiceNames(), ", ")
}

func newFirewallSetCmd(opts *rootOptions) *cobra.Command {
	var port int
	var splitProxy []string
	var splitBypass []string
	var splitExcludedHosts []string

	cmd := &cobra.Command{
		Use:   "set <option> <value...>",
		Short: "Change firewall routing settings",
		Long: strings.TrimSpace(`
Common firewall options:
- targets: selected service presets, domains, IPv4 addresses, CIDRs, or ranges
- bypass: proxy everything except selected direct resources and excluded devices
- hosts: LAN clients whose traffic should go through RouteFlux
- port: transparent redirect port
- block-quic: true or false
- ipv6: disable or enable router IPv6 handling managed by RouteFlux

Advanced routing combinations are documented in README.
`),
		Example: strings.TrimSpace(`
routeflux firewall set hosts 192.168.1.150
routeflux firewall set hosts 192.168.1.0/24
routeflux firewall set targets youtube instagram 1.1.1.1
routeflux firewall set bypass gosuslugi.ru --exclude-host 192.168.1.50
routeflux firewall set port 12345
routeflux firewall set block-quic true
routeflux firewall set ipv6 disable
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
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall targets set to %s", firewallSelectorSummary(updated.Targets)))
			case "split":
				settings, err := opts.service.GetFirewallSettings()
				if err != nil {
					return err
				}
				targetPort := settings.TransparentPort
				if cmd.Flags().Changed("port") {
					targetPort = port
				}

				if len(values) > 0 {
					return fmt.Errorf("firewall split uses --proxy, --bypass, and --exclude-host flags instead of positional selectors")
				}

				updated, err := opts.service.ConfigureFirewallSplit(context.Background(), splitProxy, splitBypass, splitExcludedHosts, true, targetPort)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall split set to %s", firewallSplitSummary(updated.Split)))
			case "bypass":
				settings, err := opts.service.GetFirewallSettings()
				if err != nil {
					return err
				}
				targetPort := settings.TransparentPort
				if cmd.Flags().Changed("port") {
					targetPort = port
				}
				if len(splitProxy) > 0 || len(splitBypass) > 0 {
					return fmt.Errorf("firewall bypass uses positional selectors and --exclude-host instead of --proxy or --bypass")
				}

				updated, err := opts.service.ConfigureFirewallBypass(context.Background(), values, splitExcludedHosts, true, targetPort)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall bypass set to %s", firewallBypassSummary(updated.Split)))
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
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall anti-targets set to %s (deprecated: use routeflux firewall set bypass ...)", firewallSelectorSummary(updated.Split.Bypass)))
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
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall hosts set to %s", strings.Join(updated.Hosts, ", ")))
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
			case "ipv6":
				if len(values) != 1 {
					return fmt.Errorf("firewall ipv6 expects exactly one value")
				}
				disabled, err := parseFirewallIPv6State(values[0])
				if err != nil {
					return err
				}
				updated, err := opts.service.UpdateFirewallDisableIPv6(context.Background(), disabled)
				if err != nil {
					return err
				}
				stateLabel := "enabled"
				if updated.DisableIPv6 {
					stateLabel = "disabled"
				}
				return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Firewall IPv6 protection set to %s", stateLabel))
			default:
				return fmt.Errorf("unsupported firewall option %q", option)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Override transparent redirect port for hosts or targets")
	cmd.Flags().StringSliceVar(&splitProxy, "proxy", nil, "Split selectors that should go through RouteFlux")
	cmd.Flags().StringSliceVar(&splitBypass, "bypass", nil, "Split selectors that should stay direct")
	cmd.Flags().StringSliceVar(&splitExcludedHosts, "exclude-host", nil, "LAN hosts that should never be intercepted by bypass or split mode")
	return cmd
}

func newFirewallDraftCmd(opts *rootOptions) *cobra.Command {
	var splitProxy []string
	var splitBypass []string
	var splitExcludedHosts []string

	cmd := &cobra.Command{
		Use:    "draft <hosts|targets|bypass|split> [selector...]",
		Short:  "Store or clear saved LuCI selectors for one firewall mode",
		Hidden: true,
		Long: strings.TrimSpace(`
Draft slots are saved selector sets for the LuCI Firewall page.

- routeflux firewall draft targets youtube instagram stores the targets draft
- routeflux firewall draft hosts all stores the hosts draft
- routeflux firewall draft bypass gosuslugi.ru --exclude-host 192.168.1.50 stores the bypass draft
- routeflux firewall draft split --proxy youtube --bypass gosuslugi.ru stores the split draft
- routeflux firewall draft targets clears the targets draft
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := strings.TrimSpace(strings.ToLower(args[0]))
			hasSplitDraftValues := len(splitProxy) > 0 || len(splitBypass) > 0 || len(splitExcludedHosts) > 0

			if mode != "split" && mode != "bypass" && len(args) == 1 {
				settings, err := opts.service.ClearFirewallModeDraft(context.Background(), mode)
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Firewall draft %s cleared", mode))
			}

			var (
				settings domain.FirewallSettings
				err      error
			)
			if mode == "split" {
				if len(args) > 1 {
					return fmt.Errorf("firewall draft split uses --proxy, --bypass, and --exclude-host flags instead of positional selectors")
				}
				if !hasSplitDraftValues {
					settings, err = opts.service.ClearFirewallModeDraft(context.Background(), mode)
				} else {
					settings, err = opts.service.UpdateFirewallSplitDraft(context.Background(), splitProxy, splitBypass, splitExcludedHosts)
				}
			} else if mode == "bypass" {
				if len(splitProxy) > 0 || len(splitBypass) > 0 {
					return fmt.Errorf("firewall draft bypass uses positional selectors and --exclude-host instead of --proxy or --bypass")
				}
				if len(args) == 1 && !hasSplitDraftValues {
					settings, err = opts.service.ClearFirewallModeDraft(context.Background(), mode)
				} else {
					settings, err = opts.service.UpdateFirewallBypassDraft(context.Background(), args[1:], splitExcludedHosts)
				}
			} else {
				settings, err = opts.service.UpdateFirewallModeDraft(context.Background(), mode, args[1:])
			}
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Firewall draft %s updated", mode))
		},
	}

	cmd.Flags().StringSliceVar(&splitProxy, "proxy", nil, "Split draft selectors that should go through RouteFlux")
	cmd.Flags().StringSliceVar(&splitBypass, "bypass", nil, "Split draft selectors that should stay direct")
	cmd.Flags().StringSliceVar(&splitExcludedHosts, "exclude-host", nil, "Bypass or split draft LAN hosts that should never be intercepted")
	return cmd
}

func newFirewallHostCmd(opts *rootOptions) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:    "host <ipv4-or-cidr-or-range|all|*> [more ...]",
		Short:  "Legacy alias for routeflux firewall set hosts ...",
		Hidden: true,
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

			return printOutput(cmd, opts.jsonOutput, updated, fmt.Sprintf("Host routing enabled for %s", strings.Join(updated.Hosts, ", ")))
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
	case "targets", "bypass", "anti-target", "anti-targets", "hosts", "port", "block-quic", "ipv6":
		if len(args) < 2 {
			return "", nil, fmt.Errorf("firewall %s expects at least one value", args[0])
		}
		option := strings.ToLower(strings.TrimSpace(args[0]))
		if option == "anti-targets" {
			option = "anti-target"
		}
		return option, args[1:], nil
	case "split":
		return "split", args[1:], nil
	default:
		return "targets", args, nil
	}
}

func renderFirewallSettingsText(settings domain.FirewallSettings) string {
	lines := []string{
		fmt.Sprintf("enabled=%t", settings.Enabled),
		fmt.Sprintf("mode=%s", firewallMode(settings)),
		fmt.Sprintf("mode-help=%s", firewallModeHelp(settings)),
		fmt.Sprintf("transparent-port=%d", settings.TransparentPort),
		fmt.Sprintf("default-action=%s", domain.NormalizeFirewallDefaultAction(settings.Split.DefaultAction)),
		fmt.Sprintf("targets=%s", firewallSelectorSummary(settings.Targets)),
		fmt.Sprintf("target-services=%s", strings.Join(settings.Targets.Services, ", ")),
		fmt.Sprintf("target-domains=%s", strings.Join(settings.Targets.Domains, ", ")),
		fmt.Sprintf("target-ips=%s", strings.Join(settings.Targets.CIDRs, ", ")),
		fmt.Sprintf("split-proxy=%s", firewallSelectorSummary(settings.Split.Proxy)),
		fmt.Sprintf("split-bypass=%s", firewallSelectorSummary(settings.Split.Bypass)),
		fmt.Sprintf("split-excluded-sources=%s", strings.Join(settings.Split.ExcludedSources, ", ")),
		fmt.Sprintf("hosts=%s", strings.Join(settings.Hosts, ", ")),
		fmt.Sprintf("block-quic=%t", settings.BlockQUIC),
		fmt.Sprintf("disable-ipv6=%t", settings.DisableIPv6),
	}
	return strings.Join(lines, "\n")
}

func parseFirewallIPv6State(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "disable", "disabled", "off", "true", "1":
		return true, nil
	case "enable", "enabled", "on", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported firewall ipv6 value %q: use disable or enable", raw)
	}
}

func firewallMode(settings domain.FirewallSettings) string {
	if !settings.Enabled {
		return "disabled"
	}

	if firewallLooksLikeBypass(settings) {
		return "bypass"
	}

	return string(domain.NormalizeFirewallMode(settings.Mode))
}

func firewallModeHelp(settings domain.FirewallSettings) string {
	switch firewallMode(settings) {
	case "disabled":
		return "No traffic is being redirected through RouteFlux."
	case "targets":
		return "Only traffic to selected services, domains, or destination IPv4 targets goes through RouteFlux."
	case "bypass":
		return "All traffic goes through RouteFlux except selected direct resources and excluded LAN devices."
	case "split":
		return "Advanced split tunnelling uses explicit proxy, bypass, and excluded-device lists."
	case "hosts":
		return "All traffic from selected LAN devices goes through RouteFlux."
	default:
		return "No traffic is being redirected through RouteFlux."
	}
}

func firewallSelectorSummary(selectors domain.FirewallSelectorSet) string {
	values := make([]string, 0, len(selectors.Services)+len(selectors.Domains)+len(selectors.CIDRs))
	values = append(values, selectors.Services...)
	values = append(values, selectors.Domains...)
	values = append(values, selectors.CIDRs...)
	return strings.Join(values, ", ")
}

func firewallSplitSummary(split domain.FirewallSplitSettings) string {
	parts := make([]string, 0, 4)
	if summary := firewallSelectorSummary(split.Proxy); summary != "" {
		parts = append(parts, fmt.Sprintf("proxy=[%s]", summary))
	}
	if summary := firewallSelectorSummary(split.Bypass); summary != "" {
		parts = append(parts, fmt.Sprintf("bypass=[%s]", summary))
	}
	if len(split.ExcludedSources) > 0 {
		parts = append(parts, fmt.Sprintf("excluded=[%s]", strings.Join(split.ExcludedSources, ", ")))
	}
	parts = append(parts, fmt.Sprintf("default-action=%s", domain.NormalizeFirewallDefaultAction(split.DefaultAction)))
	return strings.Join(parts, "; ")
}

func firewallBypassSummary(split domain.FirewallSplitSettings) string {
	parts := make([]string, 0, 3)
	if summary := firewallSelectorSummary(split.Bypass); summary != "" {
		parts = append(parts, fmt.Sprintf("bypass=[%s]", summary))
	}
	if len(split.ExcludedSources) > 0 {
		parts = append(parts, fmt.Sprintf("excluded=[%s]", strings.Join(split.ExcludedSources, ", ")))
	}
	parts = append(parts, fmt.Sprintf("default-action=%s", domain.NormalizeFirewallDefaultAction(split.DefaultAction)))
	return strings.Join(parts, "; ")
}

func firewallLooksLikeBypass(settings domain.FirewallSettings) bool {
	if !settings.Enabled {
		return false
	}
	if domain.CanonicalFirewallMode(settings) != domain.FirewallModeSplit {
		return false
	}
	return domain.NormalizeFirewallDefaultAction(settings.Split.DefaultAction) == domain.FirewallDefaultActionProxy &&
		!domain.FirewallSelectorSetHasEntries(settings.Split.Proxy)
}

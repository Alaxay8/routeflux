package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func newDNSCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Easy DNS settings for RouteFlux",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Long: strings.TrimSpace(`
DNS controls how RouteFlux resolves public DNS while the proxy runtime is active.
Real DNS modes: system, remote, split, disabled.
Recommended DNS preset (not a mode): routeflux dns default.
For mode-by-mode guidance, use routeflux dns explain.
`),
		Example: strings.TrimSpace(`
routeflux dns get
routeflux dns default
routeflux dns set mode system
`),
	}

	cmd.AddCommand(
		newDNSGetCmd(opts),
		newDNSDefaultCmd(opts),
		newDNSApplyCmd(opts),
		newDNSSetCmd(opts),
		newDNSExplainCmd(opts),
	)

	return cmd
}

func newDNSGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current DNS settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := opts.service.GetSettings()
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, settings.DNS, "")
			}

			return printOutput(cmd, false, nil, renderDNSSettingsText(settings.DNS))
		},
	}
}

func newDNSSetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "set <option> <value>",
		Short: "Change one DNS setting or apply the Recommended DNS preset",
		Long: strings.TrimSpace(`
DNS options:
- default: apply the Recommended DNS preset (preset, not a mode)
- mode: system, remote, split, disabled
- transport: plain, doh
- servers: main DNS servers, separated by commas
- bootstrap: fallback DNS servers used to resolve DNS server hostnames
- direct-domains: domains that stay local in split mode

Mode quick guide:
- system: leave DNS as it is
- remote: send all DNS to the servers you choose
- split: keep local names local and send internet DNS to the servers you choose
- disabled: do not write RouteFlux DNS settings into the Xray config

Recommended start: routeflux dns default
`),
		Example: strings.TrimSpace(`
routeflux dns set default
routeflux dns set mode system
routeflux dns set mode remote
routeflux dns set mode split
routeflux dns set transport doh
routeflux dns set servers "dns.google,1.1.1.1"
`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.EqualFold(args[0], "default") {
				return nil
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.EqualFold(args[0], "default") {
				return applyDefaultDNSProfile(cmd, opts)
			}

			key, err := mapDNSOptionKey(args[0])
			if err != nil {
				return err
			}

			settings, err := opts.service.SetSetting(key, args[1])
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, settings.DNS, "")
			}

			return printOutput(cmd, false, nil, fmt.Sprintf("Updated %s=%s", key, args[1]))
		},
	}
}

func newDNSApplyCmd(opts *rootOptions) *cobra.Command {
	var mode string
	var transport string
	var servers string
	var bootstrap string
	var directDomains string

	cmd := &cobra.Command{
		Use:    "apply",
		Short:  "Replace the full DNS profile in one step",
		Hidden: true,
		Long: strings.TrimSpace(`
Apply a complete DNS profile atomically.

Unlike repeated "dns set" calls, this updates mode, transport, servers, bootstrap,
and split DNS direct-domains together, then reapplies the runtime only once.

This is the safe path for LuCI and scripted profile changes while connected.
`),
		Example: strings.TrimSpace(`
routeflux dns apply --mode split --transport doh --servers "1.1.1.1,1.0.0.1" --bootstrap "" --direct-domains "domain:lan,full:router.lan"
routeflux dns apply --mode system --transport plain --servers "" --bootstrap "" --direct-domains ""
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			required := []string{"mode", "transport", "servers", "bootstrap", "direct-domains"}
			missing := make([]string, 0, len(required))
			for _, name := range required {
				if !cmd.Flags().Changed(name) {
					missing = append(missing, "--"+name)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("dns apply requires %s", strings.Join(missing, ", "))
			}

			parsedMode, err := domain.ParseDNSMode(mode)
			if err != nil {
				return err
			}
			parsedTransport, err := domain.ParseDNSTransport(transport)
			if err != nil {
				return err
			}

			settings, err := opts.service.UpdateDNS(cmd.Context(), domain.DNSSettings{
				Mode:          parsedMode,
				Transport:     parsedTransport,
				Servers:       parseListCSV(servers),
				Bootstrap:     parseListCSV(bootstrap),
				DirectDomains: parseListCSV(directDomains),
			})
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, settings.DNS, "")
			}

			return printOutput(cmd, false, nil, "Applied DNS settings atomically.")
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "DNS mode: system, remote, split, disabled")
	cmd.Flags().StringVar(&transport, "transport", "", "DNS transport: plain, doh")
	cmd.Flags().StringVar(&servers, "servers", "", "Comma-separated upstream DNS servers")
	cmd.Flags().StringVar(&bootstrap, "bootstrap", "", "Comma-separated bootstrap DNS servers")
	cmd.Flags().StringVar(&directDomains, "direct-domains", "", "Comma-separated split DNS direct domains")

	return cmd
}

func newDNSDefaultCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Apply the Recommended DNS preset",
		Long: strings.TrimSpace(`
Apply the Recommended DNS preset in one step.

This is a preset, not a fifth DNS mode.

This sets:
- mode=split
- transport=doh
- servers=1.1.1.1,1.0.0.1
- bootstrap=(empty)
- direct-domains=domain:lan,full:router.lan

On OpenWrt, while a node is connected, this also routes router and LAN public DNS
through the local Xray DNS runtime and returns to system DNS on disconnect.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyDefaultDNSProfile(cmd, opts)
		},
	}
}

func newDNSExplainCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "explain",
		Short: "Explain DNS settings in plain language",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printOutput(cmd, false, nil, strings.TrimSpace(`
DNS modes:
- system: Leave DNS as it is. Your router keeps using its usual DNS.
  Example: you only want proxy routing and your router DNS already works fine.
  Command: routeflux dns set mode system
- remote: Send every DNS request to the DNS servers you choose.
  Example: use Cloudflare or Google DNS for everything.
  Command: routeflux dns set mode remote
- split: Keep local names on the router, but send internet domains to the DNS servers you choose.
  Example: router.lan stays local, google.com goes to Cloudflare.
  Command: routeflux dns set default
- disabled: Do not write RouteFlux DNS settings into the Xray config.
  Example: advanced setup where DNS is managed somewhere else.
  Command: routeflux dns set mode disabled

DNS transports:
- plain: normal DNS, no encryption.
- doh: encrypted DNS over HTTPS.

Other options:
- servers: the main DNS servers RouteFlux should use.
- bootstrap: helper DNS servers used when your main DNS server is written as a hostname, such as dns.google.
- direct-domains: names that should stay on local DNS in split mode.

Recommended DNS preset:
- routeflux dns set default
  Preset, not a fifth mode. Good for most users: local names stay local, public DNS is encrypted, and on OpenWrt the router and LAN DNS follow it while connected.
`))
		},
	}
}

func mapDNSOptionKey(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "mode":
		return "dns.mode", nil
	case "transport":
		return "dns.transport", nil
	case "servers":
		return "dns.servers", nil
	case "bootstrap":
		return "dns.bootstrap", nil
	case "direct-domains", "domains":
		return "dns.direct-domains", nil
	default:
		return "", fmt.Errorf("unsupported dns option %q", raw)
	}
}

func renderDNSSettingsText(dns domain.DNSSettings) string {
	lines := []string{
		fmt.Sprintf("mode=%v", dns.Mode),
		fmt.Sprintf("mode-help=%s", dnsModeHelp(dns.Mode)),
		fmt.Sprintf("transport=%v", dns.Transport),
		fmt.Sprintf("transport-help=%s", dnsTransportHelp(dns.Transport)),
		fmt.Sprintf("servers=%s", strings.Join(dns.Servers, ", ")),
		fmt.Sprintf("bootstrap=%s", strings.Join(dns.Bootstrap, ", ")),
		fmt.Sprintf("direct-domains=%s", strings.Join(dns.DirectDomains, ", ")),
	}

	if isDefaultDNSProfile(dns) {
		lines = append(lines,
			"profile=Recommended DNS preset",
			"profile-help=Recommended DNS preset: encrypted public DNS with local names kept local. On OpenWrt while connected, router and LAN DNS follow this profile.",
		)
	}

	return strings.Join(lines, "\n")
}

func dnsModeHelp(mode domain.DNSMode) string {
	switch mode {
	case domain.DNSModeSystem:
		return "Leave DNS as it is. The router keeps using its usual DNS."
	case domain.DNSModeRemote:
		return "Send every DNS request to the DNS servers you chose. On OpenWrt while connected, router and LAN DNS can follow this too."
	case domain.DNSModeSplit:
		return "Keep local home names on the router, send the rest to your chosen DNS. On OpenWrt while connected, router and LAN public DNS use the local Xray DNS runtime."
	case domain.DNSModeDisabled:
		return "Do not write DNS settings into the Xray config."
	default:
		return "DNS mode is not set."
	}
}

func dnsTransportHelp(transport domain.DNSTransport) string {
	switch transport {
	case domain.DNSTransportPlain:
		return "Normal DNS, no encryption."
	case domain.DNSTransportDoH:
		return "Encrypted DNS over HTTPS."
	case domain.DNSTransportDoT:
		return "Legacy transport value. The current backend does not apply it."
	default:
		return "DNS transport is not set."
	}
}

func isDefaultDNSProfile(dns domain.DNSSettings) bool {
	defaults := domain.DefaultDNSSettings()
	return dns.Mode == defaults.Mode &&
		dns.Transport == defaults.Transport &&
		slices.Equal(dns.Servers, defaults.Servers) &&
		slices.Equal(dns.Bootstrap, defaults.Bootstrap) &&
		slices.Equal(dns.DirectDomains, defaults.DirectDomains)
}

func applyDefaultDNSProfile(cmd *cobra.Command, opts *rootOptions) error {
	settings, err := opts.service.ApplyDefaultDNS(cmd.Context())
	if err != nil {
		return err
	}

	if opts.jsonOutput {
		return printOutput(cmd, true, settings.DNS, "")
	}

	return printOutput(cmd, false, nil, strings.TrimSpace(`
Applied the Recommended DNS preset.

What it does:
- This is a preset, not a fifth DNS mode
- Uses encrypted DNS over HTTPS
- Sends public DNS through Cloudflare
- Keeps home-network names like .lan local
- On OpenWrt while connected, routes router and LAN public DNS through the local Xray DNS runtime

Preset:
- mode=split
- transport=doh
- servers=1.1.1.1, 1.0.0.1
- bootstrap=(empty)
- direct-domains=domain:lan,full:router.lan
`))
}

func parseListCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}

	return out
}

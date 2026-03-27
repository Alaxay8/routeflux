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
		Long: strings.TrimSpace(`
DNS controls how RouteFlux tells Xray to resolve domain names.

Think of it like this:
- mode answers "where should DNS go?"
- transport answers "how should it travel?"

Use this command if you want DNS help in plain language instead of raw settings keys.
`),
		Example: strings.TrimSpace(`
routeflux dns get
routeflux dns explain
routeflux dns default
routeflux dns set default
routeflux dns set mode system
routeflux dns set servers "dns.google,1.1.1.1"
routeflux dns set transport doh
routeflux dns set bootstrap "9.9.9.9"
routeflux dns set direct-domains "domain:lan,full:router.lan"
routeflux dns set mode split
`),
	}

	cmd.AddCommand(
		newDNSGetCmd(opts),
		newDNSDefaultCmd(opts),
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
		Short: "Change one DNS setting or apply the default profile",
		Long: strings.TrimSpace(`
DNS options:
- default: apply the RouteFlux recommended DNS profile in one step
- mode: system, remote, split, disabled
- transport: plain, doh
- servers: main DNS servers, separated by commas
- bootstrap: fallback DNS servers used to resolve DNS server hostnames
- direct-domains: domains that stay local in split mode

Simple meaning:
- default: use the RouteFlux recommended everyday profile
- system: RouteFlux leaves DNS alone
- remote: all DNS goes to the DNS servers you chose
- split: local router/home names stay local, the rest goes to your chosen DNS
- plain: normal DNS
- doh: DNS over HTTPS
`),
		Example: strings.TrimSpace(`
routeflux dns set default
routeflux dns set mode system
routeflux dns set mode remote
routeflux dns set mode split
routeflux dns set transport plain
routeflux dns set transport doh
routeflux dns set servers "dns.google,1.1.1.1"
routeflux dns set bootstrap "9.9.9.9"
routeflux dns set direct-domains "domain:lan,full:router.lan"

If you just want the recommended setup, use:
- routeflux dns default
- routeflux dns set default
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

func newDNSDefaultCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Apply the RouteFlux recommended DNS profile",
		Long: strings.TrimSpace(`
Apply the recommended RouteFlux DNS profile in one step.

This sets:
- mode=split
- transport=doh
- servers=1.1.1.1,1.0.0.1
- bootstrap=(empty)
- direct-domains=domain:lan,full:router.lan
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

Easiest starting point:
- routeflux dns set default
  Good for most users: local names stay local, public DNS is encrypted.
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
			"profile=routeflux-default",
			"profile-help=Encrypted DNS for public domains, local names stay local.",
		)
	}

	return strings.Join(lines, "\n")
}

func dnsModeHelp(mode domain.DNSMode) string {
	switch mode {
	case domain.DNSModeSystem:
		return "Leave DNS as it is. The router keeps using its usual DNS."
	case domain.DNSModeRemote:
		return "Send every DNS request to the DNS servers you chose."
	case domain.DNSModeSplit:
		return "Keep local home names on the router, send the rest to your chosen DNS."
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
Applied the RouteFlux default DNS profile.

What it does:
- Uses encrypted DNS over HTTPS
- Sends public DNS through Cloudflare
- Keeps home-network names like .lan local

Profile:
- mode=split
- transport=doh
- servers=1.1.1.1, 1.0.0.1
- bootstrap=(empty)
- direct-domains=domain:lan,full:router.lan
`))
}

package cli

import (
	"fmt"
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
		Short: "Change one DNS setting",
		Long: strings.TrimSpace(`
DNS options:
- mode: system, remote, split, disabled
- transport: plain, doh, dot
- servers: main DNS servers, separated by commas
- bootstrap: fallback DNS servers used to resolve DNS server hostnames
- direct-domains: domains that stay local in split mode

Simple meaning:
- system: RouteFlux leaves DNS alone
- remote: all DNS goes to the DNS servers you chose
- split: local router/home names stay local, the rest goes to your chosen DNS
- plain: normal DNS
- doh: DNS over HTTPS
- dot: DNS over TLS
`),
		Example: strings.TrimSpace(`
routeflux dns set mode system
routeflux dns set mode remote
routeflux dns set mode split
routeflux dns set transport plain
routeflux dns set transport doh
routeflux dns set servers "dns.google,1.1.1.1"
routeflux dns set bootstrap "9.9.9.9"
routeflux dns set direct-domains "domain:lan,full:router.lan"
`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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

func newDNSExplainCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "explain",
		Short: "Explain DNS settings in plain language",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printOutput(cmd, false, nil, strings.TrimSpace(`
DNS modes:
- system: RouteFlux does not touch DNS settings. Use your router's normal DNS.
- remote: Send all DNS requests to the DNS servers you choose.
- split: Keep local home-network names local, but send the rest to the DNS servers you choose.
- disabled: Do not write a RouteFlux DNS block into the Xray config.

DNS transports:
- plain: regular DNS, not encrypted.
- doh: DNS over HTTPS. This is the working encrypted DNS option right now.
- dot: DNS over TLS. The setting exists, but the current Xray backend in RouteFlux does not apply it yet.

Other options:
- servers: the main DNS servers RouteFlux should use.
- bootstrap: helper DNS servers used when your main DNS server is written as a hostname, such as dns.google.
- direct-domains: local domains that should stay on local DNS in split mode.

Good starting profiles:
- Safe default: mode=system
- Encrypted DNS for everything: mode=remote + transport=doh
- Home network + encrypted public DNS: mode=split + transport=doh + direct-domains=domain:lan,full:router.lan
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
	return fmt.Sprintf(
		"mode=%v\nmode-help=%s\ntransport=%v\ntransport-help=%s\nservers=%s\nbootstrap=%s\ndirect-domains=%s",
		dns.Mode,
		dnsModeHelp(dns.Mode),
		dns.Transport,
		dnsTransportHelp(dns.Transport),
		strings.Join(dns.Servers, ", "),
		strings.Join(dns.Bootstrap, ", "),
		strings.Join(dns.DirectDomains, ", "),
	)
}

func dnsModeHelp(mode domain.DNSMode) string {
	switch mode {
	case domain.DNSModeSystem:
		return "Use your router's normal DNS. RouteFlux does not change it."
	case domain.DNSModeRemote:
		return "Send all DNS requests to the DNS servers you selected."
	case domain.DNSModeSplit:
		return "Keep local home-network names local, but send the rest to your selected DNS."
	case domain.DNSModeDisabled:
		return "Do not write a DNS block into the Xray config."
	default:
		return "DNS mode is not set."
	}
}

func dnsTransportHelp(transport domain.DNSTransport) string {
	switch transport {
	case domain.DNSTransportPlain:
		return "Regular DNS, not encrypted."
	case domain.DNSTransportDoH:
		return "DNS over HTTPS. This is the working encrypted DNS option right now."
	case domain.DNSTransportDoT:
		return "DNS over TLS. The setting exists, but the current backend does not apply it yet."
	default:
		return "DNS transport is not set."
	}
}

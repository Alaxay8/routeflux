package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func newServicesCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Manage target service aliases for firewall targets",
		Long: strings.TrimSpace(`
Services are advanced CLI-only reusable aliases for firewall targets.
Zapret fallback accepts fully qualified domains only.

Built-in presets like youtube or telegram are readonly.
Custom services let you define your own alias once and then reuse it in:
  routeflux firewall set targets <service-name>

Custom services may also include existing built-in or custom service aliases,
so you can build reusable bundles like:
  routeflux services set daily youtube openai telegram
`),
		Example: strings.TrimSpace(`
routeflux services list
routeflux services get youtube
routeflux services set openai openai.com chatgpt.com oaistatic.com
routeflux services set daily youtube openai telegram
routeflux services set telegram-work 91.108.0.0/16 149.154.0.0/16 web.telegram.org
routeflux services delete openai
routeflux firewall set targets openai youtube
`),
	}

	cmd.AddCommand(
		newServicesListCmd(opts),
		newServicesGetCmd(opts),
		newServicesSetCmd(opts),
		newServicesDeleteCmd(opts),
	)

	return cmd
}

func newServicesListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List built-in and custom target services",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := opts.service.ListFirewallTargetServices()
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, services, "")
			}

			return printOutput(cmd, false, nil, renderFirewallTargetServices(services))
		},
	}
}

func newServicesGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show one target service definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := opts.service.GetFirewallTargetService(args[0])
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, service, "")
			}

			return printOutput(cmd, false, nil, renderFirewallTargetService(service))
		},
	}
}

func newServicesSetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <selector...>",
		Short: "Create or update a custom target service",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := opts.service.SetFirewallTargetService(context.Background(), args[0], args[1:])
			if err != nil {
				return err
			}

			message := fmt.Sprintf("Target service %s set to %s", service.Name, strings.Join(serviceSelectors(service), ", "))
			return printOutput(cmd, opts.jsonOutput, service, message)
		},
	}
}

func newServicesDeleteCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a custom target service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := opts.service.GetFirewallTargetService(args[0])
			if err != nil {
				return err
			}
			if entry.ReadOnly {
				return fmt.Errorf("target service %q is readonly and cannot be deleted", entry.Name)
			}

			if err := opts.service.DeleteFirewallTargetService(context.Background(), args[0]); err != nil {
				return err
			}

			return printOutput(cmd, false, nil, fmt.Sprintf("Target service %s deleted", entry.Name))
		},
	}
}

func renderFirewallTargetServices(services []domain.FirewallTargetService) string {
	if len(services) == 0 {
		return "No target services configured."
	}

	blocks := make([]string, 0, len(services))
	for _, service := range services {
		blocks = append(blocks, renderFirewallTargetService(service))
	}
	return strings.Join(blocks, "\n\n")
}

func renderFirewallTargetService(service domain.FirewallTargetService) string {
	return fmt.Sprintf(
		"name=%s\nsource=%s\nreadonly=%t\nservices=%s\ndomains=%s\ncidrs=%s",
		service.Name,
		service.Source,
		service.ReadOnly,
		strings.Join(service.Services, ", "),
		strings.Join(service.Domains, ", "),
		strings.Join(service.CIDRs, ", "),
	)
}

func serviceSelectors(service domain.FirewallTargetService) []string {
	values := make([]string, 0, len(service.Services)+len(service.Domains)+len(service.CIDRs))
	values = append(values, service.Services...)
	values = append(values, service.Domains...)
	values = append(values, service.CIDRs...)
	return values
}

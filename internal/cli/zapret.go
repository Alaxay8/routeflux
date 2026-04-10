package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/domain"
)

func newZapretCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zapret",
		Short: "Manage Zapret fallback settings and status",
	}

	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Update Zapret fallback settings",
	}
	setCmd.AddCommand(
		newZapretSetEnabledCmd(opts),
		newZapretSetSelectorsCmd(opts),
		newZapretSetFailbackThresholdCmd(opts),
	)

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Force Zapret test mode on or off",
	}
	testCmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Force the router into Zapret test mode",
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := opts.service.StartZapretTest(context.Background())
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, status, renderZapretStatus(status))
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop Zapret test mode and restore the previous route",
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := opts.service.StopZapretTest(context.Background())
				if err != nil {
					return err
				}
				return printOutput(cmd, opts.jsonOutput, status, renderZapretStatus(status))
			},
		},
	)

	cmd.AddCommand(
		&cobra.Command{
			Use:   "get",
			Short: "Show Zapret fallback settings",
			RunE: func(cmd *cobra.Command, args []string) error {
				settings, err := opts.service.GetZapretSettings()
				if err != nil {
					return err
				}
				if opts.jsonOutput {
					return printOutput(cmd, true, settings, "")
				}

				return printOutput(cmd, false, nil, renderZapretSettings(settings))
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show Zapret runtime status",
			RunE: func(cmd *cobra.Command, args []string) error {
				status, err := opts.service.GetZapretStatus(context.Background())
				if err != nil && opts.jsonOutput {
					return printOutput(cmd, true, status, "")
				}
				if err != nil {
					return printOutput(cmd, false, nil, renderZapretStatus(status))
				}
				return printOutput(cmd, opts.jsonOutput, status, renderZapretStatus(status))
			},
		},
		setCmd,
		testCmd,
	)

	return cmd
}

func newZapretSetEnabledCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "enabled <true|false>",
		Short: "Enable or disable Zapret fallback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			enabled, err := strconv.ParseBool(strings.TrimSpace(args[0]))
			if err != nil {
				return fmt.Errorf("parse enabled value %q: %w", args[0], err)
			}
			settings, err := opts.service.SetZapretEnabled(context.Background(), enabled)
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Zapret fallback enabled=%t", settings.Enabled))
		},
	}
}

func newZapretSetSelectorsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "selectors [selector...]",
		Short: "Set Zapret selectors to domains and IPv4 selectors",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selectors := args
			settings, err := opts.service.SetZapretSelectors(context.Background(), selectors)
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Zapret selectors updated: %s", zapretSelectorSummary(settings.Selectors)))
		},
	}
}

func newZapretSetFailbackThresholdCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "failback-success-threshold <n>",
		Short: "Set how many healthy cycles are required before proxy failback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threshold, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				return fmt.Errorf("parse failback threshold %q: %w", args[0], err)
			}
			settings, err := opts.service.SetZapretFailbackSuccessThreshold(threshold)
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, settings, fmt.Sprintf("Zapret failback-success-threshold=%d", settings.FailbackSuccessThreshold))
		},
	}
}

func renderZapretSettings(settings domain.ZapretSettings) string {
	return fmt.Sprintf(
		"enabled=%t\nselectors=%s\ndomains=%s\ncidrs=%s\nfailback-success-threshold=%d",
		settings.Enabled,
		zapretSelectorSummary(settings.Selectors),
		strings.Join(settings.Selectors.Domains, ", "),
		strings.Join(settings.Selectors.CIDRs, ", "),
		settings.FailbackSuccessThreshold,
	)
}

func renderZapretStatus(status domain.ZapretStatus) string {
	return fmt.Sprintf(
		"installed=%t\nmanaged=%t\nactive=%t\nservice-active=%t\ntest-active=%t\nservice-state=%s\nlast-reason=%s",
		status.Installed,
		status.Managed,
		status.Active,
		status.ServiceActive,
		status.TestActive,
		status.ServiceState,
		status.LastReason,
	)
}

func zapretSelectorSummary(selectors domain.FirewallSelectorSet) string {
	values := make([]string, 0, len(selectors.Domains)+len(selectors.CIDRs))
	values = append(values, selectors.Domains...)
	values = append(values, selectors.CIDRs...)
	return strings.Join(values, ", ")
}

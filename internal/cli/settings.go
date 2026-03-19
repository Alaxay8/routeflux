package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSettingsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Get or update RouteFlux settings",
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
					"refresh-interval=%s\nhealth-check-interval=%s\nswitch-cooldown=%s\nlatency-threshold=%s\nauto-mode=%t\nmode=%s\nlog-level=%s",
					settings.RefreshInterval,
					settings.HealthCheckInterval,
					settings.SwitchCooldown,
					settings.LatencyThreshold,
					settings.AutoMode,
					settings.Mode,
					settings.LogLevel,
				)
				return printOutput(cmd, false, nil, text)
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Update a setting",
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

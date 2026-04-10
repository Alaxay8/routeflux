package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRefreshCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var all bool

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh subscriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				subscriptions, err := opts.service.RefreshAll(context.Background())
				if err != nil {
					return err
				}
				if opts.jsonOutput {
					return printOutput(cmd, true, subscriptions, "")
				}
				var lines []string
				for _, sub := range subscriptions {
					lines = append(lines, fmt.Sprintf("Refreshed %s (%d nodes)", sub.ID, len(sub.Nodes)))
				}
				return printOutput(cmd, false, nil, strings.Join(lines, "\n"))
			}

			if strings.TrimSpace(subscriptionID) == "" {
				return fmt.Errorf("use --all or --subscription <id-or-prefix>")
			}

			sub, err := opts.service.RefreshSubscription(context.Background(), subscriptionID)
			if err != nil {
				return err
			}
			return printOutput(cmd, opts.jsonOutput, sub, fmt.Sprintf("Refreshed %s (%d nodes)", sub.ID, len(sub.Nodes)))
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription", "", "Subscription ID or unique prefix")
	cmd.Flags().BoolVar(&all, "all", false, "Refresh all subscriptions")

	return cmd
}

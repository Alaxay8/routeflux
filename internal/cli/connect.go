package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newConnectCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string
	var auto bool

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to a specific node or enable auto mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			if auto {
				node, err := opts.service.ConnectAuto(context.Background(), subscriptionID)
				if err != nil {
					return err
				}
				if node.ID == "" {
					return printOutput(
						cmd,
						opts.jsonOutput,
						map[string]string{"subscription": subscriptionID, "transport": "zapret"},
						fmt.Sprintf("Auto mode enabled Zapret fallback for %s", subscriptionID),
					)
				}
				return printOutput(cmd, opts.jsonOutput, node, fmt.Sprintf("Auto selected %s (%s)", node.DisplayName(), node.ID))
			}

			if err := opts.service.ConnectManual(context.Background(), subscriptionID, nodeID); err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, map[string]string{"subscription": subscriptionID, "node": nodeID}, fmt.Sprintf("Connected %s/%s", subscriptionID, nodeID))
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription", "", "Subscription ID or unique prefix")
	cmd.Flags().StringVar(&nodeID, "node", "", "Node ID")
	cmd.Flags().BoolVar(&auto, "auto", false, "Automatically select the best node")
	_ = cmd.MarkFlagRequired("subscription")
	return cmd
}

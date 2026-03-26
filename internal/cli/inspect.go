package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newInspectCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "inspect",
		Short:  "Internal inspection helpers for LuCI",
		Hidden: true,
	}

	cmd.AddCommand(
		newInspectXrayCmd(opts),
		newInspectSpeedCmd(opts),
	)

	return cmd
}

func newInspectXrayCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string

	cmd := &cobra.Command{
		Use:    "xray",
		Short:  "Render generated Xray JSON for a node",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rendered, err := opts.service.InspectXrayConfig(subscriptionID, nodeID)
			if err != nil {
				return err
			}

			output := string(rendered)
			if !strings.HasSuffix(output, "\n") {
				output += "\n"
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output)
			return err
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription", "", "Subscription ID")
	cmd.Flags().StringVar(&nodeID, "node", "", "Node ID")
	_ = cmd.MarkFlagRequired("subscription")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

func newInspectSpeedCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string

	cmd := &cobra.Command{
		Use:    "speed",
		Short:  "Run isolated speed test for a node",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := opts.service.InspectSpeed(context.Background(), subscriptionID, nodeID)
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, result, "")
			}

			return printOutput(
				cmd,
				false,
				nil,
				fmt.Sprintf(
					"node=%s latency_ms=%.2f download_mbps=%.2f upload_mbps=%.2f",
					result.NodeName,
					result.LatencyMS,
					result.DownloadMbps,
					result.UploadMbps,
				),
			)
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription", "", "Subscription ID")
	cmd.Flags().StringVar(&nodeID, "node", "", "Node ID")
	_ = cmd.MarkFlagRequired("subscription")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

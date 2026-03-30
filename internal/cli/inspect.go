package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/pkg/api"
)

func newInspectCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "inspect",
		Short:        "Internal inspection helpers for LuCI",
		Hidden:       true,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newInspectXrayCmd(opts),
		newInspectXraySafeCmd(opts),
		newInspectSpeedCmd(opts),
	)

	return cmd
}

func newInspectXrayCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string

	cmd := &cobra.Command{
		Use:          "xray",
		Short:        "Render generated Xray JSON for a node",
		Hidden:       true,
		SilenceUsage: true,
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

func newInspectXraySafeCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string

	cmd := &cobra.Command{
		Use:          "xray-safe",
		Short:        "Render generated Xray JSON with secrets redacted",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rendered, err := opts.service.InspectXrayConfig(subscriptionID, nodeID)
			if err != nil {
				return err
			}

			metadata, err := inspectPreviewMetadata(opts, subscriptionID, nodeID)
			if err != nil {
				return err
			}

			safePreview, err := api.RedactXrayPreview(rendered, metadata)
			if err != nil {
				return err
			}

			output := string(safePreview)
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

func inspectPreviewMetadata(opts *rootOptions, subscriptionID, nodeID string) (*api.XrayPreviewMetadata, error) {
	nodes, err := opts.service.ListNodes(subscriptionID)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.ID != nodeID {
			continue
		}

		remark := strings.TrimSpace(node.Remark)
		if remark == "" {
			remark = strings.TrimSpace(node.Name)
		}

		return &api.XrayPreviewMetadata{
			Remark:     remark,
			ServerName: strings.TrimSpace(node.ServerName),
		}, nil
	}

	return nil, fmt.Errorf("node %q not found in subscription %q", nodeID, subscriptionID)
}

func newInspectSpeedCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string
	var nodeID string

	cmd := &cobra.Command{
		Use:          "speed",
		Short:        "Run isolated speed test for a node",
		Hidden:       true,
		SilenceUsage: true,
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

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/pkg/api"
)

func newListCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List subscriptions or nodes",
	}

	cmd.AddCommand(
		newListSubscriptionsCmd(opts),
		newListNodesCmd(opts),
	)

	return cmd
}

func newListSubscriptionsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "subscriptions",
		Short: "List imported subscriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			subscriptions, err := opts.service.ListSubscriptions()
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, api.SubscriptionSummariesFromDomain(subscriptions, true), "")
			}

			if len(subscriptions) == 0 {
				return printOutput(cmd, false, nil, "No subscriptions imported")
			}

			var lines []string
			for _, sub := range subscriptions {
				lines = append(lines, fmt.Sprintf("%s  %s  nodes=%d  updated=%s  status=%s", sub.ID, sub.DisplayName, len(sub.Nodes), formatLocalTimestamp(sub.LastUpdatedAt), sub.ParserStatus))
			}

			return printOutput(cmd, false, nil, strings.Join(lines, "\n"))
		},
	}
}

func newListNodesCmd(opts *rootOptions) *cobra.Command {
	var subscriptionID string

	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "List nodes for a subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			nodes, err := opts.service.ListNodes(subscriptionID)
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, api.NodeSummariesFromDomain(nodes), "")
			}

			if len(nodes) == 0 {
				return printOutput(cmd, false, nil, "No nodes available")
			}

			var lines []string
			for _, node := range nodes {
				lines = append(lines, fmt.Sprintf("%s  %s  %s  %s:%d  transport=%s security=%s", node.ID, node.DisplayName(), node.Protocol, node.Address, node.Port, node.Transport, node.Security))
			}

			return printOutput(cmd, false, nil, strings.Join(lines, "\n"))
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription", "", "Subscription ID")
	_ = cmd.MarkFlagRequired("subscription")
	return cmd
}

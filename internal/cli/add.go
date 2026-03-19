package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/app"
)

func newAddCmd(opts *rootOptions) *cobra.Command {
	var rawURL string
	var rawPayload string
	var name string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a subscription from URL or raw payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			sub, err := opts.service.AddSubscription(context.Background(), app.AddSubscriptionRequest{
				URL:  rawURL,
				Raw:  rawPayload,
				Name: name,
			})
			if err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, sub, fmt.Sprintf("Added %s (%s) with %d node(s)", sub.DisplayName, sub.ID, len(sub.Nodes)))
		},
	}

	cmd.Flags().StringVar(&rawURL, "url", "", "Subscription URL")
	cmd.Flags().StringVar(&rawPayload, "raw", "", "Raw subscription payload")
	cmd.Flags().StringVar(&name, "name", "", "Optional display name")

	return cmd
}

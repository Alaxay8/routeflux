package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRemoveCmd(opts *rootOptions) *cobra.Command {
	var removeAll bool

	cmd := &cobra.Command{
		Use:   "remove <subscription-id>|all",
		Short: "Remove imported subscriptions",
		Args: func(cmd *cobra.Command, args []string) error {
			if removeAll {
				if len(args) != 0 {
					return fmt.Errorf("--all does not accept a subscription id")
				}

				return nil
			}

			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if removeAll || args[0] == "all" {
				removed, err := opts.service.RemoveAllSubscriptions(context.Background())
				if err != nil {
					return err
				}

				return printOutput(
					cmd,
					opts.jsonOutput,
					map[string]any{"removed": "all", "count": removed},
					fmt.Sprintf("Removed %d subscriptions", removed),
				)
			}

			id := args[0]
			if err := opts.service.RemoveSubscription(context.Background(), id); err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, map[string]string{"removed": id}, fmt.Sprintf("Removed subscription %s", id))
		},
	}

	cmd.Flags().BoolVar(&removeAll, "all", false, "Remove all imported subscriptions")

	return cmd
}

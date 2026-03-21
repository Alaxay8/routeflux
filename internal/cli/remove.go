package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRemoveCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <subscription-id>",
		Short: "Remove an imported subscription",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := opts.service.RemoveSubscription(context.Background(), id); err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, map[string]string{"removed": id}, fmt.Sprintf("Removed subscription %s", id))
		},
	}
}

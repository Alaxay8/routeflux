package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newDisconnectCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect the current active node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.service.Disconnect(context.Background()); err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, map[string]bool{"disconnected": true}, "Disconnected")
		},
	}
}

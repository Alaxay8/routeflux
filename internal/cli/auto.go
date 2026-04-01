package cli

import (
	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/pkg/api"
)

func newStatusCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current RouteFlux status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := opts.service.Status()
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, api.StatusResponseFromSnapshot(status), "")
			}

			text := "Disconnected"
			if status.State.Connected {
				text = "Connected"
			}

			if status.ActiveSubscription != nil && status.ActiveNode != nil {
				text += "\nSubscription: " + status.ActiveSubscription.DisplayName
				text += "\nNode: " + status.ActiveNode.DisplayName()
				text += "\nMode: " + string(status.State.Mode)
			}

			return printOutput(cmd, false, nil, text)
		},
	}
}

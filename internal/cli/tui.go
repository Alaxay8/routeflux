package cli

import (
	"github.com/spf13/cobra"

	routefluxtui "github.com/Alaxay8/routeflux/internal/tui"
)

func newTUICmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive terminal UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return routefluxtui.Run(opts.service)
		},
	}
}

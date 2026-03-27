package cli

import (
	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/buildinfo"
)

func newVersionCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print RouteFlux version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printVersion(cmd, opts.jsonOutput)
		},
	}
}

func printVersion(cmd *cobra.Command, jsonOutput bool) error {
	return printOutput(cmd, jsonOutput, buildinfo.Current(), buildinfo.Text())
}

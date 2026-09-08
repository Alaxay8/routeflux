package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newRestartCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the RouteFlux service and clear LuCI caches",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Clear LuCI cache
			matches, _ := filepath.Glob("/tmp/luci-indexcache*")
			for _, m := range matches {
				_ = os.Remove(m)
			}
			_ = os.RemoveAll("/tmp/luci-modulecache")

			// Remove lock files
			_ = os.Remove(filepath.Join(opts.rootDir, ".routeflux.lock"))
			_ = os.Remove(filepath.Join(opts.rootDir, "speedtest.lock"))

			// 2. Restart the service
			initCmd := exec.Command("/etc/init.d/routeflux", "restart")
			if err := initCmd.Start(); err != nil {
				return printOutput(
					cmd,
					opts.jsonOutput,
					map[string]any{"success": false, "error": err.Error()},
					fmt.Sprintf("Cleared caches but failed to restart service: %v", err),
				)
			}

			return printOutput(
				cmd,
				opts.jsonOutput,
				map[string]any{"success": true},
				"RouteFlux service restarted and LuCI cache cleared successfully",
			)
		},
	}
}

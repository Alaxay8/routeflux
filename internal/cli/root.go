package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/backend/xray"
	"github.com/Alaxay8/routeflux/internal/platform/openwrt"
	"github.com/Alaxay8/routeflux/internal/probe"
	"github.com/Alaxay8/routeflux/internal/store"
)

type rootOptions struct {
	rootDir    string
	jsonOutput bool
	service    *app.Service
}

// Execute runs the RouteFlux CLI.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "routeflux",
		Short: "RouteFlux manages subscription-based Xray routing on OpenWrt",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.initService()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.rootDir, "root", "", "RouteFlux state directory")
	cmd.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "Output JSON")

	cmd.AddCommand(
		newAddCmd(opts),
		newDaemonCmd(opts),
		newDiagnosticsCmd(opts),
		newDNSCmd(opts),
		newFirewallCmd(opts),
		newListCmd(opts),
		newLogsCmd(opts),
		newRemoveCmd(opts),
		newRefreshCmd(opts),
		newConnectCmd(opts),
		newDisconnectCmd(opts),
		newStatusCmd(opts),
		newSettingsCmd(opts),
		newTUICmd(opts),
	)

	return cmd
}

func (o *rootOptions) initService() error {
	if o.service != nil {
		return nil
	}

	root := o.rootDir
	if root == "" {
		root = openwrt.RootDir()
	}

	fileStore := store.NewFileStore(root)
	controller := openwrt.NewXrayController()
	firewall := openwrt.NewFirewallManager()
	var runtimeBackend backend.Backend = xray.NewRuntimeBackend(openwrt.XrayConfigPath(), controller)
	o.service = app.NewService(app.Dependencies{
		Store:      fileStore,
		Backend:    runtimeBackend,
		Firewaller: firewall,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		Checker:    probe.TCPChecker{Timeout: 5 * time.Second},
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})

	return nil
}

func printOutput(cmd *cobra.Command, jsonOutput bool, value any, text string) error {
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}

	_, err := fmt.Fprintln(cmd.OutOrStdout(), text)
	return err
}

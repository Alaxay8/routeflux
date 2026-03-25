package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	configPath := openwrt.XrayConfigPath()
	if o.rootDir != "" && !openwrt.IsOpenWrt() && os.Getenv("ROUTEFLUX_XRAY_CONFIG") == "" {
		configPath = filepath.Join(root, "xray-config.json")
	}

	bootstrapLogger := newLogger("info")
	fileStore := store.NewFileStore(root).WithLogger(bootstrapLogger)
	logLevel := "info"
	if settings, err := fileStore.LoadSettings(); err == nil {
		logLevel = settings.LogLevel
	} else {
		bootstrapLogger.Warn("load settings for logger level", "path", filepath.Join(root, "settings.json"), "error", err.Error())
	}
	logger := newLogger(logLevel)
	fileStore.WithLogger(logger)
	controller := openwrt.NewXrayController()
	firewall := openwrt.NewFirewallManager()
	var runtimeBackend backend.Backend = xray.NewRuntimeBackend(configPath, controller).WithLogger(logger)
	o.service = app.NewService(app.Dependencies{
		Store:      fileStore,
		Backend:    runtimeBackend,
		Firewaller: firewall,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		Checker:    probe.TCPChecker{Timeout: 5 * time.Second},
		Logger:     logger,
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

func newLogger(rawLevel string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseSlogLevel(rawLevel)}))
}

func parseSlogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

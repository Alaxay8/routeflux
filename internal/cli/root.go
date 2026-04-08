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
	"github.com/Alaxay8/routeflux/internal/speedtest"
	"github.com/Alaxay8/routeflux/internal/store"
)

type rootOptions struct {
	rootDir     string
	jsonOutput  bool
	showVersion bool
	runUpgrade  bool
	service     *app.Service
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.showVersion && opts.runUpgrade {
				return fmt.Errorf("--version and --upgrade cannot be used together")
			}
			if opts.showVersion {
				return printVersion(cmd, opts.jsonOutput)
			}
			if opts.runUpgrade {
				return runUpgrade(cmd, opts.jsonOutput)
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.showVersion || opts.runUpgrade || cmd.Name() == "version" || cmd.Name() == "help" {
				return nil
			}
			return opts.initService()
		},
	}

	cmd.PersistentFlags().StringVar(&opts.rootDir, "root", "", "RouteFlux state directory")
	cmd.PersistentFlags().BoolVar(&opts.jsonOutput, "json", false, "Output JSON")
	cmd.Flags().BoolVar(&opts.showVersion, "version", false, "Print RouteFlux version")
	cmd.Flags().BoolVar(&opts.runUpgrade, "upgrade", false, "Download and install the latest RouteFlux release")

	cmd.AddCommand(
		newAddCmd(opts),
		newDaemonCmd(opts),
		newDiagnosticsCmd(opts),
		newDNSCmd(opts),
		newFirewallCmd(opts),
		newListCmd(opts),
		newLogsCmd(opts),
		newInspectCmd(opts),
		newRemoveCmd(opts),
		newRefreshCmd(opts),
		newConnectCmd(opts),
		newDisconnectCmd(opts),
		newServicesCmd(opts),
		newStatusCmd(opts),
		newSettingsCmd(opts),
		newZapretCmd(opts),
		newTUICmd(opts),
		newVersionCmd(opts),
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
	if err := fileStore.HardenSecretPermissions(configPath); err != nil {
		bootstrapLogger.Warn("harden secret storage permissions", "root", root, "config_path", configPath, "error", err.Error())
	}
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
	ipv6Manager := openwrt.NewIPv6Manager()
	zapretManager := openwrt.NewZapretManager()
	var dnsManager app.DNSManager
	if openwrt.IsOpenWrt() {
		manager := openwrt.NewDNSRuntimeManager()
		dnsManager = manager
	}
	var runtimeBackend backend.Backend = xray.NewRuntimeBackend(configPath, controller).WithLogger(logger)
	o.service = app.NewService(app.Dependencies{
		Store:              fileStore,
		Backend:            runtimeBackend,
		DNSManager:         dnsManager,
		Firewaller:         firewall,
		IPv6Manager:        ipv6Manager,
		ZapretManager:      zapretManager,
		HTTPClient:         &http.Client{Timeout: 20 * time.Second},
		Checker:            probe.TCPChecker{Timeout: 5 * time.Second},
		RuntimeEgressProbe: openwrt.IsOpenWrt(),
		SpeedTester: speedtest.Runner{
			LockPath:   filepath.Join(root, "speedtest.lock"),
			BinaryPath: xray.BinaryPath(),
		},
		Logger: logger,
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

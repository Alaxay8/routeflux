package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/backend"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/platform/openwrt"
	"github.com/Alaxay8/routeflux/pkg/api"
)

const routefluxBinaryPath = "/usr/bin/routeflux"

type diagnosticsSnapshot struct {
	Status                api.StatusResponse    `json:"status"`
	Runtime               backend.RuntimeStatus `json:"runtime"`
	RuntimeError          string                `json:"runtime_error,omitempty"`
	TransparentQUICPolicy string                `json:"transparent_quic_policy"`
	IPv6                  diagnosticsIPv6Status `json:"ipv6"`
	Files                 diagnosticsFiles      `json:"files"`
}

type diagnosticsIPv6Status struct {
	Available          bool     `json:"available"`
	ConfiguredDisabled bool     `json:"configured_disabled"`
	ConfigPath         string   `json:"config_path,omitempty"`
	FailState          bool     `json:"fail_state"`
	PersistentDisabled bool     `json:"persistent_disabled"`
	RuntimeDisabled    bool     `json:"runtime_disabled"`
	EnabledInterfaces  []string `json:"enabled_interfaces,omitempty"`
	Message            string   `json:"message,omitempty"`
	Error              string   `json:"error,omitempty"`
}

type diagnosticsFiles struct {
	RoutefluxBinary   diagnosticsPathStatus `json:"routeflux_binary"`
	RoutefluxRoot     diagnosticsPathStatus `json:"routeflux_root"`
	SubscriptionsFile diagnosticsPathStatus `json:"subscriptions_file"`
	SettingsFile      diagnosticsPathStatus `json:"settings_file"`
	StateFile         diagnosticsPathStatus `json:"state_file"`
	XrayConfig        diagnosticsPathStatus `json:"xray_config"`
	XrayService       diagnosticsPathStatus `json:"xray_service"`
	NFTBinary         diagnosticsPathStatus `json:"nft_binary"`
	FirewallRules     diagnosticsPathStatus `json:"firewall_rules"`
}

type diagnosticsPathStatus struct {
	Path          string `json:"path"`
	Exists        bool   `json:"exists"`
	Directory     bool   `json:"directory"`
	Executable    bool   `json:"executable"`
	IsSymlink     bool   `json:"is_symlink"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
	Mode          string `json:"mode,omitempty"`
	ModifiedAt    string `json:"modified_at,omitempty"`
	Error         string `json:"error,omitempty"`
}

func newDiagnosticsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "diagnostics",
		Short: "Show RouteFlux runtime and file diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot, err := buildDiagnosticsSnapshot(context.Background(), opts)
			if err != nil {
				return err
			}

			if opts.jsonOutput {
				return printOutput(cmd, true, snapshot, "")
			}

			return printOutput(cmd, false, nil, renderDiagnosticsText(snapshot))
		},
	}
}

func buildDiagnosticsSnapshot(ctx context.Context, opts *rootOptions) (diagnosticsSnapshot, error) {
	status, err := opts.service.Status()
	if err != nil {
		return diagnosticsSnapshot{}, err
	}

	runtimeStatus, runtimeErr := opts.service.RuntimeStatus(ctx)
	ipv6Status, ipv6Err := opts.service.IPv6Status(ctx)

	rootDir := opts.rootDir
	if rootDir == "" {
		rootDir = openwrt.RootDir()
	}

	snapshot := diagnosticsSnapshot{
		Status:                api.StatusResponseFromSnapshot(status),
		Runtime:               runtimeStatus,
		TransparentQUICPolicy: diagnosticsTransparentQUICPolicy(status.Settings.Firewall, status.ActiveNode),
		IPv6:                  buildDiagnosticsIPv6Status(status.Settings.Firewall, ipv6Status),
		Files: diagnosticsFiles{
			RoutefluxBinary:   inspectPath(routefluxBinaryPath),
			RoutefluxRoot:     inspectPath(rootDir),
			SubscriptionsFile: inspectPath(filepath.Join(rootDir, "subscriptions.json")),
			SettingsFile:      inspectPath(filepath.Join(rootDir, "settings.json")),
			StateFile:         inspectPath(filepath.Join(rootDir, "state.json")),
			XrayConfig:        inspectPath(openwrt.XrayConfigPath()),
			XrayService:       inspectPath(openwrt.XrayServicePath()),
			NFTBinary:         inspectPath("/usr/sbin/nft"),
			FirewallRules:     inspectPath(openwrt.FirewallRulesPath()),
		},
	}

	if runtimeErr != nil {
		snapshot.RuntimeError = runtimeErr.Error()
	}
	if ipv6Err != nil {
		snapshot.IPv6.Error = ipv6Err.Error()
		if snapshot.IPv6.Message == "" {
			snapshot.IPv6.Message = ipv6Err.Error()
		}
		if snapshot.IPv6.ConfigPath == "" {
			snapshot.IPv6.ConfigPath = openwrt.IPv6SysctlConfigPath()
		}
	}

	return snapshot, nil
}

func inspectPath(path string) diagnosticsPathStatus {
	status := diagnosticsPathStatus{Path: path}

	info, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			status.Error = err.Error()
		}
		return status
	}

	status.Exists = true
	status.IsSymlink = info.Mode()&os.ModeSymlink != 0
	if status.IsSymlink {
		if target, err := os.Readlink(path); err == nil {
			status.SymlinkTarget = target
		} else {
			status.Error = err.Error()
		}
	}

	statInfo, err := os.Stat(path)
	if err != nil {
		status.Error = err.Error()
		status.Mode = info.Mode().String()
		status.Directory = info.IsDir()
		return status
	}

	status.Directory = statInfo.IsDir()
	status.Executable = statInfo.Mode().Perm()&0o111 != 0 && !status.Directory
	status.Mode = statInfo.Mode().String()
	status.ModifiedAt = statInfo.ModTime().UTC().Format(time.RFC3339)

	return status
}

func renderDiagnosticsText(snapshot diagnosticsSnapshot) string {
	lines := []string{
		fmt.Sprintf("connected=%t", snapshot.Status.State.Connected),
		fmt.Sprintf("mode=%s", snapshot.Status.State.Mode),
		fmt.Sprintf("transparent-quic-policy=%s", snapshot.TransparentQUICPolicy),
		fmt.Sprintf("ipv6-available=%t", snapshot.IPv6.Available),
		fmt.Sprintf("ipv6-configured-disabled=%t", snapshot.IPv6.ConfiguredDisabled),
		fmt.Sprintf("ipv6-fail-state=%t", snapshot.IPv6.FailState),
		fmt.Sprintf("ipv6-runtime-disabled=%t", snapshot.IPv6.RuntimeDisabled),
		fmt.Sprintf("ipv6-persistent-disabled=%t", snapshot.IPv6.PersistentDisabled),
		fmt.Sprintf("ipv6-enabled-interfaces=%s", strings.Join(snapshot.IPv6.EnabledInterfaces, ", ")),
		fmt.Sprintf("ipv6-config-path=%s", snapshot.IPv6.ConfigPath),
		fmt.Sprintf("ipv6-message=%s", snapshot.IPv6.Message),
		fmt.Sprintf("ipv6-error=%s", snapshot.IPv6.Error),
		fmt.Sprintf("backend-running=%t", snapshot.Runtime.Running),
		fmt.Sprintf("backend-service-state=%s", snapshot.Runtime.ServiceState),
		fmt.Sprintf("backend-config=%s", snapshot.Runtime.ConfigPath),
		fmt.Sprintf("backend-error=%s", snapshot.RuntimeError),
		fmt.Sprintf("last-success=%s", formatLocalTimestamp(snapshot.Status.State.LastSuccessAt)),
		fmt.Sprintf("last-failure=%s", snapshot.Status.State.LastFailureReason),
		describeDiagnosticFile("routeflux-binary", snapshot.Files.RoutefluxBinary),
		describeDiagnosticFile("routeflux-root", snapshot.Files.RoutefluxRoot),
		describeDiagnosticFile("subscriptions-file", snapshot.Files.SubscriptionsFile),
		describeDiagnosticFile("settings-file", snapshot.Files.SettingsFile),
		describeDiagnosticFile("state-file", snapshot.Files.StateFile),
		describeDiagnosticFile("xray-config", snapshot.Files.XrayConfig),
		describeDiagnosticFile("xray-service", snapshot.Files.XrayService),
		describeDiagnosticFile("nft-binary", snapshot.Files.NFTBinary),
		describeDiagnosticFile("firewall-rules", snapshot.Files.FirewallRules),
	}

	return strings.Join(lines, "\n")
}

func diagnosticsTransparentQUICPolicy(settings domain.FirewallSettings, activeNode *domain.Node) string {
	if !domain.FirewallRoutingEnabled(settings) {
		return "disabled"
	}
	if settings.BlockQUIC {
		return "blocked"
	}
	if domain.EffectiveTransparentBlockQUIC(settings, activeNode) {
		return "blocked-incompatible-node"
	}
	return "proxied"
}

func buildDiagnosticsIPv6Status(settings domain.FirewallSettings, status domain.IPv6Status) diagnosticsIPv6Status {
	diagnostics := diagnosticsIPv6Status{
		Available:          status.Available,
		ConfiguredDisabled: settings.DisableIPv6,
		ConfigPath:         status.ConfigPath,
		PersistentDisabled: status.PersistentDisabled,
		RuntimeDisabled:    status.RuntimeDisabled,
		EnabledInterfaces:  append([]string(nil), status.EnabledInterfaces...),
	}

	routingEnabled := domain.FirewallRoutingEnabled(settings)
	configMismatch := settings.DisableIPv6 && (!status.PersistentDisabled || !status.RuntimeDisabled)
	bypassRisk := routingEnabled && status.Available && !status.RuntimeDisabled

	diagnostics.FailState = configMismatch || bypassRisk

	switch {
	case settings.DisableIPv6 && status.Available && (!status.PersistentDisabled || !status.RuntimeDisabled):
		if len(status.EnabledInterfaces) > 0 {
			diagnostics.Message = fmt.Sprintf(
				"RouteFlux is configured to disable IPv6, but IPv6 is still enabled on %s. Transparent routing does not intercept IPv6 traffic.",
				strings.Join(status.EnabledInterfaces, ", "),
			)
			return diagnostics
		}
		if !status.PersistentDisabled {
			diagnostics.Message = "RouteFlux is configured to disable IPv6, but the persistent IPv6 sysctl file is missing."
			return diagnostics
		}
		diagnostics.Message = "RouteFlux is configured to disable IPv6, but runtime IPv6 is still active. Transparent routing does not intercept IPv6 traffic."
		return diagnostics
	case bypassRisk:
		diagnostics.Message = "Transparent routing does not intercept IPv6 traffic. Disable IPv6 in RouteFlux to prevent bypass."
	case settings.DisableIPv6 && status.Available && status.PersistentDisabled && status.RuntimeDisabled:
		diagnostics.Message = "IPv6 is disabled by RouteFlux."
	case status.Available && !status.RuntimeDisabled:
		diagnostics.Message = "IPv6 remains enabled on the router."
	}

	return diagnostics
}

func describeDiagnosticFile(label string, status diagnosticsPathStatus) string {
	parts := []string{
		fmt.Sprintf("%s=%s", label, status.Path),
		fmt.Sprintf("exists=%t", status.Exists),
		fmt.Sprintf("directory=%t", status.Directory),
		fmt.Sprintf("executable=%t", status.Executable),
	}

	if status.IsSymlink {
		parts = append(parts, fmt.Sprintf("symlink=%s", status.SymlinkTarget))
	}
	if status.Mode != "" {
		parts = append(parts, fmt.Sprintf("mode=%s", status.Mode))
	}
	if status.ModifiedAt != "" {
		parts = append(parts, fmt.Sprintf("modified=%s", formatLocalTimestampString(status.ModifiedAt)))
	}
	if status.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", status.Error))
	}

	return strings.Join(parts, " ")
}

package openwrt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

type zapretMarker struct {
	Domains        []string `json:"domains"`
	CIDRs          []string `json:"cidrs,omitempty"`
	OriginalExists bool     `json:"original_exists"`
}

const (
	zapretConfigManagedStart = "# RouteFlux managed start"
	zapretConfigManagedEnd   = "# RouteFlux managed end"
)

// ZapretManager manages RouteFlux-owned zapret-openwrt fallback state.
type ZapretManager struct {
	ServicePath        string
	ConfigPath         string
	ConfigBackupPath   string
	HostlistPath       string
	HostlistBackupPath string
	IPListPath         string
	IPListBackupPath   string
	MarkerPath         string
}

// NewZapretManager creates an OpenWrt Zapret manager with RouteFlux defaults.
func NewZapretManager() ZapretManager {
	return ZapretManager{
		ServicePath:        ZapretServicePath(),
		ConfigPath:         ZapretConfigPath(),
		ConfigBackupPath:   ZapretConfigBackupPath(),
		HostlistPath:       ZapretHostlistPath(),
		HostlistBackupPath: ZapretHostlistBackupPath(),
		IPListPath:         ZapretIPListPath(),
		IPListBackupPath:   ZapretIPListBackupPath(),
		MarkerPath:         ZapretMarkerPath(),
	}
}

// Apply activates RouteFlux-managed Zapret hostlist fallback.
func (m ZapretManager) Apply(ctx context.Context, domains, cidrs []string) (domain.ZapretStatus, error) {
	status, err := m.Status(ctx)
	if err != nil {
		return status, err
	}
	if !status.Installed {
		status.LastReason = "zapret package is not installed"
		return status, fmt.Errorf("%s", status.LastReason)
	}

	domains = domain.NormalizeZapretDomainList(domains)
	cidrs = domain.NormalizeZapretCIDRList(cidrs)
	if len(domains) == 0 && len(cidrs) == 0 {
		status.LastReason = "zapret selectors are empty"
		return status, fmt.Errorf("%s", status.LastReason)
	}

	if err := m.backupOriginalFile(m.hostlistPath(), m.hostlistBackupPath()); err != nil {
		return status, fmt.Errorf("backup zapret hostlist: %w", err)
	}
	if err := m.backupOriginalFile(m.ipListPath(), m.ipListBackupPath()); err != nil {
		return status, fmt.Errorf("backup zapret ip list: %w", err)
	}
	if err := m.syncConfig(cidrs); err != nil {
		return status, fmt.Errorf("prepare zapret config: %w", err)
	}

	if err := atomicWriteText(m.hostlistPath(), buildZapretList("# Managed by RouteFlux", domains), 0o644); err != nil {
		return status, fmt.Errorf("write zapret hostlist: %w", err)
	}
	if err := atomicWriteText(m.ipListPath(), buildZapretList("# Managed by RouteFlux", cidrs), 0o644); err != nil {
		return status, fmt.Errorf("write zapret ip list: %w", err)
	}

	markerData, err := json.MarshalIndent(zapretMarker{
		Domains:        domains,
		CIDRs:          cidrs,
		OriginalExists: fileExists(m.hostlistBackupPath()) || fileExists(m.ipListBackupPath()),
	}, "", "  ")
	if err != nil {
		return status, fmt.Errorf("encode zapret marker: %w", err)
	}
	if err := atomicWriteText(m.markerPath(), string(markerData)+"\n", 0o644); err != nil {
		return status, fmt.Errorf("write zapret marker: %w", err)
	}

	action := "start"
	if status.ServiceActive {
		action = "restart"
	}
	if err := m.run(ctx, action); err != nil {
		return status, err
	}

	status, err = m.Status(ctx)
	if err != nil {
		return status, err
	}
	if !status.Active {
		status.LastReason = firstNonEmpty(status.LastReason, "zapret service is not running")
		return status, fmt.Errorf("%s", status.LastReason)
	}

	status.Managed = true
	status.LastReason = ""
	return status, nil
}

// Disable removes RouteFlux-managed Zapret state and restores the previous hostlist.
func (m ZapretManager) Disable(ctx context.Context) error {
	if !fileExists(m.markerPath()) {
		return nil
	}

	if fileExists(m.servicePath()) {
		if err := m.run(ctx, "stop"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not running") {
			return err
		}
	}

	if fileExists(m.hostlistBackupPath()) {
		if err := m.restoreManagedFile(m.hostlistPath(), m.hostlistBackupPath()); err != nil {
			return fmt.Errorf("restore zapret hostlist: %w", err)
		}
	} else if err := os.Remove(m.hostlistPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret hostlist: %w", err)
	}
	if fileExists(m.ipListBackupPath()) {
		if err := m.restoreManagedFile(m.ipListPath(), m.ipListBackupPath()); err != nil {
			return fmt.Errorf("restore zapret ip list: %w", err)
		}
	} else if err := os.Remove(m.ipListPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret ip list: %w", err)
	}
	if fileExists(m.configBackupPath()) {
		if err := m.restoreManagedFile(m.configPath(), m.configBackupPath()); err != nil {
			return fmt.Errorf("restore zapret config: %w", err)
		}
	}

	if err := os.Remove(m.hostlistBackupPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret hostlist backup: %w", err)
	}
	if err := os.Remove(m.ipListBackupPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret ip list backup: %w", err)
	}
	if err := os.Remove(m.configBackupPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret config backup: %w", err)
	}
	if err := os.Remove(m.markerPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove zapret marker: %w", err)
	}

	return nil
}

// Status reports whether Zapret is installed, managed by RouteFlux, and active.
func (m ZapretManager) Status(ctx context.Context) (domain.ZapretStatus, error) {
	status := domain.ZapretStatus{
		Installed:     fileExists(m.servicePath()),
		Managed:       fileExists(m.markerPath()),
		ServiceState:  "not-installed",
		ServiceActive: false,
	}

	if !status.Installed {
		return status, nil
	}

	serviceState, serviceActive, err := m.serviceStatus(ctx)
	status.ServiceState = firstNonEmpty(serviceState, "unknown")
	status.ServiceActive = serviceActive
	status.Active = serviceActive && status.Managed
	if status.ServiceActive && !status.Managed {
		status.LastReason = "zapret service is running outside RouteFlux"
	}
	if err != nil && status.LastReason == "" {
		status.LastReason = err.Error()
	}

	return status, err
}

func (m ZapretManager) backupOriginalFile(path, backupPath string) error {
	if fileExists(m.markerPath()) || fileExists(backupPath) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return atomicWriteText(backupPath, string(data), 0o644)
}

func (m ZapretManager) restoreManagedFile(path, backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup %s: %w", backupPath, err)
	}
	if err := atomicWriteText(path, string(data), 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", path, err)
	}
	return nil
}

func (m ZapretManager) serviceStatus(ctx context.Context) (string, bool, error) {
	cmd := exec.CommandContext(ctx, m.servicePath(), "status")
	output, err := cmd.CombinedOutput()
	serviceState := strings.TrimSpace(string(output))
	if serviceState == "" {
		serviceState = "unknown"
	}
	active := statusOutputLooksRunning(serviceState)

	if err != nil && !active {
		return serviceState, false, nil
	}

	return serviceState, active, err
}

func (m ZapretManager) run(ctx context.Context, action string) error {
	cmd := exec.CommandContext(ctx, m.servicePath(), action)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run %s %s: %w: %s", m.servicePath(), action, err, string(output))
	}
	return nil
}

func (m ZapretManager) servicePath() string {
	return firstNonEmpty(m.ServicePath, ZapretServicePath())
}

func (m ZapretManager) configPath() string {
	return firstNonEmpty(m.ConfigPath, ZapretConfigPath())
}

func (m ZapretManager) configBackupPath() string {
	return firstNonEmpty(m.ConfigBackupPath, ZapretConfigBackupPath())
}

func (m ZapretManager) hostlistPath() string {
	return firstNonEmpty(m.HostlistPath, ZapretHostlistPath())
}

func (m ZapretManager) hostlistBackupPath() string {
	return firstNonEmpty(m.HostlistBackupPath, ZapretHostlistBackupPath())
}

func (m ZapretManager) ipListPath() string {
	return firstNonEmpty(m.IPListPath, ZapretIPListPath())
}

func (m ZapretManager) ipListBackupPath() string {
	return firstNonEmpty(m.IPListBackupPath, ZapretIPListBackupPath())
}

func (m ZapretManager) markerPath() string {
	return firstNonEmpty(m.MarkerPath, ZapretMarkerPath())
}

func buildZapretList(header string, values []string) string {
	lines := []string{"# Managed by RouteFlux", ""}
	if header != "" {
		lines[0] = header
	}
	lines = append(lines, values...)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m ZapretManager) syncConfig(cidrs []string) error {
	if err := m.backupOriginalFile(m.configPath(), m.configBackupPath()); err != nil {
		return err
	}

	sourcePath := m.configBackupPath()
	if !fileExists(sourcePath) {
		sourcePath = m.configPath()
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	managed := buildManagedZapretConfig(string(data), m.ipListPath(), len(cidrs) > 0)
	return atomicWriteText(m.configPath(), managed, 0o644)
}

func buildManagedZapretConfig(base, ipListPath string, enableIPProfile bool) string {
	clean := strings.TrimRight(stripManagedZapretConfig(base), "\n")
	if !enableIPProfile {
		if clean == "" {
			return ""
		}
		return clean + "\n"
	}
	if clean == "" {
		return renderManagedZapretConfig(ipListPath)
	}
	return clean + "\n\n" + renderManagedZapretConfig(ipListPath)
}

func renderManagedZapretConfig(ipListPath string) string {
	lines := []string{
		zapretConfigManagedStart,
		fmt.Sprintf(`ROUTEFLUX_ZAPRET_IPLIST="%s"`, ipListPath),
		`if [ -s "$ROUTEFLUX_ZAPRET_IPLIST" ]; then`,
		`  NFQWS_OPT="$NFQWS_OPT --new --comment=RouteFlux_IPSet_HTTP --filter-tcp=80 --ipset=$ROUTEFLUX_ZAPRET_IPLIST --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-fake-http=/opt/zapret/files/fake/http_iana_org.bin --new --comment=RouteFlux_IPSet_TLS --filter-tcp=443,5222 --ipset=$ROUTEFLUX_ZAPRET_IPLIST --dpi-desync=fake --dpi-desync-repeats=8 --dpi-desync-fake-tls=/opt/zapret/files/fake/tls_clienthello_www_google_com.bin --dpi-desync-fake-tls-mod=rnd --new --comment=RouteFlux_IPSet_UDP --filter-udp=443 --ipset=$ROUTEFLUX_ZAPRET_IPLIST --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-fake-quic=/opt/zapret/files/fake/quic_initial_www_google_com.bin"`,
		`fi`,
		`unset ROUTEFLUX_ZAPRET_IPLIST`,
		zapretConfigManagedEnd,
		"",
	}
	return strings.Join(lines, "\n")
}

func stripManagedZapretConfig(base string) string {
	for {
		start := strings.Index(base, zapretConfigManagedStart)
		if start < 0 {
			return base
		}
		end := strings.Index(base[start:], zapretConfigManagedEnd)
		if end < 0 {
			return base[:start]
		}
		end += start + len(zapretConfigManagedEnd)
		if end < len(base) && base[end] == '\n' {
			end++
		}
		for start > 0 && base[start-1] == '\n' {
			start--
		}
		base = base[:start] + base[end:]
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func statusOutputLooksRunning(output string) bool {
	normalized := strings.ToLower(strings.TrimSpace(output))
	switch {
	case normalized == "", normalized == "unknown":
		return false
	case strings.Contains(normalized, "no instances"):
		return false
	case strings.Contains(normalized, "inactive"):
		return false
	case strings.Contains(normalized, "not running"):
		return false
	case strings.Contains(normalized, "stopped"):
		return false
	case strings.Contains(normalized, "running"):
		return true
	case strings.Contains(normalized, "active"):
		return true
	default:
		return false
	}
}

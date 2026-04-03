package openwrt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

const defaultIPv6SysctlConfigPath = "/etc/sysctl.d/99-routeflux-ipv6.conf"

// IPv6SysctlConfigPath returns the RouteFlux-managed sysctl file used for IPv6 state.
func IPv6SysctlConfigPath() string {
	if path := os.Getenv("ROUTEFLUX_IPV6_SYSCTL"); path != "" {
		return path
	}
	return defaultIPv6SysctlConfigPath
}

// IPv6Manager applies and inspects RouteFlux-managed IPv6 system state.
type IPv6Manager struct {
	ProcRoot         string
	SysctlConfigPath string
}

// NewIPv6Manager creates an IPv6 manager with OpenWrt defaults.
func NewIPv6Manager() IPv6Manager {
	return IPv6Manager{
		ProcRoot:         "/proc",
		SysctlConfigPath: IPv6SysctlConfigPath(),
	}
}

// Apply enables or disables IPv6 for current and future interfaces.
func (m IPv6Manager) Apply(_ context.Context, disabled bool) error {
	if disabled {
		if err := atomicWriteText(m.sysctlConfigPath(), managedIPv6SysctlConfig(true), 0o644); err != nil {
			return fmt.Errorf("write ipv6 sysctl config: %w", err)
		}
	} else if err := os.Remove(m.sysctlConfigPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove ipv6 sysctl config: %w", err)
	}

	if err := m.writeRuntimeDisableValue(disabled); err != nil {
		return err
	}

	return nil
}

// Status reports the current persistent and runtime IPv6 state.
func (m IPv6Manager) Status(_ context.Context) (domain.IPv6Status, error) {
	status := domain.IPv6Status{
		Available:  true,
		ConfigPath: m.sysctlConfigPath(),
	}

	if _, err := os.Stat(status.ConfigPath); err == nil {
		status.PersistentDisabled = true
	} else if !os.IsNotExist(err) {
		return domain.IPv6Status{}, fmt.Errorf("stat ipv6 sysctl config: %w", err)
	}

	confDir := m.procIPv6ConfDir()
	entries, err := os.ReadDir(confDir)
	if err != nil {
		if os.IsNotExist(err) {
			status.RuntimeDisabled = true
			return status, nil
		}
		return domain.IPv6Status{}, fmt.Errorf("read ipv6 proc state: %w", err)
	}

	enabled := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || name == "lo" {
			continue
		}

		valuePath := filepath.Join(confDir, name, "disable_ipv6")
		data, err := os.ReadFile(valuePath)
		if err != nil {
			return domain.IPv6Status{}, fmt.Errorf("read ipv6 state for %s: %w", name, err)
		}
		if strings.TrimSpace(string(data)) != "1" {
			enabled = append(enabled, name)
		}
	}

	sort.Strings(enabled)
	status.EnabledInterfaces = enabled
	status.RuntimeDisabled = len(enabled) == 0
	return status, nil
}

func (m IPv6Manager) procIPv6ConfDir() string {
	return filepath.Join(firstNonEmpty(m.ProcRoot, "/proc"), "sys", "net", "ipv6", "conf")
}

func (m IPv6Manager) sysctlConfigPath() string {
	return firstNonEmpty(m.SysctlConfigPath, IPv6SysctlConfigPath())
}

func managedIPv6SysctlConfig(disabled bool) string {
	value := "0"
	if disabled {
		value = "1"
	}

	return strings.Join([]string{
		"# Managed by RouteFlux",
		"net.ipv6.conf.all.disable_ipv6=" + value,
		"net.ipv6.conf.default.disable_ipv6=" + value,
		"",
	}, "\n")
}

func (m IPv6Manager) writeRuntimeDisableValue(disabled bool) error {
	confDir := m.procIPv6ConfDir()
	entries, err := os.ReadDir(confDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read ipv6 proc state: %w", err)
	}

	value := "0\n"
	if disabled {
		value = "1\n"
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(confDir, entry.Name(), "disable_ipv6")
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			return fmt.Errorf("write ipv6 state for %s: %w", entry.Name(), err)
		}
	}

	return nil
}

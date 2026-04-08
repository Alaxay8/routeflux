package openwrt

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alaxay8/routeflux/internal/domain"
)

const defaultDNSRuntimeSnippetName = "routeflux-dns.conf"

type dnsmasqRuntimeInfo struct {
	ConfDir    string
	ConfigPath string
	ResolvFile string
}

// DNSRuntimeManager manages dnsmasq forwarding into the local Xray DNS runtime.
type DNSRuntimeManager struct {
	DNSMasqServicePath string
	DNSMasqSnippetPath string
	ProcRoot           string
}

// NewDNSRuntimeManager creates an OpenWrt dnsmasq-based DNS runtime manager.
func NewDNSRuntimeManager() DNSRuntimeManager {
	return DNSRuntimeManager{
		DNSMasqServicePath: dnsmasqServicePath(),
		DNSMasqSnippetPath: dnsRuntimeSnippetOverridePath(),
		ProcRoot:           "/proc",
	}
}

func dnsRuntimeSnippetOverridePath() string {
	return os.Getenv("ROUTEFLUX_DNSMASQ_DNS_SNIPPET")
}

// SystemResolvers returns the currently configured system upstream resolvers from dnsmasq.
func (m DNSRuntimeManager) SystemResolvers(context.Context) ([]string, error) {
	info, err := m.runtimeInfo()
	if err != nil {
		return nil, err
	}
	return readResolvNameservers(info.ResolvFile)
}

// Apply writes the RouteFlux dnsmasq override and restarts dnsmasq.
func (m DNSRuntimeManager) Apply(ctx context.Context, settings domain.DNSSettings, listen string, port int) error {
	mode, err := domain.ParseDNSMode(string(settings.Mode))
	if err != nil {
		return err
	}
	if mode == domain.DNSModeSystem || mode == domain.DNSModeDisabled {
		return m.Disable(ctx)
	}

	info, err := m.runtimeInfo()
	if err != nil {
		return err
	}
	systemResolvers, err := readResolvNameservers(info.ResolvFile)
	if err != nil {
		return err
	}
	if len(systemResolvers) == 0 {
		return fmt.Errorf("detect system dns resolvers: no upstream resolvers found in %s", info.ResolvFile)
	}

	snippetPath := m.snippetPath(info)
	config, err := buildDNSMasqRouteFluxDNSConfig(settings, systemResolvers, listen, port)
	if err != nil {
		return err
	}
	if err := atomicWriteText(snippetPath, config, 0o644); err != nil {
		return fmt.Errorf("write dnsmasq dns runtime snippet: %w", err)
	}

	return m.restartDNSMasq(ctx)
}

// Disable removes the RouteFlux dnsmasq override and restarts dnsmasq when needed.
func (m DNSRuntimeManager) Disable(ctx context.Context) error {
	snippetPath := strings.TrimSpace(m.DNSMasqSnippetPath)
	if snippetPath == "" {
		info, err := m.runtimeInfo()
		if err != nil {
			return nil
		}
		snippetPath = m.snippetPath(info)
	}

	if err := os.Remove(snippetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove dnsmasq dns runtime snippet: %w", err)
	}

	return m.restartDNSMasq(ctx)
}

// Status reports the current RouteFlux-managed DNS runtime status on OpenWrt.
func (m DNSRuntimeManager) Status(ctx context.Context) (domain.DNSRuntimeStatus, error) {
	status := domain.DNSRuntimeStatus{
		Available:      true,
		LocalDNSListen: "127.0.0.1",
		LocalDNSPort:   1053,
	}

	info, err := m.runtimeInfo()
	if err != nil {
		status.Error = err.Error()
		return status, err
	}

	status.ResolvFile = info.ResolvFile
	status.DNSMasqSnippetPath = m.snippetPath(info)

	if resolvers, err := readResolvNameservers(info.ResolvFile); err == nil {
		status.SystemResolvers = resolvers
	} else {
		status.DegradedReason = err.Error()
	}

	if _, err := os.Stat(status.DNSMasqSnippetPath); err == nil {
		status.DNSMasqSnippetFound = true
		status.Active = true
	} else if !os.IsNotExist(err) {
		status.DegradedReason = err.Error()
	}

	return status, nil
}

func (m DNSRuntimeManager) restartDNSMasq(ctx context.Context) error {
	script := firstNonEmpty(m.DNSMasqServicePath, dnsmasqServicePath())
	if script == "" {
		return nil
	}

	if err := runCommand(ctx, script, "restart"); err != nil {
		return fmt.Errorf("restart dnsmasq service: %w", err)
	}

	return nil
}

func (m DNSRuntimeManager) runtimeInfo() (dnsmasqRuntimeInfo, error) {
	procRoot := firstNonEmpty(m.ProcRoot, "/proc")
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return dnsmasqRuntimeInfo{}, fmt.Errorf("read %s: %w", procRoot, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isNumeric(entry.Name()) {
			continue
		}

		pidDir := filepath.Join(procRoot, entry.Name())
		comm, err := os.ReadFile(filepath.Join(pidDir, "comm"))
		if err == nil && strings.TrimSpace(string(comm)) != "dnsmasq" {
			continue
		}

		args, err := readNullSeparated(filepath.Join(pidDir, "cmdline"))
		if err != nil {
			continue
		}

		info := dnsmasqRuntimeInfo{
			ConfDir:    dnsmasqConfDirFromArgs(args),
			ConfigPath: dnsmasqConfigPathFromArgs(args),
			ResolvFile: dnsmasqResolvFileFromArgs(args),
		}
		if info.ConfigPath != "" {
			if info.ConfDir == "" {
				info.ConfDir = dnsmasqConfDirFromConfig(info.ConfigPath)
			}
			if info.ResolvFile == "" {
				info.ResolvFile = dnsmasqResolvFileFromConfig(info.ConfigPath)
			}
		}
		if info.ConfDir != "" && info.ResolvFile != "" {
			return info, nil
		}
	}

	return dnsmasqRuntimeInfo{}, fmt.Errorf("detect dnsmasq runtime: no running dnsmasq with conf-dir and resolv-file found")
}

func (m DNSRuntimeManager) snippetPath(info dnsmasqRuntimeInfo) string {
	if path := firstNonEmpty(m.DNSMasqSnippetPath, dnsRuntimeSnippetOverridePath()); path != "" {
		return path
	}
	return filepath.Join(info.ConfDir, defaultDNSRuntimeSnippetName)
}

func dnsmasqConfigPathFromArgs(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case strings.HasPrefix(arg, "--conf-file="):
			return strings.TrimPrefix(arg, "--conf-file=")
		case (arg == "--conf-file" || arg == "-C") && idx+1 < len(args):
			return args[idx+1]
		}
	}

	return ""
}

func dnsmasqResolvFileFromArgs(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case strings.HasPrefix(arg, "--resolv-file="):
			return strings.TrimPrefix(arg, "--resolv-file=")
		case arg == "--resolv-file" && idx+1 < len(args):
			return args[idx+1]
		}
	}

	return ""
}

func dnsmasqResolvFileFromConfig(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "resolv-file=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "resolv-file="))
		}
	}

	return ""
}

func readResolvNameservers(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read dnsmasq resolv-file %s: %w", path, err)
	}
	defer file.Close()

	seen := map[string]struct{}{}
	out := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		server := strings.TrimSpace(fields[1])
		if server == "" {
			continue
		}
		if _, ok := seen[server]; ok {
			continue
		}
		seen[server] = struct{}{}
		out = append(out, server)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan dnsmasq resolv-file %s: %w", path, err)
	}
	return out, nil
}

func buildDNSMasqRouteFluxDNSConfig(settings domain.DNSSettings, systemResolvers []string, listen string, port int) (string, error) {
	if len(systemResolvers) == 0 {
		return "", fmt.Errorf("detect system dns resolvers: no upstream resolvers found")
	}

	var builder strings.Builder
	builder.WriteString("# Generated by RouteFlux. Routes public DNS through the local Xray DNS runtime.\n")
	builder.WriteString("no-resolv\n")
	fmt.Fprintf(&builder, "server=%s#%d\n", firstNonEmpty(listen, "127.0.0.1"), defaultPort(port, 1053))

	mode, err := domain.ParseDNSMode(string(settings.Mode))
	if err != nil {
		return "", err
	}
	if mode != domain.DNSModeSplit {
		return builder.String(), nil
	}

	for _, raw := range settings.DirectDomains {
		matcher, err := dnsmasqDirectDomainMatcher(raw)
		if err != nil {
			return "", err
		}
		if matcher == "" {
			continue
		}
		for _, resolver := range systemResolvers {
			fmt.Fprintf(&builder, "server=/%s/%s\n", matcher, resolver)
		}
	}

	return builder.String(), nil
}

func dnsmasqDirectDomainMatcher(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	switch {
	case strings.HasPrefix(value, "domain:"):
		value = strings.TrimSpace(strings.TrimPrefix(value, "domain:"))
	case strings.HasPrefix(value, "full:"):
		value = strings.TrimSpace(strings.TrimPrefix(value, "full:"))
	case strings.Contains(value, ":"):
		return "", fmt.Errorf("unsupported direct domain matcher %q", raw)
	}

	if value == "" {
		return "", fmt.Errorf("direct domain matcher %q is empty", raw)
	}

	return value, nil
}

func defaultPort(got, fallback int) int {
	if got > 0 {
		return got
	}
	return fallback
}

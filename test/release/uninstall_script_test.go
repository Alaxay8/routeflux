package release_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallScriptRemovesRouteFluxAndXrayArtifacts(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(repoRoot(t), "scripts", "uninstall.sh")
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")
	opkgStatePath := filepath.Join(t.TempDir(), "opkg-state.txt")

	binDir := t.TempDir()
	writeInstallOpkgStub(t, filepath.Join(binDir, "opkg"), "mipsel_24kc")

	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "routeflux"), "#!/bin/sh\nset -eu\nprintf 'routeflux-bin:%s\\n' \"$*\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "routeflux"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "xray"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "zapret"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))

	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "subscriptions.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "settings.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "state.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "speedtest.lock"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "install-manifest.txt"), strings.Join([]string{
		"pkg=ca-bundle",
		"pkg=curl",
		"pkg=nftables",
		"pkg=kmod-nft-tproxy",
		"pkg=dnsmasq-full",
		"pkg=unzip",
		"pkg=zapret",
		"restore=dnsmasq",
		"",
	}, "\n"), 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "xray", "config.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "xray", "config.json.last-known-good"), "{}\n", 0o644)
	cronHelper, err := os.ReadFile(filepath.Join(repoRoot(t), "openwrt", "root", "usr", "libexec", "routeflux-cron"))
	if err != nil {
		t.Fatalf("read routeflux cron helper: %v", err)
	}
	writeExecutable(t, filepath.Join(installRoot, "usr", "libexec", "routeflux-cron"), string(cronHelper))
	selfUpdateHelper, err := os.ReadFile(filepath.Join(repoRoot(t), "openwrt", "root", "usr", "libexec", "routeflux-self-update"))
	if err != nil {
		t.Fatalf("read routeflux self-update helper: %v", err)
	}
	writeExecutable(t, filepath.Join(installRoot, "usr", "libexec", "routeflux-self-update"), string(selfUpdateHelper))
	xrayUpdateHelper, err := os.ReadFile(filepath.Join(repoRoot(t), "openwrt", "root", "usr", "libexec", "routeflux-xray-update"))
	if err != nil {
		t.Fatalf("read routeflux xray update helper: %v", err)
	}
	writeExecutable(t, filepath.Join(installRoot, "usr", "libexec", "routeflux-xray-update"), string(xrayUpdateHelper))
	writeFile(t, filepath.Join(installRoot, "etc", "crontabs", "root"), strings.Join([]string{
		"15 4 * * * echo keep",
		"# routeflux:xray-log-retention:start",
		"0 * * * * [ -f /var/log/xray.log ] && : > /var/log/xray.log",
		"# routeflux:xray-log-retention:end",
		"",
	}, "\n"), 0o644)
	writeFile(t, filepath.Join(installRoot, "var", "log", "xray.log"), "log\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "var", "run", "xray.pid"), "123\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "usr", "share", "luci", "menu.d", "luci-app-routeflux.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "www", "luci-static", "resources", "routeflux", "ui.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "www", "luci-static", "resources", "view", "routeflux", "subscriptions.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "rc.d", "S95routeflux"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "rc.d", "S95xray"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "rc.d", "S95zapret"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "routeflux-firewall.nft"), "table inet routeflux {}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "routeflux-speedtest-123", "config.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "xray-cache"), "cache\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "lock", "procd_routeflux.lock"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "lock", "procd_zapret.lock"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "luci-indexcache"), "cache\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "luci-modulecache", "index"), "cache\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "opt", "zapret", "ipset", "zapret-hosts-user.txt"), "youtube.com\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "opt", "zapret", "ipset", "zapret-hosts-user.txt.routeflux.bak"), "original.example.com\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "opt", "zapret", "ipset", "zapret-ip-user.txt"), "91.108.0.0/16\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "opt", "zapret", "ipset", "zapret-ip-user.txt.routeflux.bak"), "203.0.113.0/24\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "config", "zapret"), "config zapret 'base'\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "hotplug.d", "iface", "90-zapret"), "#!/bin/sh\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "sysctl.d", "99-routeflux-ipv6.conf"), "# Managed by RouteFlux\nnet.ipv6.conf.all.disable_ipv6=1\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "init.d", "routeflux.bak.20260327-233221"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(installRoot, "etc", "opkg", "customfeeds.conf"), "src/gz routeflux https://github.com/Alaxay8/routeflux/releases/download/v0.1.4\nsrc/gz other https://example.invalid/feed\nsrc/gz routeflux https://github.com/Alaxay8/routeflux/releases/download/v0.1.4\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "opkg", "keys", "9e842876f8b9501d"), "untrusted comment: RouteFlux opkg feed\nPUBLICKEY\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "zapret-managed.json"), "{\"domains\":[\"youtube.com\"]}\n", 0o644)
	writeFile(t, opkgStatePath, strings.Join([]string{
		"base-files",
		"ca-bundle",
		"curl",
		"nftables",
		"kmod-nft-tproxy",
		"dnsmasq-full",
		"unzip",
		"zapret",
		"",
	}, "\n"), 0o644)

	stdout, stderr, err := runUninstallScriptWithEnv(
		t,
		scriptPath,
		installRoot,
		serviceLogPath,
		binDir,
		map[string]string{
			"ROUTEFLUX_TEST_BIN_DIR":      binDir,
			"ROUTEFLUX_TEST_INSTALL_ROOT": installRoot,
			"ROUTEFLUX_TEST_OPKG_STATE":   opkgStatePath,
		},
	)
	if err != nil {
		t.Fatalf("run uninstall script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	for _, path := range []string{
		filepath.Join(installRoot, "usr", "bin", "routeflux"),
		filepath.Join(installRoot, "etc", "init.d", "routeflux"),
		filepath.Join(installRoot, "etc", "routeflux"),
		filepath.Join(installRoot, "usr", "bin", "xray"),
		filepath.Join(installRoot, "etc", "init.d", "xray"),
		filepath.Join(installRoot, "etc", "xray"),
		filepath.Join(installRoot, "etc", "init.d", "zapret"),
		filepath.Join(installRoot, "opt", "zapret"),
		filepath.Join(installRoot, "usr", "libexec", "routeflux-cron"),
		filepath.Join(installRoot, "usr", "libexec", "routeflux-self-update"),
		filepath.Join(installRoot, "usr", "libexec", "routeflux-xray-update"),
		filepath.Join(installRoot, "var", "log", "xray.log"),
		filepath.Join(installRoot, "var", "run", "xray.pid"),
		filepath.Join(installRoot, "usr", "share", "luci", "menu.d", "luci-app-routeflux.json"),
		filepath.Join(installRoot, "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json"),
		filepath.Join(installRoot, "www", "luci-static", "resources", "routeflux"),
		filepath.Join(installRoot, "www", "luci-static", "resources", "view", "routeflux"),
		filepath.Join(installRoot, "etc", "rc.d", "S95routeflux"),
		filepath.Join(installRoot, "etc", "rc.d", "S95xray"),
		filepath.Join(installRoot, "etc", "rc.d", "S95zapret"),
		filepath.Join(installRoot, "etc", "config", "zapret"),
		filepath.Join(installRoot, "etc", "hotplug.d", "iface", "90-zapret"),
		filepath.Join(installRoot, "etc", "sysctl.d", "99-routeflux-ipv6.conf"),
		filepath.Join(installRoot, "etc", "init.d", "routeflux.bak.20260327-233221"),
		filepath.Join(installRoot, "tmp", "routeflux-firewall.nft"),
		filepath.Join(installRoot, "tmp", "routeflux-speedtest-123"),
		filepath.Join(installRoot, "tmp", "xray-cache"),
		filepath.Join(installRoot, "tmp", "lock", "procd_routeflux.lock"),
		filepath.Join(installRoot, "tmp", "lock", "procd_zapret.lock"),
		filepath.Join(installRoot, "tmp", "luci-indexcache"),
		filepath.Join(installRoot, "tmp", "luci-modulecache"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", path, err)
		}
	}

	serviceLog, err := os.ReadFile(serviceLogPath)
	if err != nil {
		t.Fatalf("read service log: %v", err)
	}

	for _, want := range []string{
		"routeflux-bin:--root ",
		" disconnect",
		" firewall disable",
		"cron:restart",
		"routeflux:stop",
		"routeflux:disable",
		"xray:stop",
		"xray:disable",
		"zapret:stop",
		"zapret:disable",
		"opkg:remove:zapret",
		"opkg:remove:unzip",
		"opkg:remove:dnsmasq-full",
		"opkg:remove:kmod-nft-tproxy",
		"opkg:remove:nftables",
		"opkg:remove:curl",
		"opkg:remove:ca-bundle",
		"opkg:install:dnsmasq",
		"rpcd:reload",
		"uhttpd:reload",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}

	if !strings.Contains(stdout, "RouteFlux, bundled Xray, Zapret, and installer-managed packages removed.") {
		t.Fatalf("expected completion message in stdout, got %q", stdout)
	}

	opkgState, err := os.ReadFile(opkgStatePath)
	if err != nil {
		t.Fatalf("read opkg state: %v", err)
	}
	for _, unwanted := range []string{
		"ca-bundle",
		"curl",
		"nftables",
		"kmod-nft-tproxy",
		"dnsmasq-full",
		"unzip",
		"zapret",
	} {
		if strings.Contains(string(opkgState), unwanted+"\n") {
			t.Fatalf("expected %q to be removed from opkg state, got %q", unwanted, string(opkgState))
		}
	}
	if !strings.Contains(string(opkgState), "dnsmasq\n") {
		t.Fatalf("expected dnsmasq to be restored, got %q", string(opkgState))
	}
	if !strings.Contains(string(opkgState), "base-files\n") {
		t.Fatalf("expected unrelated packages to remain, got %q", string(opkgState))
	}

	crontabPath := filepath.Join(installRoot, "etc", "crontabs", "root")
	contents, err := os.ReadFile(crontabPath)
	if err != nil {
		t.Fatalf("read crontab: %v", err)
	}
	if strings.Contains(string(contents), "routeflux:xray-log-retention") {
		t.Fatalf("expected managed cron block to be removed, got %q", string(contents))
	}
	if !strings.Contains(string(contents), "15 4 * * * echo keep") {
		t.Fatalf("expected unrelated cron entry to remain, got %q", string(contents))
	}

	customfeeds, err := os.ReadFile(filepath.Join(installRoot, "etc", "opkg", "customfeeds.conf"))
	if err != nil {
		t.Fatalf("read customfeeds.conf: %v", err)
	}
	if strings.Contains(string(customfeeds), "routeflux") {
		t.Fatalf("expected routeflux feed entries to be removed, got %q", string(customfeeds))
	}
	if !strings.Contains(string(customfeeds), "src/gz other https://example.invalid/feed") {
		t.Fatalf("expected unrelated feed entry to remain, got %q", string(customfeeds))
	}
	if _, err := os.Stat(filepath.Join(installRoot, "etc", "opkg", "keys", "9e842876f8b9501d")); !os.IsNotExist(err) {
		t.Fatalf("expected routeflux opkg key to be removed, stat err=%v", err)
	}
}

func TestUninstallScriptRemovesLegacyZapretAndRouteFluxTailsWithoutManifest(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(repoRoot(t), "scripts", "uninstall.sh")
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")
	opkgStatePath := filepath.Join(t.TempDir(), "opkg-state.txt")

	binDir := t.TempDir()
	writeInstallOpkgStub(t, filepath.Join(binDir, "opkg"), "mipsel_24kc")

	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "zapret"))
	writeFile(t, filepath.Join(installRoot, "usr", "libexec", "routeflux-xray-update"), "#!/bin/sh\nexit 0\n", 0o755)
	writeFile(t, filepath.Join(installRoot, "etc", "init.d", "routeflux.bak.20260327-233221"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(installRoot, "etc", "sysctl.d", "99-routeflux-ipv6.conf"), "# Managed by RouteFlux\nnet.ipv6.conf.all.disable_ipv6=1\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "config", "zapret"), "config zapret 'base'\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "hotplug.d", "iface", "90-zapret"), "#!/bin/sh\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "opkg", "customfeeds.conf"), "src/gz routeflux https://github.com/Alaxay8/routeflux/releases/download/v0.1.4\nsrc/gz keep https://example.invalid/feed\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "opkg", "keys", "9e842876f8b9501d"), "untrusted comment: RouteFlux opkg feed\nPUBLICKEY\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "opt", "zapret", "config"), "# config\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "lock", "procd_routeflux.lock"), "", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "lock", "procd_zapret.lock"), "", 0o644)
	writeFile(t, opkgStatePath, "base-files\nzapret\n", 0o644)

	stdout, stderr, err := runUninstallScriptWithEnv(
		t,
		scriptPath,
		installRoot,
		serviceLogPath,
		binDir,
		map[string]string{
			"ROUTEFLUX_TEST_BIN_DIR":      binDir,
			"ROUTEFLUX_TEST_INSTALL_ROOT": installRoot,
			"ROUTEFLUX_TEST_OPKG_STATE":   opkgStatePath,
		},
	)
	if err != nil {
		t.Fatalf("run uninstall script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	for _, path := range []string{
		filepath.Join(installRoot, "usr", "libexec", "routeflux-xray-update"),
		filepath.Join(installRoot, "etc", "init.d", "routeflux.bak.20260327-233221"),
		filepath.Join(installRoot, "etc", "sysctl.d", "99-routeflux-ipv6.conf"),
		filepath.Join(installRoot, "etc", "config", "zapret"),
		filepath.Join(installRoot, "etc", "hotplug.d", "iface", "90-zapret"),
		filepath.Join(installRoot, "opt", "zapret"),
		filepath.Join(installRoot, "tmp", "lock", "procd_routeflux.lock"),
		filepath.Join(installRoot, "tmp", "lock", "procd_zapret.lock"),
		filepath.Join(installRoot, "etc", "opkg", "keys", "9e842876f8b9501d"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", path, err)
		}
	}

	customfeeds, err := os.ReadFile(filepath.Join(installRoot, "etc", "opkg", "customfeeds.conf"))
	if err != nil {
		t.Fatalf("read customfeeds.conf: %v", err)
	}
	if strings.Contains(string(customfeeds), "routeflux") {
		t.Fatalf("expected routeflux feed entries to be removed, got %q", string(customfeeds))
	}
	if !strings.Contains(string(customfeeds), "src/gz keep https://example.invalid/feed") {
		t.Fatalf("expected unrelated feed entry to remain, got %q", string(customfeeds))
	}

	serviceLog, err := os.ReadFile(serviceLogPath)
	if err != nil {
		t.Fatalf("read service log: %v", err)
	}
	for _, want := range []string{
		"zapret:stop",
		"zapret:disable",
		"opkg:remove:zapret",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}

	opkgState, err := os.ReadFile(opkgStatePath)
	if err != nil {
		t.Fatalf("read opkg state: %v", err)
	}
	if strings.Contains(string(opkgState), "zapret\n") {
		t.Fatalf("expected zapret to be removed from opkg state, got %q", string(opkgState))
	}
	if !strings.Contains(stdout, "RouteFlux, bundled Xray, Zapret, and installer-managed packages removed.") {
		t.Fatalf("expected completion message in stdout, got %q", stdout)
	}
}

func TestUninstallScriptSucceedsWhenArtifactsAreMissing(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(repoRoot(t), "scripts", "uninstall.sh")
	installRoot := t.TempDir()

	stdout, stderr, err := runUninstallScript(t, scriptPath, installRoot, filepath.Join(t.TempDir(), "services.log"))
	if err != nil {
		t.Fatalf("run uninstall script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
}

func runUninstallScript(t *testing.T, scriptPath, installRoot, serviceLogPath string, extraArgs ...string) (string, string, error) {
	t.Helper()

	return runUninstallScriptWithEnv(t, scriptPath, installRoot, serviceLogPath, "", nil, extraArgs...)
}

func runUninstallScriptWithEnv(
	t *testing.T,
	scriptPath, installRoot, serviceLogPath, binDir string,
	extraEnv map[string]string,
	extraArgs ...string,
) (string, string, error) {
	t.Helper()

	args := append([]string{scriptPath, "--install-root", installRoot}, extraArgs...)
	cmd := exec.Command("sh", args...)
	env := append(os.Environ(),
		"ROUTEFLUX_TEST_SERVICE_LOG="+serviceLogPath,
	)
	if binDir != "" {
		env = append(env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

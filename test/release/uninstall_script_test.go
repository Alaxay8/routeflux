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

	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "routeflux"), "#!/bin/sh\nset -eu\nprintf 'routeflux-bin:%s\\n' \"$*\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "routeflux"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "xray"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))

	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "subscriptions.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "settings.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "state.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "etc", "routeflux", "speedtest.lock"), "", 0o644)
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
	writeFile(t, filepath.Join(installRoot, "tmp", "routeflux-firewall.nft"), "table inet routeflux {}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "routeflux-speedtest-123", "config.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "xray-cache"), "cache\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "luci-indexcache"), "cache\n", 0o644)
	writeFile(t, filepath.Join(installRoot, "tmp", "luci-modulecache", "index"), "cache\n", 0o644)

	stdout, stderr, err := runUninstallScript(t, scriptPath, installRoot, serviceLogPath)
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
		filepath.Join(installRoot, "usr", "libexec", "routeflux-cron"),
		filepath.Join(installRoot, "usr", "libexec", "routeflux-self-update"),
		filepath.Join(installRoot, "var", "log", "xray.log"),
		filepath.Join(installRoot, "var", "run", "xray.pid"),
		filepath.Join(installRoot, "usr", "share", "luci", "menu.d", "luci-app-routeflux.json"),
		filepath.Join(installRoot, "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json"),
		filepath.Join(installRoot, "www", "luci-static", "resources", "routeflux"),
		filepath.Join(installRoot, "www", "luci-static", "resources", "view", "routeflux"),
		filepath.Join(installRoot, "etc", "rc.d", "S95routeflux"),
		filepath.Join(installRoot, "etc", "rc.d", "S95xray"),
		filepath.Join(installRoot, "tmp", "routeflux-firewall.nft"),
		filepath.Join(installRoot, "tmp", "routeflux-speedtest-123"),
		filepath.Join(installRoot, "tmp", "xray-cache"),
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
		"rpcd:reload",
		"uhttpd:reload",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}

	if !strings.Contains(stdout, "RouteFlux and bundled Xray removed.") {
		t.Fatalf("expected completion message in stdout, got %q", stdout)
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

	args := append([]string{scriptPath, "--install-root", installRoot}, extraArgs...)
	cmd := exec.Command("sh", args...)
	cmd.Env = append(os.Environ(),
		"ROUTEFLUX_TEST_SERVICE_LOG="+serviceLogPath,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

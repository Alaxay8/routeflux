package release_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageOpenWrtFallsBackToTarWhenBSDTarMissing(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	scriptSource, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "package-openwrt.sh"))
	if err != nil {
		t.Fatalf("read package-openwrt.sh: %v", err)
	}

	writeExecutable(t, filepath.Join(repoDir, "scripts", "package-openwrt.sh"), string(scriptSource))
	writeExecutable(t, filepath.Join(repoDir, "bin", "openwrt", "x86_64", "routeflux"), "#!/bin/sh\nprintf 'routeflux test binary\\n'\n")
	writeExecutable(t, filepath.Join(repoDir, "openwrt", "root", "etc", "init.d", "routeflux"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(repoDir, "openwrt", "root", "usr", "libexec", "routeflux-cron"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(repoDir, "openwrt", "root", "usr", "libexec", "routeflux-self-update"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "root", "usr", "share", "luci", "menu.d", "luci-app-routeflux.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "root", "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json"), "{}\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "htdocs", "luci-static", "resources", "routeflux", "ui.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "subscriptions.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "firewall.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "about.js"), "'use strict';\n", 0o644)
	writeFile(t, filepath.Join(repoDir, "luci-app-routeflux", "htdocs", "luci-static", "resources", "view", "routeflux", "overview.js"), "'use strict';\n", 0o644)

	toolDir := t.TempDir()
	for _, name := range []string{"basename", "cat", "chmod", "cp", "date", "dirname", "find", "gzip", "mkdir", "rm", "sort", "tar", "tr", "wc"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(toolDir, name)); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}

	cmd := exec.Command("sh", filepath.Join(repoDir, "scripts", "package-openwrt.sh"))
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"PATH="+toolDir,
		"VERSION=1.2.3",
		"ARCH=x86_64",
		"BINARY_PATH="+filepath.Join(repoDir, "bin", "openwrt", "x86_64", "routeflux"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run package-openwrt.sh: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux_1.2.3_x86_64.ipk")); err != nil {
		t.Fatalf("expected ipk artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux_1.2.3_x86_64.tar.gz")); err != nil {
		t.Fatalf("expected tarball artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "www", "luci-static", "resources", "routeflux", "ui.js")); err != nil {
		t.Fatalf("expected shared routeflux ui helper in package data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "www", "luci-static", "resources", "view", "routeflux", "subscriptions.js")); err != nil {
		t.Fatalf("expected subscriptions view in package data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "www", "luci-static", "resources", "view", "routeflux", "firewall.js")); err != nil {
		t.Fatalf("expected routing view in package data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "www", "luci-static", "resources", "view", "routeflux", "about.js")); err != nil {
		t.Fatalf("expected about view in package data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "www", "luci-static", "resources", "view", "routeflux", "overview.js")); !os.IsNotExist(err) {
		t.Fatalf("expected obsolete overview view to stay out of package data, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "usr", "libexec", "routeflux-cron")); err != nil {
		t.Fatalf("expected cron helper in package data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dist", "routeflux-ipk", "data", "usr", "libexec", "routeflux-self-update")); err != nil {
		t.Fatalf("expected self-update helper in package data: %v", err)
	}
	postinstPath := filepath.Join(repoDir, "dist", "routeflux-ipk", "control", "postinst")
	postinst, err := os.ReadFile(postinstPath)
	if err != nil {
		t.Fatalf("read generated postinst: %v", err)
	}
	for _, want := range []string{
		"chmod 0700 /etc/routeflux",
		"/etc/routeflux/.routeflux.lock",
		"/etc/routeflux/speedtest.lock",
		"find /etc/routeflux -maxdepth 1 -type f -name '*.corrupt-*' -exec chmod 0600 {} \\;",
		"/etc/xray/config.json.last-known-good",
		"/www/luci-static/resources/view/routeflux/overview.js",
		"/www/luci-static/resources/view/routeflux/dns.js",
		"/www/luci-static/resources/view/routeflux/settings.js",
		"/www/luci-static/resources/view/routeflux/diagnostics.js",
		"/www/luci-static/resources/view/routeflux/logs.js",
		"/www/luci-static/resources/view/routeflux/services.js",
	} {
		if !strings.Contains(string(postinst), want) {
			t.Fatalf("expected generated postinst to contain %q, got:\n%s", want, postinst)
		}
	}

	if strings.Contains(string(postinst), "/www/luci-static/resources/view/routeflux/about.js") {
		t.Fatalf("expected generated postinst to keep about.js installed, got:\n%s", postinst)
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

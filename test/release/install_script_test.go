package release_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func supportedReleaseArches() []string {
	return []string{"mipsel_24kc", "x86_64", "aarch64_cortex-a53"}
}

func TestInstallScriptInstallsMatchedOpenWrtTarball(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(installRoot, "etc", "init.d", "xray"), "#!/bin/sh\nexit 0\n")

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch mipsel_24kc 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); err != nil {
		t.Fatalf("routeflux binary not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "etc", "init.d", "routeflux")); err != nil {
		t.Fatalf("routeflux service not installed: %v", err)
	}

	serviceLog, err := os.ReadFile(serviceLogPath)
	if err != nil {
		t.Fatalf("read service log: %v", err)
	}

	for _, want := range []string{
		"cron:restart",
		"rpcd:reload",
		"uhttpd:reload",
		"routeflux:enable",
		"routeflux:restart",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}

	if !strings.Contains(stdout, "mipsel_24kc") {
		t.Fatalf("expected stdout to mention resolved arch, got %q", stdout)
	}

	crontabPath := filepath.Join(installRoot, "etc", "crontabs", "root")
	contents, err := os.ReadFile(crontabPath)
	if err != nil {
		t.Fatalf("read crontab: %v", err)
	}
	if !strings.Contains(string(contents), "0 * * * * [ -f /var/log/xray.log ] && : > /var/log/xray.log") {
		t.Fatalf("expected xray retention cron entry, got %q", string(contents))
	}
}

func TestInstallScriptHardensExistingSecretFilePermissions(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(installRoot, "etc", "init.d", "xray"), "#!/bin/sh\nexit 0\n")

	routefluxRoot := filepath.Join(installRoot, "etc", "routeflux")
	if err := os.MkdirAll(routefluxRoot, 0o755); err != nil {
		t.Fatalf("create routeflux root: %v", err)
	}
	for _, path := range []string{
		filepath.Join(routefluxRoot, "subscriptions.json"),
		filepath.Join(routefluxRoot, "settings.json"),
		filepath.Join(routefluxRoot, "state.json"),
		filepath.Join(routefluxRoot, ".routeflux.lock"),
		filepath.Join(routefluxRoot, "speedtest.lock"),
		filepath.Join(routefluxRoot, "settings.corrupt-20260329T120000Z.json"),
		filepath.Join(installRoot, "etc", "xray", "config.json"),
		filepath.Join(installRoot, "etc", "xray", "config.json.last-known-good"),
	} {
		writeFile(t, path, "{}\n", 0o644)
	}

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch mipsel_24kc 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertMode(t, routefluxRoot, 0o700)
	for _, path := range []string{
		filepath.Join(routefluxRoot, "subscriptions.json"),
		filepath.Join(routefluxRoot, "settings.json"),
		filepath.Join(routefluxRoot, "state.json"),
		filepath.Join(routefluxRoot, ".routeflux.lock"),
		filepath.Join(routefluxRoot, "speedtest.lock"),
		filepath.Join(routefluxRoot, "settings.corrupt-20260329T120000Z.json"),
		filepath.Join(installRoot, "etc", "xray", "config.json"),
		filepath.Join(installRoot, "etc", "xray", "config.json.last-known-good"),
	} {
		assertMode(t, path, 0o600)
	}
}

func TestInstallScriptRejectsUnsupportedArchitecture(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)
	installRoot := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch aarch64_generic 10\\n'\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, t.TempDir(), filepath.Join(t.TempDir(), "services.log"))
	if err == nil {
		t.Fatalf("expected install script to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "unsupported architecture") {
		t.Fatalf("expected unsupported architecture error, got %q", combined)
	}
	for _, arch := range supportedReleaseArches() {
		if !strings.Contains(combined, arch) {
			t.Fatalf("expected supported arch list in output, got %q", combined)
		}
	}
}

func TestInstallScriptInstallsBundledXrayWhenMissing(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)
	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))
	writeXrayTarball(t, filepath.Join(downloadDir, "xray_"+version+"_mipsel_24kc.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch mipsel_24kc 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); err != nil {
		t.Fatalf("expected routeflux binary to be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "xray")); err != nil {
		t.Fatalf("expected xray binary to be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "etc", "init.d", "xray")); err != nil {
		t.Fatalf("expected xray service to be installed: %v", err)
	}
	if !strings.Contains(stdout, "Bundled Xray installed") {
		t.Fatalf("expected stdout to mention bundled xray install, got %q", stdout)
	}
}

func TestInstallScriptFallsBackToUnameForX8664(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_x86_64.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(installRoot, "etc", "init.d", "xray"), "#!/bin/sh\nexit 0\n")

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "uname"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"-m\" ] || exit 1\nprintf 'x86_64\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); err != nil {
		t.Fatalf("routeflux binary not installed: %v", err)
	}
	if !strings.Contains(stdout, "x86_64") {
		t.Fatalf("expected stdout to mention resolved arch, got %q", stdout)
	}
}

func TestInstallScriptFailsWhenBundledXrayAssetIsMissing(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	downloadDir := t.TempDir()
	installRoot := t.TempDir()

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch mipsel_24kc 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, filepath.Join(t.TempDir(), "services.log"))
	if err == nil {
		t.Fatalf("expected install script to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "xray_1.2.3_mipsel_24kc.tar.gz") {
		t.Fatalf("expected failure to mention missing xray asset, got %q", combined)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); !os.IsNotExist(err) {
		t.Fatalf("expected routeflux binary to remain uninstalled, stat err=%v", err)
	}
}

func TestInstallScriptInstallsMatchedAArch64OpenWrtTarball(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_aarch64_cortex-a53.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(installRoot, "etc", "init.d", "xray"), "#!/bin/sh\nexit 0\n")

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch aarch64_cortex-a53 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); err != nil {
		t.Fatalf("routeflux binary not installed: %v", err)
	}
	if !strings.Contains(stdout, "aarch64_cortex-a53") {
		t.Fatalf("expected stdout to mention resolved arch, got %q", stdout)
	}
}

func TestInstallScriptInstallsBundledXrayForAArch64WhenMissing(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)
	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_aarch64_cortex-a53.tar.gz"))
	writeXrayTarball(t, filepath.Join(downloadDir, "xray_"+version+"_aarch64_cortex-a53.tar.gz"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch aarch64_cortex-a53 10\\n'\n")
	writeExecutable(t, filepath.Join(binDir, "wget"), "#!/bin/sh\nset -eu\nout=''\nwhile [ \"$#\" -gt 0 ]; do\n\tcase \"$1\" in\n\t\t-O)\n\t\t\tout=\"$2\"\n\t\t\tshift 2\n\t\t\t;;\n\t\t*)\n\t\t\turl=\"$1\"\n\t\t\tshift\n\t\t\t;;\n\tesac\ndone\nmkdir -p \"$(dirname \"$out\")\"\ncp \"${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}/${url##*/}\" \"$out\"\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "routeflux")); err != nil {
		t.Fatalf("expected routeflux binary to be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "usr", "bin", "xray")); err != nil {
		t.Fatalf("expected xray binary to be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "etc", "init.d", "xray")); err != nil {
		t.Fatalf("expected xray service to be installed: %v", err)
	}
	if !strings.Contains(stdout, "Bundled Xray installed") {
		t.Fatalf("expected stdout to mention bundled xray install, got %q", stdout)
	}
}

func TestInstallScriptInstallsDependenciesAndZapretWhenEnabled(t *testing.T) {
	t.Parallel()

	systemUnzip, err := exec.LookPath("unzip")
	if err != nil {
		t.Skip("unzip is required on the test host")
	}

	version := "1.2.3"
	zapretVersion := "v72.20260113"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)
	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")
	opkgStatePath := filepath.Join(t.TempDir(), "opkg-state.txt")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))
	writeFile(t, filepath.Join(downloadDir, "zapret-api-latest.json"), fmt.Sprintf("{\"tag_name\":\"%s\"}\n", zapretVersion), 0o644)
	writeZapretZip(t, filepath.Join(downloadDir, "zapret_"+zapretVersion+"_mipsel_24kc.zip"), zapretVersion, "mipsel_24kc")
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
	writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
	writeExecutable(t, filepath.Join(installRoot, "usr", "bin", "xray"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(installRoot, "etc", "init.d", "xray"), "#!/bin/sh\nexit 0\n")

	binDir := t.TempDir()
	writeInstallOpkgStub(t, filepath.Join(binDir, "opkg"), "mipsel_24kc")
	writeInstallWgetStub(t, filepath.Join(binDir, "wget"))

	stdout, stderr, err := runInstallScriptWithEnv(
		t,
		scriptPath,
		installRoot,
		binDir,
		downloadDir,
		serviceLogPath,
		map[string]string{
			"ROUTEFLUX_TEST_BIN_DIR":      binDir,
			"ROUTEFLUX_TEST_INSTALL_ROOT": installRoot,
			"ROUTEFLUX_TEST_OPKG_STATE":   opkgStatePath,
			"ROUTEFLUX_TEST_SYSTEM_UNZIP": systemUnzip,
			"ROUTEFLUX_ZAPRET_VERSION":    "latest",
		},
		"--base-url", "https://example.test/releases/download/v1.2.3",
		"--install-root", installRoot,
	)
	if err != nil {
		t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if _, err := os.Stat(filepath.Join(installRoot, "etc", "init.d", "zapret")); err != nil {
		t.Fatalf("expected zapret service to be installed: %v", err)
	}

	serviceLog, err := os.ReadFile(serviceLogPath)
	if err != nil {
		t.Fatalf("read service log: %v", err)
	}
	for _, want := range []string{
		"opkg:update",
		"opkg:install:ca-bundle",
		"opkg:install:nftables",
		"opkg:install:kmod-nft-tproxy",
		"opkg:install:dnsmasq-full",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}
	if !strings.Contains(string(serviceLog), "opkg:install:zapret_") {
		t.Fatalf("expected service log to contain zapret ipk install, got %q", string(serviceLog))
	}
	for _, want := range []string{
		"zapret:stop",
		"zapret:disable",
	} {
		if !strings.Contains(string(serviceLog), want) {
			t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
		}
	}

	if !strings.Contains(stdout, "Bundled Zapret installed") {
		t.Fatalf("expected stdout to mention bundled zapret install, got %q", stdout)
	}
}

func TestInstallScriptBootstrapsBareRouterAcrossSupportedArchitectures(t *testing.T) {
	t.Parallel()

	systemUnzip, err := exec.LookPath("unzip")
	if err != nil {
		t.Skip("unzip is required on the test host")
	}

	version := "1.2.3"
	zapretVersion := "v72.20260113"
	scriptPath := renderInstallScript(t, version, supportedReleaseArches()...)

	for _, arch := range supportedReleaseArches() {
		arch := arch
		t.Run(arch, func(t *testing.T) {
			t.Parallel()

			downloadDir := t.TempDir()
			installRoot := t.TempDir()
			serviceLogPath := filepath.Join(t.TempDir(), "services.log")
			opkgStatePath := filepath.Join(t.TempDir(), "opkg-state.txt")

			writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_"+arch+".tar.gz"))
			writeXrayTarball(t, filepath.Join(downloadDir, "xray_"+version+"_"+arch+".tar.gz"))
			writeFile(t, filepath.Join(downloadDir, "zapret-api-latest.json"), fmt.Sprintf("{\"tag_name\":\"%s\"}\n", zapretVersion), 0o644)
			writeZapretZip(t, filepath.Join(downloadDir, "zapret_"+zapretVersion+"_"+arch+".zip"), zapretVersion, arch)
			writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "cron"))
			writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "rpcd"))
			writeServiceStub(t, filepath.Join(installRoot, "etc", "init.d", "uhttpd"))
			writeFile(t, opkgStatePath, "dnsmasq\n", 0o644)

			binDir := t.TempDir()
			writeInstallOpkgStub(t, filepath.Join(binDir, "opkg"), arch)
			writeInstallWgetStub(t, filepath.Join(binDir, "wget"))

			stdout, stderr, err := runInstallScriptWithEnv(
				t,
				scriptPath,
				installRoot,
				binDir,
				downloadDir,
				serviceLogPath,
				map[string]string{
					"ROUTEFLUX_TEST_BIN_DIR":      binDir,
					"ROUTEFLUX_TEST_INSTALL_ROOT": installRoot,
					"ROUTEFLUX_TEST_OPKG_STATE":   opkgStatePath,
					"ROUTEFLUX_TEST_SYSTEM_UNZIP": systemUnzip,
					"ROUTEFLUX_ZAPRET_VERSION":    "latest",
				},
				"--base-url", "https://example.test/releases/download/v1.2.3",
				"--install-root", installRoot,
			)
			if err != nil {
				t.Fatalf("run install script: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
			}

			for _, path := range []string{
				filepath.Join(installRoot, "usr", "bin", "routeflux"),
				filepath.Join(installRoot, "usr", "bin", "xray"),
				filepath.Join(installRoot, "etc", "init.d", "xray"),
				filepath.Join(installRoot, "etc", "init.d", "zapret"),
				filepath.Join(installRoot, "etc", "routeflux", "install-manifest.txt"),
			} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("expected %s to exist: %v", path, err)
				}
			}

			serviceLog, err := os.ReadFile(serviceLogPath)
			if err != nil {
				t.Fatalf("read service log: %v", err)
			}
			for _, want := range []string{
				"opkg:update",
				"opkg:install:ca-bundle",
				"opkg:install:nftables",
				"opkg:install:kmod-nft-tproxy",
				"opkg:install:dnsmasq-full",
				"opkg:remove:dnsmasq",
				"zapret:stop",
				"zapret:disable",
				"routeflux:enable",
				"routeflux:restart",
				"rpcd:reload",
				"uhttpd:reload",
			} {
				if !strings.Contains(string(serviceLog), want) {
					t.Fatalf("expected service log to contain %q, got %q", want, string(serviceLog))
				}
			}
			if !strings.Contains(string(serviceLog), "opkg:install:zapret_") {
				t.Fatalf("expected service log to contain zapret ipk install, got %q", string(serviceLog))
			}

			manifestData, err := os.ReadFile(filepath.Join(installRoot, "etc", "routeflux", "install-manifest.txt"))
			if err != nil {
				t.Fatalf("read install manifest: %v", err)
			}
			for _, want := range []string{
				"pkg=ca-bundle",
				"pkg=nftables",
				"pkg=kmod-nft-tproxy",
				"pkg=dnsmasq-full",
				"pkg=zapret",
				"restore=dnsmasq",
			} {
				if !strings.Contains(string(manifestData), want) {
					t.Fatalf("expected manifest to contain %q, got %q", want, string(manifestData))
				}
			}

			if !strings.Contains(stdout, "Bundled Xray installed") {
				t.Fatalf("expected stdout to mention bundled xray install, got %q", stdout)
			}
			if !strings.Contains(stdout, "Bundled Zapret installed") {
				t.Fatalf("expected stdout to mention bundled zapret install, got %q", stdout)
			}
			if !strings.Contains(stdout, arch) {
				t.Fatalf("expected stdout to mention resolved arch %q, got %q", arch, stdout)
			}
		})
	}
}

func renderInstallScript(t *testing.T, version string, arches ...string) string {
	t.Helper()

	root := repoRoot(t)
	outPath := filepath.Join(t.TempDir(), "install.sh")
	args := append([]string{filepath.Join(root, "scripts", "render-install.sh"), version, outPath}, arches...)

	cmd := exec.Command("sh", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render install script: %v\n%s", err, output)
	}

	return outPath
}

func runInstallScript(t *testing.T, scriptPath, installRoot, binDir, downloadDir, serviceLogPath string, extraArgs ...string) (string, string, error) {
	t.Helper()

	args := append([]string{
		scriptPath,
		"--base-url", "https://example.test/releases/download/v1.2.3",
		"--install-root", installRoot,
		"--without-deps",
		"--without-zapret",
	}, extraArgs...)
	return runInstallScriptWithEnv(t, scriptPath, installRoot, binDir, downloadDir, serviceLogPath, nil, args[1:]...)
}

func runInstallScriptWithEnv(
	t *testing.T,
	scriptPath, installRoot, binDir, downloadDir, serviceLogPath string,
	extraEnv map[string]string,
	extraArgs ...string,
) (string, string, error) {
	t.Helper()

	args := append([]string{scriptPath}, extraArgs...)
	cmd := exec.Command("sh", args...)
	env := append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ROUTEFLUX_TEST_DOWNLOAD_DIR="+downloadDir,
		"ROUTEFLUX_TEST_SERVICE_LOG="+serviceLogPath,
	)
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

func writeTestTarball(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create tarball dir: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	cronHelper, err := os.ReadFile(filepath.Join(repoRoot(t), "openwrt", "root", "usr", "libexec", "routeflux-cron"))
	if err != nil {
		t.Fatalf("read routeflux cron helper: %v", err)
	}

	addTarFile(t, tw, "./usr/bin/routeflux", 0o755, "#!/bin/sh\nprintf 'routeflux stub\\n'\n")
	addTarFile(t, tw, "./etc/init.d/routeflux", 0o755, "#!/bin/sh\nset -eu\nprintf '%s:%s\\n' \"$(basename \"$0\")\" \"${1:-}\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")
	addTarFile(t, tw, "./usr/libexec/routeflux-cron", 0o755, string(cronHelper))
	addTarFile(t, tw, "./www/luci-static/resources/view/routeflux/overview.js", 0o644, "'use strict';\n")
}

func writeXrayTarball(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create tarball dir: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	addTarFile(t, tw, "./usr/bin/xray", 0o755, "#!/bin/sh\nprintf 'xray stub\\n'\n")
	addTarFile(t, tw, "./etc/init.d/xray", 0o755, "#!/bin/sh\nexit 0\n")
}

func writeZapretZip(t *testing.T, path, version, arch string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create zapret zip dir: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zapret zip: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	name := fmt.Sprintf("zapret_%s_%s.ipk", strings.TrimPrefix(version, "v"), arch)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("fake zapret ipk")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
}

func addTarFile(t *testing.T, tw *tar.Writer, name string, mode int64, data string) {
	t.Helper()

	header := &tar.Header{
		Name: name,
		Mode: mode,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write tar header for %s: %v", name, err)
	}
	if _, err := tw.Write([]byte(data)); err != nil {
		t.Fatalf("write tar data for %s: %v", name, err)
	}
}

func writeServiceStub(t *testing.T, path string) {
	t.Helper()
	writeExecutable(t, path, "#!/bin/sh\nset -eu\nprintf '%s:%s\\n' \"$(basename \"$0\")\" \"${1:-}\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeInstallOpkgStub(t *testing.T, path, arch string) {
	t.Helper()

	script := `#!/bin/sh
set -eu
state="${ROUTEFLUX_TEST_OPKG_STATE:?}"
log="${ROUTEFLUX_TEST_SERVICE_LOG:?}"
bin_dir="${ROUTEFLUX_TEST_BIN_DIR:?}"
install_root="${ROUTEFLUX_TEST_INSTALL_ROOT:?}"
system_unzip="${ROUTEFLUX_TEST_SYSTEM_UNZIP:-}"
dnsmasq_conflict_state="${state}.dnsmasq-full-conflict"
touch "${state}"

has_pkg() {
	grep -Fxq "$1" "${state}" 2>/dev/null
}

add_pkg() {
	if has_pkg "$1"; then
		return 0
	fi
	printf '%s\n' "$1" >> "${state}"
}

remove_pkg() {
	tmp="${state}.tmp"
	grep -Fxv "$1" "${state}" > "${tmp}" 2>/dev/null || true
	mv "${tmp}" "${state}"
}

case "${1:-}" in
	print-architecture)
		printf 'arch all 1\narch noarch 1\narch ` + arch + ` 10\n'
		;;
	list-installed)
		pkg="${2:-}"
		if has_pkg "${pkg}"; then
			printf '%s - 1\n' "${pkg}"
		fi
		;;
	update)
		printf 'opkg:update\n' >> "${log}"
		;;
	install)
		shift
		for item in "$@"; do
			base="$(basename "${item}")"
			printf 'opkg:install:%s\n' "${base}" >> "${log}"
			case "${item}" in
				*.ipk)
					case "${base}" in
						zapret*.ipk)
							add_pkg zapret
							mkdir -p "${install_root}/etc/init.d"
							cat > "${install_root}/etc/init.d/zapret" <<'EOS'
#!/bin/sh
set -eu
printf '%s:%s\n' "$(basename "$0")" "${1:-}" >> "${ROUTEFLUX_TEST_SERVICE_LOG:?}"
EOS
							chmod 0755 "${install_root}/etc/init.d/zapret"
							;;
					esac
					;;
				*)
					if [ "${item}" = "dnsmasq-full" ] && has_pkg dnsmasq && [ ! -f "${dnsmasq_conflict_state}" ]; then
						: > "${dnsmasq_conflict_state}"
						exit 1
					fi
					add_pkg "${item}"
					if [ "${item}" = "unzip" ] && [ -n "${system_unzip}" ]; then
						printf '#!/bin/sh\nexec "%s" "$@"\n' "${system_unzip}" > "${bin_dir}/unzip"
						chmod 0755 "${bin_dir}/unzip"
					elif [ "${item}" = "curl" ]; then
						cat > "${bin_dir}/curl" <<'EOS'
#!/bin/sh
set -eu
download_dir="${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}"
out=""
stdout=1
url=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			stdout=0
			out="$2"
			shift 2
			;;
		-f|-s|-S|-L)
			shift
			;;
		*)
			url="$1"
			shift
			;;
	esac
done

case "${url}" in
	*api.github.com*/releases/latest)
		source="${download_dir}/zapret-api-latest.json"
		;;
	*)
		source="${download_dir}/${url##*/}"
		;;
esac

if [ "${stdout}" = "1" ]; then
	cat "${source}"
else
	mkdir -p "$(dirname "${out}")"
	cp "${source}" "${out}"
fi
EOS
						chmod 0755 "${bin_dir}/curl"
					elif [ "${item}" = "wget-ssl" ]; then
						cat > "${bin_dir}/wget" <<'EOS'
#!/bin/sh
set -eu
download_dir="${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}"
mode="file"
out=""
url=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		-qO-)
			mode="stdout"
			shift
			;;
		-O)
			mode="file"
			out="$2"
			shift 2
			;;
		*)
			url="$1"
			shift
			;;
	esac
done

case "${url}" in
	*api.github.com*/releases/latest)
		source="${download_dir}/zapret-api-latest.json"
		;;
	*)
		source="${download_dir}/${url##*/}"
		;;
esac

if [ "${mode}" = "stdout" ]; then
	cat "${source}"
else
	mkdir -p "$(dirname "${out}")"
	cp "${source}" "${out}"
fi
EOS
						chmod 0755 "${bin_dir}/wget"
					fi
					;;
			esac
		done
		;;
	remove)
		shift
		for item in "$@"; do
			printf 'opkg:remove:%s\n' "${item}" >> "${log}"
			remove_pkg "${item}"
		done
		;;
	*)
		exit 0
		;;
esac
`

	writeExecutable(t, path, script)
}

func writeInstallWgetStub(t *testing.T, path string) {
	t.Helper()

	writeExecutable(t, path, `#!/bin/sh
set -eu
download_dir="${ROUTEFLUX_TEST_DOWNLOAD_DIR:?}"
mode="file"
out=""
url=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		-qO-)
			mode="stdout"
			shift
			;;
		-O)
			mode="file"
			out="$2"
			shift 2
			;;
		*)
			url="$1"
			shift
			;;
	esac
done

case "${url}" in
	*api.github.com*/releases/latest)
		source="${download_dir}/zapret-api-latest.json"
		if [ "${mode}" = "stdout" ]; then
			cat "${source}"
		else
			mkdir -p "$(dirname "${out}")"
			cp "${source}" "${out}"
		fi
		;;
	*)
		source="${download_dir}/${url##*/}"
		if [ "${mode}" = "stdout" ]; then
			cat "${source}"
		else
			mkdir -p "$(dirname "${out}")"
			cp "${source}" "${out}"
		fi
		;;
esac
`)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("unexpected mode for %s: got %o want %o", path, got, want)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("ROUTEFLUX_TEST_SERVICE_LOG") != "" {
		fmt.Fprintln(os.Stderr, "ROUTEFLUX_TEST_SERVICE_LOG should not be set for test process")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

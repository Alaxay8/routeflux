package release_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptInstallsMatchedOpenWrtTarball(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, "mipsel_24kc", "x86_64")

	downloadDir := t.TempDir()
	installRoot := t.TempDir()
	serviceLogPath := filepath.Join(t.TempDir(), "services.log")

	writeTestTarball(t, filepath.Join(downloadDir, "routeflux_"+version+"_mipsel_24kc.tar.gz"))
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
}

func TestInstallScriptRejectsUnsupportedArchitecture(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, "mipsel_24kc", "x86_64")
	installRoot := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "opkg"), "#!/bin/sh\nset -eu\n[ \"${1:-}\" = \"print-architecture\" ] || exit 1\nprintf 'arch all 1\\narch noarch 1\\narch aarch64_cortex-a53 10\\n'\n")

	stdout, stderr, err := runInstallScript(t, scriptPath, installRoot, binDir, t.TempDir(), filepath.Join(t.TempDir(), "services.log"))
	if err == nil {
		t.Fatalf("expected install script to fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "unsupported architecture") {
		t.Fatalf("expected unsupported architecture error, got %q", combined)
	}
	if !strings.Contains(combined, "mipsel_24kc") || !strings.Contains(combined, "x86_64") {
		t.Fatalf("expected supported arch list in output, got %q", combined)
	}
}

func TestInstallScriptInstallsBundledXrayWhenMissing(t *testing.T) {
	t.Parallel()

	version := "1.2.3"
	scriptPath := renderInstallScript(t, version, "mipsel_24kc", "x86_64")
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
	scriptPath := renderInstallScript(t, version, "mipsel_24kc", "x86_64")

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
	scriptPath := renderInstallScript(t, version, "mipsel_24kc", "x86_64")

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

	args := append([]string{scriptPath, "--base-url", "https://example.test/releases/download/v1.2.3", "--install-root", installRoot}, extraArgs...)
	cmd := exec.Command("sh", args...)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ROUTEFLUX_TEST_DOWNLOAD_DIR="+downloadDir,
		"ROUTEFLUX_TEST_SERVICE_LOG="+serviceLogPath,
	)

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

	addTarFile(t, tw, "./usr/bin/routeflux", 0o755, "#!/bin/sh\nprintf 'routeflux stub\\n'\n")
	addTarFile(t, tw, "./etc/init.d/routeflux", 0o755, "#!/bin/sh\nset -eu\nprintf '%s:%s\\n' \"$(basename \"$0\")\" \"${1:-}\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")
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

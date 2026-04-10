package release_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestXrayUpdateHelperSkipsInstallWhenAlreadyLatest(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	helperPath := filepath.Join(repoRoot, "openwrt", "root", "usr", "libexec", "routeflux-xray-update")
	helperSource, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}

	workDir := t.TempDir()
	helperCopy := filepath.Join(workDir, "routeflux-xray-update")
	writeExecutable(t, helperCopy, string(helperSource))

	xrayStub := filepath.Join(workDir, "xray")
	writeExecutable(t, xrayStub, "#!/bin/sh\nset -eu\nprintf 'Xray 26.3.27 (Xray, Penetrates Everything.)\\n'\n")

	wgetStub := filepath.Join(workDir, "wget")
	writeExecutable(t, wgetStub, "#!/bin/sh\nset -eu\nif [ \"$1\" = \"-qO-\" ]; then\n\t[ \"$2\" = \"https://example.invalid/releases/latest\" ] || { echo \"unexpected url: $2\" >&2; exit 1; }\n\tprintf '{\"tag_name\":\"v26.3.27\"}\\n'\n\texit 0\nfi\necho \"unexpected download\" >&2\nexit 1\n")

	stdout, stderr, err := runXrayUpdateHelper(t, helperCopy, map[string]string{
		"ROUTEFLUX_XRAY_BINARY":       xrayStub,
		"ROUTEFLUX_XRAY_WGET":         wgetStub,
		"ROUTEFLUX_XRAY_RELEASES_API": "https://example.invalid/releases/latest",
	})
	if err != nil {
		t.Fatalf("run xray update helper: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "ROUTEFLUX_XRAY_UPDATE_STATUS=up-to-date") {
		t.Fatalf("expected up-to-date status, got stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Xray is already up to date (26.3.27).") {
		t.Fatalf("expected up-to-date message, got stdout:\n%s", stdout)
	}
}

func TestXrayUpdateHelperRejectsUnsupportedSoftFloatMips(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	helperPath := filepath.Join(repoRoot, "openwrt", "root", "usr", "libexec", "routeflux-xray-update")
	helperSource, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}

	workDir := t.TempDir()
	helperCopy := filepath.Join(workDir, "routeflux-xray-update")
	writeExecutable(t, helperCopy, string(helperSource))

	xrayStub := filepath.Join(workDir, "xray")
	writeExecutable(t, xrayStub, "#!/bin/sh\nset -eu\nprintf 'Xray 26.2.6 (Xray, Penetrates Everything.)\\n'\n")

	wgetStub := filepath.Join(workDir, "wget")
	writeExecutable(t, wgetStub, "#!/bin/sh\nset -eu\nif [ \"$1\" = \"-qO-\" ]; then\n\t[ \"$2\" = \"https://example.invalid/releases/latest\" ] || { echo \"unexpected url: $2\" >&2; exit 1; }\n\tprintf '{\"tag_name\":\"v26.3.27\"}\\n'\n\texit 0\nfi\necho \"unexpected download\" >&2\nexit 1\n")

	stdout, stderr, err := runXrayUpdateHelper(t, helperCopy, map[string]string{
		"ROUTEFLUX_XRAY_BINARY":        xrayStub,
		"ROUTEFLUX_XRAY_WGET":          wgetStub,
		"ROUTEFLUX_XRAY_RELEASES_API":  "https://example.invalid/releases/latest",
		"ROUTEFLUX_XRAY_ARCH_OVERRIDE": "mipsel_24kc",
	})
	if err == nil {
		t.Fatalf("expected helper to fail on unsupported mips, stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	output := stdout + stderr
	if !strings.Contains(output, "Official Xray releases do not publish a soft-float MIPS build.") {
		t.Fatalf("expected unsupported arch message, got:\n%s", output)
	}
}

func TestXrayUpdateHelperInstallsSupportedOfficialAsset(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	helperPath := filepath.Join(repoRoot, "openwrt", "root", "usr", "libexec", "routeflux-xray-update")
	helperSource, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}

	workDir := t.TempDir()
	helperCopy := filepath.Join(workDir, "routeflux-xray-update")
	writeExecutable(t, helperCopy, string(helperSource))

	xrayTarget := filepath.Join(workDir, "bin", "xray")
	writeExecutable(t, xrayTarget, "#!/bin/sh\nset -eu\nprintf 'Xray 26.2.6 (Xray, Penetrates Everything.)\\n'\n")

	serviceLog := filepath.Join(workDir, "service.log")
	serviceStub := filepath.Join(workDir, "xray-service")
	writeExecutable(t, serviceStub, "#!/bin/sh\nset -eu\nprintf '%s\\n' \"${1:-}\" >> \"${ROUTEFLUX_TEST_SERVICE_LOG:?}\"\n")

	wgetStub := filepath.Join(workDir, "wget")
	writeExecutable(t, wgetStub, "#!/bin/sh\nset -eu\nif [ \"$1\" = \"-qO-\" ]; then\n\t[ \"$2\" = \"https://example.invalid/releases/latest\" ] || { echo \"unexpected url: $2\" >&2; exit 1; }\n\tprintf '{\"tag_name\":\"v26.3.27\"}\\n'\n\texit 0\nfi\nout=\"$2\"\nurl=\"$3\"\n[ \"$url\" = \"https://example.invalid/download/v26.3.27/Xray-linux-64.zip\" ] || { echo \"unexpected asset url: $url\" >&2; exit 1; }\nprintf 'fake zip' > \"$out\"\n")

	unzipStub := filepath.Join(workDir, "unzip")
	writeExecutable(t, unzipStub, "#!/bin/sh\nset -eu\n[ \"$1\" = \"-p\" ] || { echo \"unexpected unzip arg: $1\" >&2; exit 1; }\ncat <<'EOS'\n#!/bin/sh\nset -eu\nprintf 'Xray 26.3.27 (Xray, Penetrates Everything.)\\n'\nEOS\n")

	stdout, stderr, err := runXrayUpdateHelper(t, helperCopy, map[string]string{
		"ROUTEFLUX_XRAY_BINARY":           xrayTarget,
		"ROUTEFLUX_XRAY_SERVICE":          serviceStub,
		"ROUTEFLUX_XRAY_WGET":             wgetStub,
		"ROUTEFLUX_XRAY_UNZIP":            unzipStub,
		"ROUTEFLUX_XRAY_RELEASES_API":     "https://example.invalid/releases/latest",
		"ROUTEFLUX_XRAY_RELEASE_BASE_URL": "https://example.invalid/download",
		"ROUTEFLUX_XRAY_ARCH_OVERRIDE":    "x86_64",
		"ROUTEFLUX_XRAY_WORKDIR":          filepath.Join(workDir, "tmp"),
		"ROUTEFLUX_TEST_SERVICE_LOG":      serviceLog,
	})
	if err != nil {
		t.Fatalf("run xray update helper: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "ROUTEFLUX_XRAY_UPDATE_STATUS=updated") {
		t.Fatalf("expected updated status, got stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Xray updated from 26.2.6 to 26.3.27.") {
		t.Fatalf("expected update message, got stdout:\n%s", stdout)
	}

	data, err := os.ReadFile(serviceLog)
	if err != nil {
		t.Fatalf("read service log: %v", err)
	}
	if !strings.Contains(string(data), "restart") {
		t.Fatalf("expected service restart, got %q", string(data))
	}

	output, err := exec.Command(xrayTarget, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run installed xray: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "26.3.27") {
		t.Fatalf("expected installed xray version, got %q", string(output))
	}
}

func runXrayUpdateHelper(t *testing.T, helperPath string, env map[string]string) (string, string, error) {
	t.Helper()

	cmd := exec.Command("sh", helperPath)
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	return string(output), "", err
}

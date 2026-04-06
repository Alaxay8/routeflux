package release_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfUpdateHelperSkipsInstallWhenAlreadyLatest(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	helperPath := filepath.Join(repoRoot, "openwrt", "root", "usr", "libexec", "routeflux-self-update")
	helperSource, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}

	workDir := t.TempDir()
	helperCopy := filepath.Join(workDir, "routeflux-self-update")
	writeExecutable(t, helperCopy, string(helperSource))

	statePath := filepath.Join(workDir, "routeflux-version-state")
	writeFile(t, statePath, "0.1.6 959bc2f\n", 0o644)

	routefluxStub := filepath.Join(workDir, "routeflux")
	writeExecutable(t, routefluxStub, "#!/bin/sh\nset -eu\nset -- $(cat \"${ROUTEFLUX_TEST_STATE:?}\")\nprintf '{\"version\":\"%s\",\"commit\":\"%s\",\"build_date\":\"2026-04-06T08:08:29Z\"}\\n' \"$1\" \"$2\"\n")

	wgetStub := filepath.Join(workDir, "wget")
	writeExecutable(t, wgetStub, "#!/bin/sh\nset -eu\nif [ \"$1\" = \"-qO-\" ]; then\n\turl=\"$2\"\n\tcase \"$url\" in\n\t\thttps://example.invalid/releases/latest)\n\t\t\tprintf '{\"tag_name\":\"v0.1.6\"}\\n'\n\t\t\t;;\n\t\thttps://example.invalid/git/ref/tags/v0.1.6)\n\t\t\tprintf '{\"object\":{\"sha\":\"959bc2f5bf3bcae91b704857c66bd21b21b97744\"}}\\n'\n\t\t\t;;\n\t\t*)\n\t\t\techo \"unexpected url: $url\" >&2\n\t\t\texit 1\n\t\t\t;;\n\tesac\n\texit 0\nfi\nout=\"$2\"\nurl=\"$3\"\necho \"unexpected install download: $url\" >&2\nexit 1\n")

	stdout, stderr, err := runSelfUpdateHelper(t, helperCopy, map[string]string{
		"ROUTEFLUX_SELF_UPDATE_TEST_MODE": "1",
		"ROUTEFLUX_BINARY":                routefluxStub,
		"ROUTEFLUX_WGET":                  wgetStub,
		"ROUTEFLUX_TEST_STATE":            statePath,
		"ROUTEFLUX_RELEASES_API":          "https://example.invalid/releases/latest",
		"ROUTEFLUX_TAG_REF_API_BASE":      "https://example.invalid/git/ref/tags",
		"ROUTEFLUX_RELEASE_INSTALL_URL":   "https://example.invalid/releases/latest/download/install.sh",
		"ROUTEFLUX_UPDATE_SCRIPT_PATH":    filepath.Join(workDir, "routeflux-install.sh"),
	})
	if err != nil {
		t.Fatalf("run self-update helper: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "ROUTEFLUX_SELF_UPDATE_STATUS=up-to-date") {
		t.Fatalf("expected up-to-date status, got stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "RouteFlux is already up to date (0.1.6, 959bc2f).") {
		t.Fatalf("expected up-to-date message, got stdout:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(workDir, "routeflux-install.sh")); !os.IsNotExist(err) {
		t.Fatalf("expected install script download to be skipped, stat err=%v", err)
	}
}

func TestSelfUpdateHelperInstallsWhenCommitDiffers(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	helperPath := filepath.Join(repoRoot, "openwrt", "root", "usr", "libexec", "routeflux-self-update")
	helperSource, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}

	workDir := t.TempDir()
	helperCopy := filepath.Join(workDir, "routeflux-self-update")
	writeExecutable(t, helperCopy, string(helperSource))

	statePath := filepath.Join(workDir, "routeflux-version-state")
	writeFile(t, statePath, "0.1.6 463158b\n", 0o644)
	installLog := filepath.Join(workDir, "install.log")

	routefluxStub := filepath.Join(workDir, "routeflux")
	writeExecutable(t, routefluxStub, "#!/bin/sh\nset -eu\nset -- $(cat \"${ROUTEFLUX_TEST_STATE:?}\")\nprintf '{\"version\":\"%s\",\"commit\":\"%s\",\"build_date\":\"2026-04-06T08:08:29Z\"}\\n' \"$1\" \"$2\"\n")

	wgetStub := filepath.Join(workDir, "wget")
	writeExecutable(t, wgetStub, "#!/bin/sh\nset -eu\nif [ \"$1\" = \"-qO-\" ]; then\n\turl=\"$2\"\n\tcase \"$url\" in\n\t\thttps://example.invalid/releases/latest)\n\t\t\tprintf '{\"tag_name\":\"v0.1.6\"}\\n'\n\t\t\t;;\n\t\thttps://example.invalid/git/ref/tags/v0.1.6)\n\t\t\tprintf '{\"object\":{\"sha\":\"959bc2f5bf3bcae91b704857c66bd21b21b97744\"}}\\n'\n\t\t\t;;\n\t\t*)\n\t\t\techo \"unexpected url: $url\" >&2\n\t\t\texit 1\n\t\t\t;;\n\tesac\n\texit 0\nfi\nout=\"$2\"\nurl=\"$3\"\n[ \"$url\" = \"https://example.invalid/releases/latest/download/install.sh\" ] || { echo \"unexpected install url: $url\" >&2; exit 1; }\ncat > \"$out\" <<'EOS'\n#!/bin/sh\nset -eu\nprintf 'installed\\n' >> \"${ROUTEFLUX_TEST_INSTALL_LOG:?}\"\nprintf '0.1.6 959bc2f\\n' > \"${ROUTEFLUX_TEST_STATE:?}\"\nEOS\nchmod 0755 \"$out\"\n")

	stdout, stderr, err := runSelfUpdateHelper(t, helperCopy, map[string]string{
		"ROUTEFLUX_SELF_UPDATE_TEST_MODE": "1",
		"ROUTEFLUX_BINARY":                routefluxStub,
		"ROUTEFLUX_WGET":                  wgetStub,
		"ROUTEFLUX_TEST_STATE":            statePath,
		"ROUTEFLUX_TEST_INSTALL_LOG":      installLog,
		"ROUTEFLUX_RELEASES_API":          "https://example.invalid/releases/latest",
		"ROUTEFLUX_TAG_REF_API_BASE":      "https://example.invalid/git/ref/tags",
		"ROUTEFLUX_RELEASE_INSTALL_URL":   "https://example.invalid/releases/latest/download/install.sh",
		"ROUTEFLUX_UPDATE_SCRIPT_PATH":    filepath.Join(workDir, "routeflux-install.sh"),
	})
	if err != nil {
		t.Fatalf("run self-update helper: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "ROUTEFLUX_SELF_UPDATE_STATUS=updated") {
		t.Fatalf("expected updated status, got stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "RouteFlux updated from 0.1.6 (463158b) to 0.1.6 (959bc2f).") {
		t.Fatalf("expected update message, got stdout:\n%s", stdout)
	}
	if data, err := os.ReadFile(installLog); err != nil {
		t.Fatalf("read install log: %v", err)
	} else if !strings.Contains(string(data), "installed") {
		t.Fatalf("expected install script to run, got %q", string(data))
	}
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if strings.TrimSpace(string(stateData)) != "0.1.6 959bc2f" {
		t.Fatalf("expected updated state, got %q", string(stateData))
	}
}

func runSelfUpdateHelper(t *testing.T, helperPath string, env map[string]string) (string, string, error) {
	t.Helper()

	cmd := exec.Command("sh", helperPath)
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	return string(output), "", err
}

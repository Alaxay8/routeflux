package openwrt_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	openWrtVersion             = "24.10.5"
	openWrtImageURL            = "https://downloads.openwrt.org/releases/24.10.5/targets/x86/64/openwrt-24.10.5-x86-64-generic-ext4-combined.img.gz"
	xrayVersion                = "v26.2.6"
	xrayLinuxAMD64URL          = "https://github.com/XTLS/Xray-core/releases/download/v26.2.6/Xray-linux-64.zip"
	integrationRawVLESSFixture = "vless://11111111-1111-1111-1111-111111111111@203.0.113.10:443?encryption=none&security=tls&sni=edge.example.com&type=ws&path=%2Fproxy&host=cdn.example.com#OpenWrt%20Integration"
	routefluxRemoteBinary      = "/usr/bin/routeflux"
	xrayRemoteBinary           = "/usr/bin/xray"
	xrayRemoteService          = "/etc/init.d/xray"
	routefluxRemoteService     = "/etc/init.d/routeflux"
	xrayRemoteConfigDir        = "/etc/xray"
	consoleLoginPrompt         = "login:"
	consoleRootPrompt          = "root@"
	openWrtBootTimeout         = 10 * time.Minute
	sshRetryDelay              = 2 * time.Second
	sshRetryAttempts           = 5
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestOpenWrtEndToEnd(t *testing.T) {
	if os.Getenv("ROUTEFLUX_RUN_OPENWRT_INTEGRATION") != "1" {
		t.Skip("set ROUTEFLUX_RUN_OPENWRT_INTEGRATION=1 to run OpenWrt/QEMU integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	harness, err := newOpenWRTHarness(t)
	if err != nil {
		t.Fatalf("create integration harness: %v", err)
	}
	defer harness.Close()

	if err := harness.Start(ctx); err != nil {
		t.Fatalf("start OpenWrt VM: %v", err)
	}
	if err := harness.InstallRouteFlux(ctx); err != nil {
		t.Fatalf("install routeflux: %v", err)
	}
	if err := harness.InstallXray(ctx); err != nil {
		t.Fatalf("install xray: %v", err)
	}

	subID, nodeID, err := harness.AddSubscription(ctx, integrationRawVLESSFixture)
	if err != nil {
		t.Fatalf("add subscription: %v", err)
	}
	if err := harness.Connect(ctx, subID, nodeID); err != nil {
		t.Fatalf("connect routeflux: %v", err)
	}
	if err := harness.AssertXrayRunning(ctx); err != nil {
		t.Fatalf("assert xray running after connect: %v", err)
	}
	if err := harness.EnableFirewallTargets(ctx, "1.1.1.1"); err != nil {
		t.Fatalf("enable firewall targets: %v", err)
	}
	if err := harness.AssertFirewallTableContains(ctx, "ip daddr @target_v4"); err != nil {
		t.Fatalf("assert firewall table: %v", err)
	}

	if err := harness.RebootAndWait(ctx); err != nil {
		t.Fatalf("reboot and wait: %v", err)
	}
	if err := harness.AssertRouteFluxRestore(ctx); err != nil {
		t.Fatalf("assert restore after reboot: %v", err)
	}

	if err := harness.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect routeflux: %v", err)
	}
	if err := harness.AssertFirewallTableRemoved(ctx); err != nil {
		t.Fatalf("assert firewall table removed: %v", err)
	}
}

type openWRTHarness struct {
	t             *testing.T
	repoRoot      string
	workDir       string
	cacheDir      string
	sshPort       int
	qemuImagePath string
	routefluxBin  string
	xrayBin       string
	sshKeyPath    string
	qemuCmd       *exec.Cmd
	console       *consoleLog
	consoleStdin  io.WriteCloser
}

func newOpenWRTHarness(t *testing.T) (*openWRTHarness, error) {
	t.Helper()

	repoRoot, err := repoRoot()
	if err != nil {
		return nil, err
	}

	workDir := t.TempDir()
	cacheDir := filepath.Join(repoRoot, ".cache", "openwrt-integration")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	sshPort, err := freeTCPPort()
	if err != nil {
		return nil, err
	}

	routefluxBin, err := buildRouteFluxLinuxAMD64(t, repoRoot, workDir)
	if err != nil {
		return nil, err
	}

	xrayBin, err := ensureXrayLinuxAMD64(cacheDir)
	if err != nil {
		return nil, err
	}

	qemuImagePath, err := ensureOpenWrtImage(cacheDir, workDir)
	if err != nil {
		return nil, err
	}

	sshKeyPath := filepath.Join(workDir, "integration-key")
	if err := generateSSHKeyPair(sshKeyPath); err != nil {
		return nil, err
	}

	return &openWRTHarness{
		t:             t,
		repoRoot:      repoRoot,
		workDir:       workDir,
		cacheDir:      cacheDir,
		sshPort:       sshPort,
		qemuImagePath: qemuImagePath,
		routefluxBin:  routefluxBin,
		xrayBin:       xrayBin,
		sshKeyPath:    sshKeyPath,
	}, nil
}

func (h *openWRTHarness) Start(ctx context.Context) error {
	qemuPath, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return fmt.Errorf("find qemu-system-x86_64: %w", err)
	}

	stdoutReader, stdoutWriter := io.Pipe()
	cmd := exec.CommandContext(ctx, qemuPath,
		"-accel", "tcg",
		"-m", "512",
		"-display", "none",
		"-monitor", "none",
		"-serial", "stdio",
		"-drive", fmt.Sprintf("file=%s,format=raw", h.qemuImagePath),
		"-nic", fmt.Sprintf("user,model=e1000,hostfwd=tcp::%d-:22", h.sshPort),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open qemu stdin: %w", err)
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stdoutWriter

	h.console = newConsoleLog(stdoutReader)
	h.consoleStdin = stdin
	h.qemuCmd = cmd

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	bootCtx, cancel := context.WithTimeout(ctx, openWrtBootTimeout)
	defer cancel()

	if err := h.console.WaitForAny(bootCtx, 0, consoleLoginPrompt, consoleRootPrompt, "Please press Enter to activate this console."); err != nil {
		return fmt.Errorf("wait for OpenWrt console activation: %w", err)
	}
	bootOutput := h.console.SliceFrom(0)
	if strings.Contains(bootOutput, "Please press Enter to activate this console.") {
		if _, err := io.WriteString(h.consoleStdin, "\n"); err != nil {
			return fmt.Errorf("activate OpenWrt console: %w", err)
		}
		if err := h.console.WaitForAny(bootCtx, 0, consoleLoginPrompt, consoleRootPrompt); err != nil {
			return fmt.Errorf("wait for OpenWrt login state: %w", err)
		}
	}

	if !strings.Contains(h.console.SliceFrom(0), consoleRootPrompt) {
		loginStart := h.console.Len()
		if _, err := io.WriteString(h.consoleStdin, "root\n"); err != nil {
			return fmt.Errorf("log into OpenWrt console: %w", err)
		}
		if err := h.console.WaitFor(bootCtx, loginStart, consoleRootPrompt); err != nil {
			return fmt.Errorf("wait for OpenWrt shell prompt: %w", err)
		}
	}

	if err := h.ConsoleCommand(bootCtx, "/etc/init.d/firewall stop"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "mkdir -p /etc/dropbear"); err != nil {
		return err
	}
	publicKey, err := os.ReadFile(h.sshKeyPath + ".pub")
	if err != nil {
		return fmt.Errorf("read ssh public key: %w", err)
	}
	if err := h.ConsoleCommand(bootCtx, "printf '%s\n' "+shellQuote(strings.TrimSpace(string(publicKey)))+" > /etc/dropbear/authorized_keys"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "/etc/init.d/dropbear restart"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci set network.lan.proto='dhcp'"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci -q delete network.lan.ipaddr"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci -q delete network.lan.netmask"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci -q delete network.lan.gateway"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci -q delete network.lan.dns"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "uci commit network"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "service network restart"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "sleep 5"); err != nil {
		return err
	}
	if err := h.ConsoleCommand(bootCtx, "/etc/init.d/dropbear restart"); err != nil {
		return err
	}

	if err := h.waitForSSH(bootCtx); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) InstallRouteFlux(ctx context.Context) error {
	if err := h.sshCommand(ctx, "mkdir -p /usr/bin /etc/routeflux"); err != nil {
		return err
	}
	if err := h.scpFile(ctx, h.routefluxBin, routefluxRemoteBinary); err != nil {
		return err
	}
	if err := h.scpFile(ctx, filepath.Join(h.repoRoot, "openwrt", "root", "etc", "init.d", "routeflux"), routefluxRemoteService); err != nil {
		return err
	}
	if err := h.sshCommand(ctx, "chmod 0755 "+routefluxRemoteBinary+" "+routefluxRemoteService); err != nil {
		return err
	}
	if err := h.sshCommand(ctx, routefluxRemoteService+" enable"); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) InstallXray(ctx context.Context) error {
	if err := h.sshCommand(ctx, "mkdir -p "+xrayRemoteConfigDir+" /var/log"); err != nil {
		return err
	}
	if err := h.scpFile(ctx, h.xrayBin, xrayRemoteBinary); err != nil {
		return err
	}
	if err := h.scpFile(ctx, filepath.Join(h.repoRoot, "openwrt", "root", "etc", "init.d", "xray"), xrayRemoteService); err != nil {
		return err
	}
	if err := h.sshCommand(ctx, "chmod 0755 "+xrayRemoteBinary+" "+xrayRemoteService); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) AddSubscription(ctx context.Context, raw string) (string, string, error) {
	output, err := h.sshOutput(ctx, routefluxRemoteBinary+" --json add --raw "+shellQuote(raw))
	if err != nil {
		return "", "", err
	}

	var response struct {
		ID    string `json:"id"`
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", "", fmt.Errorf("decode routeflux add response: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if response.ID == "" || len(response.Nodes) == 0 || response.Nodes[0].ID == "" {
		return "", "", fmt.Errorf("unexpected routeflux add response: %s", strings.TrimSpace(string(output)))
	}

	return response.ID, response.Nodes[0].ID, nil
}

func (h *openWRTHarness) Connect(ctx context.Context, subscriptionID, nodeID string) error {
	if err := h.sshCommand(ctx, fmt.Sprintf("%s connect --subscription %s --node %s", routefluxRemoteBinary, shellQuote(subscriptionID), shellQuote(nodeID))); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) EnableFirewallTargets(ctx context.Context, target string) error {
	return h.sshCommand(ctx, fmt.Sprintf("%s firewall set targets %s", routefluxRemoteBinary, shellQuote(target)))
}

func (h *openWRTHarness) Disconnect(ctx context.Context) error {
	if err := h.sshCommand(ctx, routefluxRemoteBinary+" disconnect"); err != nil {
		return err
	}
	if err := h.sshCommand(ctx, routefluxRemoteBinary+" firewall disable"); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) AssertXrayRunning(ctx context.Context) error {
	output, err := h.sshOutput(ctx, xrayRemoteService+" status")
	if err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(string(output)), "running") {
		return fmt.Errorf("unexpected xray status: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *openWRTHarness) AssertFirewallTableContains(ctx context.Context, needle string) error {
	output, err := h.sshOutput(ctx, "nft list table inet routeflux")
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), needle) {
		return fmt.Errorf("firewall table missing %q\n%s", needle, strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *openWRTHarness) AssertFirewallTableRemoved(ctx context.Context) error {
	output, err := h.sshOutput(ctx, "nft list table inet routeflux")
	if err == nil {
		return fmt.Errorf("expected routeflux nft table to be removed, got:\n%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *openWRTHarness) RebootAndWait(ctx context.Context) error {
	_ = h.sshCommand(ctx, routefluxRemoteService+" start")
	_ = h.sshCommand(ctx, "sync")
	_ = h.sshCommand(ctx, "reboot")

	downCtx, cancelDown := context.WithTimeout(ctx, 90*time.Second)
	defer cancelDown()
	for {
		if downCtx.Err() != nil {
			return fmt.Errorf("wait for OpenWrt reboot shutdown: %w", downCtx.Err())
		}
		if err := h.sshCommand(downCtx, "true"); err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	upCtx, cancelUp := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelUp()
	if err := h.waitForSSH(upCtx); err != nil {
		return err
	}
	return nil
}

func (h *openWRTHarness) AssertRouteFluxRestore(ctx context.Context) error {
	pollCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	for {
		if pollCtx.Err() != nil {
			return fmt.Errorf("wait for routeflux restore: %w", pollCtx.Err())
		}

		diagnostics, err := h.sshOutput(pollCtx, routefluxRemoteBinary+" --json diagnostics")
		if err == nil {
			var snapshot struct {
				Status struct {
					State struct {
						Connected         bool   `json:"connected"`
						LastFailureReason string `json:"last_failure_reason"`
					} `json:"state"`
				} `json:"status"`
				Runtime struct {
					Running      bool   `json:"running"`
					ServiceState string `json:"service_state"`
				} `json:"runtime"`
			}
			if json.Unmarshal(diagnostics, &snapshot) == nil &&
				snapshot.Status.State.Connected &&
				snapshot.Runtime.Running &&
				snapshot.Status.State.LastFailureReason == "" {
				if err := h.AssertFirewallTableContains(pollCtx, "ip daddr @target_v4"); err == nil {
					return nil
				}
			}
		}

		time.Sleep(3 * time.Second)
	}
}

func (h *openWRTHarness) ConsoleCommand(ctx context.Context, command string) error {
	marker := "__ROUTEFLUX_DONE__"
	start := h.console.Len()
	if _, err := io.WriteString(h.consoleStdin, command+"; printf '"+marker+"\\n'\n"); err != nil {
		return fmt.Errorf("write console command %q: %w", command, err)
	}
	if err := h.console.WaitFor(ctx, start, marker); err != nil {
		return fmt.Errorf("run console command %q: %w", command, err)
	}
	return nil
}

func (h *openWRTHarness) waitForSSH(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("wait for ssh: %w", ctx.Err())
		}
		if err := h.sshCommand(ctx, "true"); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}

func (h *openWRTHarness) sshCommand(ctx context.Context, remoteCommand string) error {
	var lastErr error
	for attempt := 1; attempt <= sshRetryAttempts; attempt++ {
		cmd := exec.CommandContext(ctx, "ssh",
			"-i", h.sshKeyPath,
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=5",
			"-o", "LogLevel=ERROR",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", fmt.Sprintf("%d", h.sshPort),
			"root@127.0.0.1",
			remoteCommand,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("ssh %q: %w: %s", remoteCommand, err, strings.TrimSpace(string(output)))
			if !isRetryableSSHError(lastErr) || attempt == sshRetryAttempts || ctx.Err() != nil {
				return lastErr
			}
			time.Sleep(sshRetryDelay)
			continue
		}
		return nil
	}
	return lastErr
}

func (h *openWRTHarness) sshOutput(ctx context.Context, remoteCommand string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= sshRetryAttempts; attempt++ {
		cmd := exec.CommandContext(ctx, "ssh",
			"-i", h.sshKeyPath,
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=5",
			"-o", "LogLevel=ERROR",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", fmt.Sprintf("%d", h.sshPort),
			"root@127.0.0.1",
			remoteCommand,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("ssh %q: %w: %s", remoteCommand, err, strings.TrimSpace(string(output)))
			if !isRetryableSSHError(lastErr) || attempt == sshRetryAttempts || ctx.Err() != nil {
				return nil, lastErr
			}
			time.Sleep(sshRetryDelay)
			continue
		}
		return output, nil
	}
	return nil, lastErr
}

func (h *openWRTHarness) scpFile(ctx context.Context, localPath, remotePath string) error {
	var lastErr error
	for attempt := 1; attempt <= sshRetryAttempts; attempt++ {
		cmd := exec.CommandContext(ctx, "scp",
			"-O",
			"-i", h.sshKeyPath,
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=5",
			"-o", "LogLevel=ERROR",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-P", fmt.Sprintf("%d", h.sshPort),
			localPath,
			"root@127.0.0.1:"+remotePath,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("scp %s -> %s: %w: %s", localPath, remotePath, err, strings.TrimSpace(string(output)))
			if !isRetryableSSHError(lastErr) || attempt == sshRetryAttempts || ctx.Err() != nil {
				return lastErr
			}
			time.Sleep(sshRetryDelay)
			continue
		}
		return nil
	}
	return lastErr
}

func (h *openWRTHarness) Close() {
	if h.consoleStdin != nil {
		_ = h.consoleStdin.Close()
	}
	if h.qemuCmd != nil && h.qemuCmd.Process != nil {
		_ = h.qemuCmd.Process.Kill()
		_, _ = h.qemuCmd.Process.Wait()
	}
}

type consoleLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newConsoleLog(reader io.Reader) *consoleLog {
	log := &consoleLog{}
	go func() {
		_, _ = io.Copy(log, reader)
	}()
	return log
}

func (c *consoleLog) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *consoleLog) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Len()
}

func (c *consoleLog) SliceFrom(offset int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if offset < 0 {
		offset = 0
	}
	data := c.buf.Bytes()
	if offset >= len(data) {
		return ""
	}
	return string(data[offset:])
}

func (c *consoleLog) WaitFor(ctx context.Context, offset int, needle string) error {
	for {
		if strings.Contains(c.SliceFrom(offset), needle) {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("wait for %q: %w\nconsole tail:\n%s", needle, ctx.Err(), tail(c.SliceFrom(0), 4000))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *consoleLog) WaitForAny(ctx context.Context, offset int, needles ...string) error {
	for {
		chunk := c.SliceFrom(offset)
		for _, needle := range needles {
			if strings.Contains(chunk, needle) {
				return nil
			}
		}
		if ctx.Err() != nil {
			return fmt.Errorf("wait for any of %q: %w\nconsole tail:\n%s", strings.Join(needles, ", "), ctx.Err(), tail(c.SliceFrom(0), 4000))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func buildRouteFluxLinuxAMD64(t *testing.T, repoRoot, workDir string) (string, error) {
	t.Helper()

	if path := os.Getenv("ROUTEFLUX_OPENWRT_ROUTEFLUX_BIN"); path != "" {
		return path, nil
	}

	outputPath := filepath.Join(workDir, "routeflux-linux-amd64")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", outputPath, "./cmd/routeflux")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build routeflux linux/amd64 binary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return outputPath, nil
}

func ensureOpenWrtImage(cacheDir, workDir string) (string, error) {
	gzipPath := filepath.Join(cacheDir, filepath.Base(openWrtImageURL))
	rawCachePath := strings.TrimSuffix(gzipPath, ".gz")
	if err := downloadFile(openWrtImageURL, gzipPath); err != nil {
		return "", err
	}
	if err := gunzipFile(gzipPath, rawCachePath); err != nil {
		_ = os.Remove(gzipPath)
		_ = os.Remove(rawCachePath)
		if retryErr := downloadFile(openWrtImageURL, gzipPath); retryErr != nil {
			return "", retryErr
		}
		if retryErr := gunzipFile(gzipPath, rawCachePath); retryErr != nil {
			return "", retryErr
		}
	}

	workingPath := filepath.Join(workDir, filepath.Base(rawCachePath))
	if err := copyFile(rawCachePath, workingPath); err != nil {
		return "", err
	}
	return workingPath, nil
}

func ensureXrayLinuxAMD64(cacheDir string) (string, error) {
	zipPath := filepath.Join(cacheDir, filepath.Base(xrayLinuxAMD64URL))
	binaryPath := filepath.Join(cacheDir, "xray-linux-64", "xray")

	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	if err := downloadFile(xrayLinuxAMD64URL, zipPath); err != nil {
		return "", err
	}
	if err := unzipSingleBinary(zipPath, "xray", binaryPath); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func generateSSHKeyPair(path string) error {
	cmd := exec.Command("ssh-keygen", "-q", "-N", "", "-f", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate ssh key pair: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func downloadFile(url, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create download temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	curlCmd := exec.CommandContext(ctx, "curl", "-fsSL", url)
	curlCmd.Stdout = tmp
	curlCmd.Stderr = os.Stderr
	if err := curlCmd.Run(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("download %s: %w", url, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close download temp file: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("rename download temp file: %w", err)
	}
	return nil
}

func gunzipFile(src, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create gunzip dir: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open gzip file %s: %w", src, err)
	}
	defer in.Close()

	reader, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("open gzip reader %s: %w", src, err)
	}
	defer reader.Close()
	reader.Multistream(false)

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create decompressed file %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("decompress %s: %w", src, err)
	}
	return nil
}

func isRetryableSSHError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection reset by peer",
		"connection refused",
		"connection closed by remote host",
		"kex_exchange_identification",
		"operation timed out",
		"connection timed out",
		"broken pipe",
	} {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func unzipSingleBinary(zipPath, binaryName, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create unzip dir: %w", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if filepath.Base(file.Name) != binaryName {
			continue
		}

		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open %s from %s: %w", binaryName, zipPath, err)
		}
		defer src.Close()

		out, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("create %s: %w", dest, err)
		}
		if _, err := io.Copy(out, src); err != nil {
			_ = out.Close()
			return fmt.Errorf("extract %s: %w", binaryName, err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dest, err)
		}
		if err := os.Chmod(dest, 0o755); err != nil {
			return fmt.Errorf("chmod %s: %w", dest, err)
		}
		return nil
	}

	return fmt.Errorf("binary %s not found in %s", binaryName, zipPath)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dest, err)
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..")), nil
}

func freeTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve tcp port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func tail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

package speedtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultBinaryPath       = "/usr/bin/xray"
	defaultWarmupDuration   = 750 * time.Millisecond
	defaultDownloadDuration = 4 * time.Second
	defaultUploadDuration   = 3 * time.Second
	defaultWarmupStreams    = 2
	defaultMeasureStreams   = 4
	downloadChunkBytes      = 1 << 20
	uploadChunkBytes        = 512 << 10
	proxyReadyTimeout       = 20 * time.Second
)

var ErrBusy = errors.New("speed test already running")

type Request struct {
	SubscriptionID string
	NodeID         string
	NodeName       string
	Config         []byte
	HTTPProxyPort  int
}

type Result struct {
	SubscriptionID string    `json:"subscription_id"`
	NodeID         string    `json:"node_id"`
	NodeName       string    `json:"node_name"`
	LatencyMS      float64   `json:"latency_ms"`
	DownloadMbps   float64   `json:"download_mbps"`
	UploadMbps     float64   `json:"upload_mbps"`
	DownloadBytes  int64     `json:"download_bytes"`
	UploadBytes    int64     `json:"upload_bytes"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
}

type Metrics struct {
	Latency          time.Duration
	DownloadBytes    int64
	DownloadDuration time.Duration
	UploadBytes      int64
	UploadDuration   time.Duration
}

type Tester interface {
	Test(context.Context, Request) (Result, error)
}

type Provider struct {
	Name              string
	ProbeURL          string
	DownloadURLFormat string
	UploadURL         string
}

type Process interface {
	Stop() error
	Output() string
}

type StartFunc func(context.Context, string, string) (Process, error)
type MeasureFunc func(context.Context, string, Provider) (Metrics, error)

type Runner struct {
	LockPath   string
	BinaryPath string
	TempRoot   string
	Provider   Provider
	Start      StartFunc
	Measure    MeasureFunc
	Now        func() time.Time
}

func (r Runner) Test(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.SubscriptionID) == "" {
		return Result{}, fmt.Errorf("speed test subscription ID is required")
	}
	if strings.TrimSpace(req.NodeID) == "" {
		return Result{}, fmt.Errorf("speed test node ID is required")
	}
	if len(req.Config) == 0 {
		return Result{}, fmt.Errorf("speed test config is required")
	}
	if req.HTTPProxyPort <= 0 {
		return Result{}, fmt.Errorf("speed test HTTP proxy port must be positive")
	}

	lock, err := acquireLock(r.LockPath)
	if err != nil {
		return Result{}, err
	}
	defer lock.Release()

	if err := cleanupStaleTempDirs(r.tempRoot()); err != nil {
		return Result{}, err
	}

	tempDir, err := os.MkdirTemp(r.tempRoot(), "routeflux-speedtest-")
	if err != nil {
		return Result{}, fmt.Errorf("create speed test temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, req.Config, 0o600); err != nil {
		return Result{}, fmt.Errorf("write speed test config: %w", err)
	}

	start := r.startFunc()
	process, err := start(ctx, r.binaryPath(), configPath)
	if err != nil {
		return Result{}, err
	}
	defer process.Stop()

	now := r.now()
	startedAt := now()
	metrics, err := r.measureFunc()(ctx, fmt.Sprintf("http://127.0.0.1:%d", req.HTTPProxyPort), r.provider())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("speed test timed out")
		}
		output := strings.TrimSpace(process.Output())
		if output != "" {
			return Result{}, fmt.Errorf("measure speed test: %w\n%s", err, output)
		}
		return Result{}, fmt.Errorf("measure speed test: %w", err)
	}

	finishedAt := now()
	if !finishedAt.After(startedAt) {
		finishedAt = startedAt.Add(time.Millisecond)
	}

	return Result{
		SubscriptionID: req.SubscriptionID,
		NodeID:         req.NodeID,
		NodeName:       req.NodeName,
		LatencyMS:      roundFloat(metrics.Latency.Seconds()*1000, 2),
		DownloadMbps:   roundFloat(bitsPerSecond(metrics.DownloadBytes, metrics.DownloadDuration)/1_000_000, 2),
		UploadMbps:     roundFloat(bitsPerSecond(metrics.UploadBytes, metrics.UploadDuration)/1_000_000, 2),
		DownloadBytes:  metrics.DownloadBytes,
		UploadBytes:    metrics.UploadBytes,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}, nil
}

func (r Runner) binaryPath() string {
	if strings.TrimSpace(r.BinaryPath) != "" {
		return strings.TrimSpace(r.BinaryPath)
	}
	return defaultBinaryPath
}

func (r Runner) tempRoot() string {
	if strings.TrimSpace(r.TempRoot) != "" {
		return strings.TrimSpace(r.TempRoot)
	}
	return os.TempDir()
}

func (r Runner) provider() Provider {
	if strings.TrimSpace(r.Provider.Name) != "" {
		return r.Provider
	}
	return defaultProvider
}

func (r Runner) startFunc() StartFunc {
	if r.Start != nil {
		return r.Start
	}
	return startProcess
}

func (r Runner) measureFunc() MeasureFunc {
	if r.Measure != nil {
		return r.Measure
	}
	return measureViaHTTPProxy
}

func (r Runner) now() func() time.Time {
	if r.Now != nil {
		return r.Now
	}
	return time.Now
}

var defaultProvider = Provider{
	Name:              "cloudflare",
	ProbeURL:          "https://speed.cloudflare.com/__down?bytes=1",
	DownloadURLFormat: "https://speed.cloudflare.com/__down?bytes=%d",
	UploadURL:         "https://speed.cloudflare.com/__up",
}

func measureViaHTTPProxy(ctx context.Context, proxyURL string, provider Provider) (Metrics, error) {
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return Metrics{}, fmt.Errorf("parse proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy:              http.ProxyURL(proxy),
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	if err := waitForProxy(ctx, client, provider.ProbeURL); err != nil {
		return Metrics{}, err
	}

	latency, err := measureLatency(ctx, client, provider.ProbeURL)
	if err != nil {
		return Metrics{}, err
	}

	if _, _, err := transferDownloadWindow(
		ctx,
		client,
		provider.DownloadURLFormat,
		defaultWarmupDuration,
		downloadChunkBytes,
		defaultWarmupStreams,
	); err != nil {
		return Metrics{}, err
	}

	downloadBytes, downloadDuration, err := transferDownloadWindow(
		ctx,
		client,
		provider.DownloadURLFormat,
		defaultDownloadDuration,
		downloadChunkBytes,
		defaultMeasureStreams,
	)
	if err != nil {
		return Metrics{}, err
	}

	if _, _, err := transferUploadWindow(
		ctx,
		client,
		provider.UploadURL,
		defaultWarmupDuration,
		uploadChunkBytes,
		defaultWarmupStreams,
	); err != nil {
		return Metrics{}, err
	}

	uploadBytes, uploadDuration, err := transferUploadWindow(
		ctx,
		client,
		provider.UploadURL,
		defaultUploadDuration,
		uploadChunkBytes,
		defaultMeasureStreams,
	)
	if err != nil {
		return Metrics{}, err
	}

	return Metrics{
		Latency:          latency,
		DownloadBytes:    downloadBytes,
		DownloadDuration: downloadDuration,
		UploadBytes:      uploadBytes,
		UploadDuration:   uploadDuration,
	}, nil
}

func waitForProxy(ctx context.Context, client *http.Client, probeURL string) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, proxyReadyTimeout)
	defer cancel()

	var lastErr error
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, probeURL, nil)
		if err != nil {
			return fmt.Errorf("build proxy readiness request: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("unexpected status %s", resp.Status)
		} else {
			lastErr = err
		}

		select {
		case <-deadlineCtx.Done():
			if lastErr != nil {
				return fmt.Errorf("wait for proxy ready: %w", lastErr)
			}
			return fmt.Errorf("wait for proxy ready: %w", deadlineCtx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func measureLatency(ctx context.Context, client *http.Client, probeURL string) (time.Duration, error) {
	samples := make([]time.Duration, 0, 3)
	for range 3 {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			return 0, fmt.Errorf("build latency request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("measure latency: %w", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return 0, fmt.Errorf("measure latency: unexpected status %s", resp.Status)
		}
		samples = append(samples, time.Since(start))
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)/2], nil
}

func transferDownloadWindow(ctx context.Context, client *http.Client, rawURLFormat string, duration time.Duration, chunkBytes int64, streams int) (int64, time.Duration, error) {
	if duration <= 0 {
		return 0, 0, nil
	}
	if chunkBytes <= 0 {
		chunkBytes = downloadChunkBytes
	}
	if streams <= 0 {
		streams = 1
	}

	return runParallelTransferWindow(ctx, duration, streams, func(phaseCtx context.Context) (int64, error) {
		return transferDownloadChunk(phaseCtx, client, fmt.Sprintf(rawURLFormat, chunkBytes))
	})
}

func transferDownloadChunk(ctx context.Context, client *http.Client, rawURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download speed test: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("download speed test: unexpected status %s", resp.Status)
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return n, fmt.Errorf("download speed test body: %w", err)
	}

	return n, nil
}

func transferUploadWindow(ctx context.Context, client *http.Client, rawURL string, duration time.Duration, chunkBytes int64, streams int) (int64, time.Duration, error) {
	if duration <= 0 {
		return 0, 0, nil
	}
	if chunkBytes <= 0 {
		chunkBytes = uploadChunkBytes
	}
	if streams <= 0 {
		streams = 1
	}

	return runParallelTransferWindow(ctx, duration, streams, func(phaseCtx context.Context) (int64, error) {
		return transferUploadChunk(phaseCtx, client, rawURL, chunkBytes)
	})
}

func runParallelTransferWindow(
	ctx context.Context,
	duration time.Duration,
	streams int,
	transferChunk func(context.Context) (int64, error),
) (int64, time.Duration, error) {
	if duration <= 0 {
		return 0, 0, nil
	}
	if streams <= 0 {
		streams = 1
	}

	phaseCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	start := time.Now()
	var transferred atomic.Int64
	errCh := make(chan error, 1)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
		cancel()
	}

	var wg sync.WaitGroup
	for range streams {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				if phaseCtx.Err() != nil {
					return
				}

				n, err := transferChunk(phaseCtx)
				if n > 0 {
					transferred.Add(n)
				}
				if err != nil {
					if phaseCtx.Err() != nil {
						return
					}
					reportErr(err)
					return
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	select {
	case err := <-errCh:
		return 0, 0, err
	default:
	}

	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	return transferred.Load(), elapsed, nil
}

func transferUploadChunk(ctx context.Context, client *http.Client, rawURL string, size int64) (int64, error) {
	body := &countingReader{reader: io.LimitReader(zeroReader{}, size)}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return 0, fmt.Errorf("build upload request: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return body.Count(), fmt.Errorf("upload speed test: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body.Count(), fmt.Errorf("upload speed test: unexpected status %s", resp.Status)
	}
	_, _ = io.Copy(io.Discard, resp.Body)

	return body.Count(), nil
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count += int64(n)
	return n, err
}

func (r *countingReader) Count() int64 {
	if r == nil {
		return 0
	}
	return r.count
}

type execProcess struct {
	cmd    *exec.Cmd
	output bytes.Buffer
	done   chan struct{}
}

func startProcess(ctx context.Context, binaryPath, configPath string) (Process, error) {
	cmd := exec.CommandContext(ctx, binaryPath, "run", "-config", configPath)
	configureProcess(cmd)
	process := &execProcess{
		cmd:  cmd,
		done: make(chan struct{}),
	}
	cmd.Stdout = &process.output
	cmd.Stderr = &process.output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start temporary xray: %w", err)
	}

	go func() {
		_ = cmd.Wait()
		close(process.done)
	}()

	return process, nil
}

func (p *execProcess) Stop() error {
	if p == nil || p.cmd == nil {
		return nil
	}

	if p.cmd.Process != nil && (p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()) {
		_ = terminateProcess(p.cmd)
	}

	if p.done != nil {
		<-p.done
	}
	return nil
}

func (p *execProcess) Output() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.output.String())
}

type fileLock struct {
	file *os.File
}

func acquireLock(path string) (*fileLock, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("speed test lock path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create speed test lock dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open speed test lock file: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock speed test file: %w", err)
	}

	return &fileLock{file: file}, nil
}

func (l *fileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	defer func() { l.file = nil }()
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return err
	}
	return l.file.Close()
}

func bitsPerSecond(bytes int64, duration time.Duration) float64 {
	if bytes <= 0 || duration <= 0 {
		return 0
	}
	return float64(bytes*8) / duration.Seconds()
}

func cleanupStaleTempDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read speed test temp root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "routeflux-speedtest-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return fmt.Errorf("remove stale speed test temp dir %q: %w", entry.Name(), err)
		}
	}

	return nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func roundFloat(value float64, scale int) float64 {
	if scale < 0 {
		return value
	}
	factor := math.Pow(10, float64(scale))
	return math.Round(value*factor) / factor
}

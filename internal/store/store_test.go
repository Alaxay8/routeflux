package store_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/Alaxay8/routeflux/internal/store"
)

func TestAtomicWriteJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := filepath.Join(dir, "routeflux")
	path := filepath.Join(root, "settings.json")

	settings := domain.DefaultSettings()
	settings.LogLevel = "debug"

	if err := store.AtomicWriteJSON(path, settings); err != nil {
		t.Fatalf("atomic write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected file content")
	}

	matches, err := filepath.Glob(filepath.Join(root, "*.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("unexpected temp files: %v", matches)
	}

	assertPerm(t, path, store.SecretFilePerm)
	assertPerm(t, root, store.PrivateDirPerm)
}

func TestFileStoreRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs := store.NewFileStore(root)

	sub := domain.Subscription{
		ID:              "sub-1",
		ProviderName:    "Example",
		SourceType:      domain.SourceTypeRaw,
		Source:          "vless://...",
		LastUpdatedAt:   time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC),
		RefreshInterval: domain.NewDuration(time.Hour),
		Nodes: []domain.Node{
			{ID: "node-1", Name: "Node 1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
		},
	}

	state := domain.RuntimeState{
		SchemaVersion:        1,
		Mode:                 domain.SelectionModeManual,
		Connected:            true,
		ActiveSubscriptionID: sub.ID,
		ActiveNodeID:         "node-1",
	}

	settings := domain.DefaultSettings()

	if err := fs.SaveSubscriptions([]domain.Subscription{sub}); err != nil {
		t.Fatalf("save subscriptions: %v", err)
	}

	if err := fs.SaveState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := fs.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	subs, err := fs.LoadSubscriptions()
	if err != nil {
		t.Fatalf("load subscriptions: %v", err)
	}

	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("unexpected subscriptions: %+v", subs)
	}

	gotState, err := fs.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if gotState.ActiveNodeID != "node-1" {
		t.Fatalf("unexpected state: %+v", gotState)
	}

	gotSettings, err := fs.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if gotSettings.LogLevel != settings.LogLevel {
		t.Fatalf("unexpected settings: %+v", gotSettings)
	}

	assertPerm(t, filepath.Join(root, "subscriptions.json"), store.SecretFilePerm)
	assertPerm(t, filepath.Join(root, "settings.json"), store.SecretFilePerm)
	assertPerm(t, filepath.Join(root, "state.json"), store.SecretFilePerm)
}

func TestFileStoreWithWriteLockSerializesAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	releasePath := filepath.Join(dir, "release")

	cmd := exec.Command(os.Args[0], "-test.run=^TestFileStoreWithWriteLockHelperProcess$")
	cmd.Env = append(os.Environ(),
		"ROUTEFLUX_STORE_LOCK_HELPER=1",
		"ROUTEFLUX_STORE_LOCK_DIR="+dir,
		"ROUTEFLUX_STORE_LOCK_RELEASE="+releasePath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read helper ready line: %v", err)
	}
	if strings.TrimSpace(line) != "locked" {
		t.Fatalf("unexpected helper ready line: %q", line)
	}

	fs := store.NewFileStore(dir)
	acquired := make(chan error, 1)
	go func() {
		acquired <- fs.WithWriteLock(func() error { return nil })
	}()

	select {
	case err := <-acquired:
		t.Fatalf("lock acquired before helper released it: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	if err := os.WriteFile(releasePath, []byte("release\n"), 0o644); err != nil {
		t.Fatalf("write release marker: %v", err)
	}

	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("acquire lock after release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for lock after helper release")
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait helper: %v", err)
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("unexpected mode for %s: got %o want %o", path, got, want)
	}
}

func TestFileStoreWithWriteLockHelperProcess(t *testing.T) {
	if os.Getenv("ROUTEFLUX_STORE_LOCK_HELPER") != "1" {
		return
	}

	dir := os.Getenv("ROUTEFLUX_STORE_LOCK_DIR")
	releasePath := os.Getenv("ROUTEFLUX_STORE_LOCK_RELEASE")

	fs := store.NewFileStore(dir)
	if err := fs.WithWriteLock(func() error {
		fmt.Fprintln(os.Stdout, "locked")
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(releasePath); err == nil {
				return nil
			}
		}
		return fmt.Errorf("timed out waiting for release marker")
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}
